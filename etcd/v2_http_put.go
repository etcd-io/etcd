/*
Copyright 2014 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
)

func (p *participant) PutHandler(w http.ResponseWriter, req *http.Request) error {
	if !p.node.IsLeader() {
		return p.redirect(w, req, p.node.Leader())
	}

	key := req.URL.Path[len("/v2/keys"):]

	req.ParseForm()

	value := req.Form.Get("value")
	dir := (req.FormValue("dir") == "true")

	expireTime, err := store.TTL(req.Form.Get("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Update", p.Store.Index())
	}

	prevValue, valueOk := firstValue(req.Form, "prevValue")
	prevIndexStr, indexOk := firstValue(req.Form, "prevIndex")
	prevExist, existOk := firstValue(req.Form, "prevExist")

	// Set handler: create a new node or replace the old one.
	if !valueOk && !indexOk && !existOk {
		return p.serveSet(w, req, key, dir, value, expireTime)
	}

	// update with test
	if existOk {
		if prevExist == "false" {
			// Create command: create a new node. Fail, if a node already exists
			// Ignore prevIndex and prevValue
			return p.serveCreate(w, req, key, dir, value, expireTime)
		}

		if prevExist == "true" && !indexOk && !valueOk {
			return p.serveUpdate(w, req, key, value, expireTime)
		}
	}

	var prevIndex uint64

	if indexOk {
		prevIndex, err = strconv.ParseUint(prevIndexStr, 10, 64)

		// bad previous index
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "CompareAndSwap", p.Store.Index())
		}
	} else {
		prevIndex = 0
	}

	if valueOk {
		if prevValue == "" {
			return etcdErr.NewError(etcdErr.EcodePrevValueRequired, "CompareAndSwap", p.Store.Index())
		}
	}

	return p.serveCAS(w, req, key, value, prevValue, prevIndex, expireTime)
}

func (p *participant) handleRet(w http.ResponseWriter, ret *store.Event) {
	b, _ := json.Marshal(ret)

	w.Header().Set("Content-Type", "application/json")
	// etcd index should be the same as the event index
	// which is also the last modified index of the node
	w.Header().Add("X-Etcd-Index", fmt.Sprint(ret.Index()))
	// w.Header().Add("X-Raft-Index", fmt.Sprint(p.CommitIndex()))
	// w.Header().Add("X-Raft-Term", fmt.Sprint(p.Term()))

	if ret.IsCreated() {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Write(b)
}

func (p *participant) serveSet(w http.ResponseWriter, req *http.Request, key string, dir bool, value string, expireTime time.Time) error {
	ret, err := p.Set(key, dir, value, expireTime)
	if err == nil {
		p.handleRet(w, ret)
		return nil
	}
	return err
}

func (p *participant) serveCreate(w http.ResponseWriter, req *http.Request, key string, dir bool, value string, expireTime time.Time) error {
	ret, err := p.Create(key, dir, value, expireTime, false)
	if err == nil {
		p.handleRet(w, ret)
		return nil
	}
	return err
}

func (p *participant) serveUpdate(w http.ResponseWriter, req *http.Request, key, value string, expireTime time.Time) error {
	// Update should give at least one option
	if value == "" && expireTime.Sub(store.Permanent) == 0 {
		return etcdErr.NewError(etcdErr.EcodeValueOrTTLRequired, "Update", p.Store.Index())
	}
	ret, err := p.Update(key, value, expireTime)
	if err == nil {
		p.handleRet(w, ret)
		return nil
	}
	return err
}

func (p *participant) serveCAS(w http.ResponseWriter, req *http.Request, key, value, prevValue string, prevIndex uint64, expireTime time.Time) error {
	ret, err := p.CAS(key, value, prevValue, prevIndex, expireTime)
	if err == nil {
		p.handleRet(w, ret)
		return nil
	}
	return err
}

func firstValue(f url.Values, key string) (string, bool) {
	l, ok := f[key]
	if !ok {
		return "", false
	}
	return l[0], true
}
