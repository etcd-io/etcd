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
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/coreos/etcd/raft"
)

const (
	raftPrefix = "/raft"
)

type raftHandler struct {
	mu      sync.RWMutex
	serving bool

	peerGetter peerGetter

	recv chan *raft.Message
	*http.ServeMux
}

func newRaftHandler(p peerGetter) *raftHandler {
	h := &raftHandler{
		recv:       make(chan *raft.Message, 512),
		peerGetter: p,
	}
	h.ServeMux = http.NewServeMux()
	h.ServeMux.HandleFunc(raftPrefix+"/cfg/", h.serveCfg)
	h.ServeMux.HandleFunc(raftPrefix, h.serveRaft)
	return h
}

func (h *raftHandler) start() {
	h.mu.Lock()
	h.serving = true
	h.mu.Unlock()
}

func (h *raftHandler) stop() {
	h.mu.Lock()
	h.serving = false
	h.mu.Unlock()
}

func (h *raftHandler) serveRaft(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	serving := h.serving
	h.mu.RUnlock()
	if !serving {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	msg := new(raft.Message)
	if err := json.NewDecoder(r.Body).Decode(msg); err != nil {
		log.Printf("raftHandler.serve decodeErr=\"%v\"\n", err)
		return
	}

	select {
	case h.recv <- msg:
	default:
		log.Printf("raftHandler.serve pushErr=\"recv channel is full\"\n")
		// drop the incoming package at network layer if the upper layer
		// cannot consume them in time.
		// TODO(xiangli): not return 200.
	}
	return
}

func (h *raftHandler) serveCfg(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	serving := h.serving
	h.mu.RUnlock()
	if !serving {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(r.URL.Path[len("/raft/cfg/"):], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := h.peerGetter.peer(id)
	if err == nil {
		w.Write([]byte(p.url))
		return
	}
	http.Error(w, err.Error(), http.StatusNotFound)
}
