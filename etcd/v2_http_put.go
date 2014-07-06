package etcd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
)

func (s *Server) PutHandler(w http.ResponseWriter, req *http.Request) error {
	if !s.node.IsLeader() {
		return s.redirect(w, req, s.node.Leader())
	}

	key := req.URL.Path[len("/v2/keys"):]

	req.ParseForm()

	value := req.Form.Get("value")
	dir := (req.FormValue("dir") == "true")

	expireTime, err := store.TTL(req.Form.Get("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Update", s.Store.Index())
	}

	prevValue, valueOk := firstValue(req.Form, "prevValue")
	prevIndexStr, indexOk := firstValue(req.Form, "prevIndex")
	prevExist, existOk := firstValue(req.Form, "prevExist")

	// Set handler: create a new node or replace the old one.
	if !valueOk && !indexOk && !existOk {
		return s.serveSet(w, req, key, dir, value, expireTime)
	}

	// update with test
	if existOk {
		if prevExist == "false" {
			// Create command: create a new node. Fail, if a node already exists
			// Ignore prevIndex and prevValue
			return s.serveCreate(w, req, key, dir, value, expireTime)
		}

		if prevExist == "true" && !indexOk && !valueOk {
			return s.serveUpdate(w, req, key, value, expireTime)
		}
	}

	var prevIndex uint64

	if indexOk {
		prevIndex, err = strconv.ParseUint(prevIndexStr, 10, 64)

		// bad previous index
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "CompareAndSwap", s.Store.Index())
		}
	} else {
		prevIndex = 0
	}

	if valueOk {
		if prevValue == "" {
			return etcdErr.NewError(etcdErr.EcodePrevValueRequired, "CompareAndSwap", s.Store.Index())
		}
	}

	return s.serveCAS(w, req, key, value, prevValue, prevIndex, expireTime)
}

func (s *Server) handleRet(w http.ResponseWriter, ret *store.Event) {
	b, _ := json.Marshal(ret)

	w.Header().Set("Content-Type", "application/json")
	// etcd index should be the same as the event index
	// which is also the last modified index of the node
	w.Header().Add("X-Etcd-Index", fmt.Sprint(ret.Index()))
	// w.Header().Add("X-Raft-Index", fmt.Sprint(s.CommitIndex()))
	// w.Header().Add("X-Raft-Term", fmt.Sprint(s.Term()))

	if ret.IsCreated() {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Write(b)
}

func (s *Server) serveSet(w http.ResponseWriter, req *http.Request, key string, dir bool, value string, expireTime time.Time) error {
	ret, err := s.Set(key, dir, value, expireTime)
	if err == nil {
		s.handleRet(w, ret)
		return nil
	}
	log.Println("set:", err)
	return err
}

func (s *Server) serveCreate(w http.ResponseWriter, req *http.Request, key string, dir bool, value string, expireTime time.Time) error {
	ret, err := s.Create(key, dir, value, expireTime, false)
	if err == nil {
		s.handleRet(w, ret)
		return nil
	}
	log.Println("create:", err)
	return err
}

func (s *Server) serveUpdate(w http.ResponseWriter, req *http.Request, key, value string, expireTime time.Time) error {
	// Update should give at least one option
	if value == "" && expireTime.Sub(store.Permanent) == 0 {
		return etcdErr.NewError(etcdErr.EcodeValueOrTTLRequired, "Update", s.Store.Index())
	}
	ret, err := s.Update(key, value, expireTime)
	if err == nil {
		s.handleRet(w, ret)
		return nil
	}
	log.Println("update:", err)
	return err
}

func (s *Server) serveCAS(w http.ResponseWriter, req *http.Request, key, value, prevValue string, prevIndex uint64, expireTime time.Time) error {
	ret, err := s.CAS(key, value, prevValue, prevIndex, expireTime)
	if err == nil {
		s.handleRet(w, ret)
		return nil
	}
	log.Println("update:", err)
	return err
}

func firstValue(f url.Values, key string) (string, bool) {
	l, ok := f[key]
	if !ok {
		return "", false
	}
	return l[0], true
}
