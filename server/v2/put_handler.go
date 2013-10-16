package v2

import (
	"net/http"
	"strconv"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"github.com/gorilla/mux"
)

func PutHandler(w http.ResponseWriter, req *http.Request, s Server) error {
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
	prevExist, existOk := req.Form["prevExist"]

	var c raft.Command

	// Set handler: create a new node or replace the old one.
	if !valueOk && !indexOk && !existOk {
		return SetHandler(w, req, s, key, value, expireTime)
	}

	// update with test
	if existOk {
		if prevExist[0] == "false" {
			// Create command: create a new node. Fail, if a node already exists
			// Ignore prevIndex and prevValue
			return CreateHandler(w, req, s, key, value, expireTime)
		}

		if prevExist[0] == "true" && !indexOk && !valueOk {
			return UpdateHandler(w, req, s, key, value, expireTime)
		}
	}

	var prevIndex uint64

	if indexOk {
		prevIndex, err = strconv.ParseUint(prevIndexStr[0], 10, 64)

		// bad previous index
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "CompareAndSwap", store.UndefIndex, store.UndefTerm)
		}
	} else {
		prevIndex = 0
	}

	if valueOk {
		if prevValue[0] == "" {
			return etcdErr.NewError(etcdErr.EcodePrevValueRequired, "CompareAndSwap", store.UndefIndex, store.UndefTerm)
		}
	}

	c = &store.CompareAndSwapCommand{
		Key:       key,
		Value:     value,
		PrevValue: prevValue[0],
		PrevIndex: prevIndex,
	}

	return s.Dispatch(c, w, req)
}

func SetHandler(w http.ResponseWriter, req *http.Request, s Server, key, value string, expireTime time.Time) error {
	c := &store.SetCommand{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
	}
	return s.Dispatch(c, w, req)
}

func CreateHandler(w http.ResponseWriter, req *http.Request, s Server, key, value string, expireTime time.Time) error {
	c := &store.CreateCommand{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
	}
	return s.Dispatch(c, w, req)
}

func UpdateHandler(w http.ResponseWriter, req *http.Request, s Server, key, value string, expireTime time.Time) error {
	c := &store.UpdateCommand{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
	}
	return s.Dispatch(c, w, req)
}
