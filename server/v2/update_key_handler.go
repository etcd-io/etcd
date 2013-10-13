package v2

import (
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"github.com/gorilla/mux"
)

func UpdateKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	req.ParseForm()

	value := req.Form.Get("value")
	expireTime, err := store.TTL(req.Form.Get("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Update", store.UndefIndex, store.UndefTerm)
	}

	// Update should give at least one option
	if value == "" && expireTime.Sub(store.Permanent) == 0 {
		return etcdErr.NewError(etcdErr.EcodeValueOrTTLRequired, "Update", store.UndefIndex, store.UndefTerm)
	}

	prevValue, valueOk := req.Form["prevValue"]
	prevIndexStr, indexOk := req.Form["prevIndex"]

	var c raft.Command
	if !valueOk && !indexOk { // update without test
		c = &store.UpdateCommand{
			Key:        key,
			Value:      value,
			ExpireTime: expireTime,
		}

	} else { // update with test
		var prevIndex uint64

		if indexOk {
			prevIndex, err = strconv.ParseUint(prevIndexStr[0], 10, 64)

			// bad previous index
			if err != nil {
				return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Update", store.UndefIndex, store.UndefTerm)
			}
		} else {
			prevIndex = 0
		}

		c = &store.TestAndSetCommand{
			Key:       key,
			Value:     value,
			PrevValue: prevValue[0],
			PrevIndex: prevIndex,
		}
	}

	return s.Dispatch(c, w, req)
}
