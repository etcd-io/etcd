package v1

import (
	"net/http"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"github.com/gorilla/mux"
)

// Sets the value for a given key.
func SetKeyHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	req.ParseForm()

	// Parse non-blank value.
	value := req.Form.Get("value")
	if len(value) == 0 {
		return etcdErr.NewError(200, "Set", store.UndefIndex, store.UndefTerm)
	}

	// Convert time-to-live to an expiration time.
	expireTime, err := store.TTL(req.Form.Get("ttl"))
	if err != nil {
		return etcdErr.NewError(202, "Set", store.UndefIndex, store.UndefTerm)
	}

	// If the "prevValue" is specified then test-and-set. Otherwise create a new key.
	var c raft.Command
	if prevValueArr, ok := req.Form["prevValue"]; ok && len(prevValueArr) > 0 {
		c = &store.CompareAndSwapCommand{
			Key:        key,
			Value:      value,
			PrevValue:  prevValueArr[0],
			ExpireTime: expireTime,
		}

	} else {
		c = &store.SetCommand{
			Key:        key,
			Value:      value,
			ExpireTime: expireTime,
		}
	}

	return s.Dispatch(c, w, req)
}
