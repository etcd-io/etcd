package v2

import (
	"net/http"
	"strconv"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/raft"
	"github.com/gorilla/mux"
)

func PutHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	var c raft.Command

	vars := mux.Vars(req)
	key := "/" + vars["key"]

	req.ParseForm()

	value := req.Form.Get("value")
	expireTime, err := store.TTL(req.Form.Get("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Update", s.Store().Index())
	}

	_, valueOk := req.Form["prevValue"]
	prevValue := req.Form.Get("prevValue")

	_, indexOk := req.Form["prevIndex"]
	prevIndexStr := req.Form.Get("prevIndex")

	_, existOk := req.Form["prevExist"]
	prevExist := req.Form.Get("prevExist")

	// Set handler: create a new node or replace the old one.
	if !valueOk && !indexOk && !existOk {
		return SetHandler(w, req, s, key, value, expireTime)
	}

	// update with test
	if existOk {
		if prevExist == "false" {
			// Create command: create a new node. Fail, if a node already exists
			// Ignore prevIndex and prevValue
			return CreateHandler(w, req, s, key, value, expireTime)
		}

		if prevExist == "true" && !indexOk && !valueOk {
			return UpdateHandler(w, req, s, key, value, expireTime)
		}
	}

	var prevIndex uint64

	if indexOk {
		prevIndex, err = strconv.ParseUint(prevIndexStr, 10, 64)

		// bad previous index
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "CompareAndSwap", s.Store().Index())
		}
	} else {
		prevIndex = 0
	}

	if valueOk {
		if prevValue == "" {
			return etcdErr.NewError(etcdErr.EcodePrevValueRequired, "CompareAndSwap", s.Store().Index())
		}
	}

	c = s.Store().CommandFactory().CreateCompareAndSwapCommand(key, value, prevValue, prevIndex, expireTime)
	return s.Dispatch(c, w, req)
}

func SetHandler(w http.ResponseWriter, req *http.Request, s Server, key, value string, expireTime time.Time) error {
	c := s.Store().CommandFactory().CreateSetCommand(key, value, expireTime)
	return s.Dispatch(c, w, req)
}

func CreateHandler(w http.ResponseWriter, req *http.Request, s Server, key, value string, expireTime time.Time) error {
	c := s.Store().CommandFactory().CreateCreateCommand(key, value, expireTime, false)
	return s.Dispatch(c, w, req)
}

func UpdateHandler(w http.ResponseWriter, req *http.Request, s Server, key, value string, expireTime time.Time) error {
	// Update should give at least one option
	if value == "" && expireTime.Sub(store.Permanent) == 0 {
		return etcdErr.NewError(etcdErr.EcodeValueOrTTLRequired, "Update", s.Store().Index())
	}

	c := s.Store().CommandFactory().CreateUpdateCommand(key, value, expireTime)
	return s.Dispatch(c, w, req)
}
