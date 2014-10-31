/*
   Copyright 2014 CoreOS, Inc.

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

package etcdhttp

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	raftPrefix        = "/raft"
	peerMembersPrefix = "/members"
)

// NewPeerHandler generates an http.Handler to handle etcd peer (raft) requests.
func NewPeerHandler(server *etcdserver.EtcdServer) http.Handler {
	rh := &raftHandler{
		stats:       server,
		server:      server,
		clusterInfo: server.Cluster,
	}

	mh := &peerMembersHandler{
		clusterInfo: server.Cluster,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", http.NotFound)
	mux.Handle(raftPrefix, rh)
	mux.Handle(peerMembersPrefix, mh)
	return mux
}

type raftHandler struct {
	stats       etcdserver.Stats
	server      etcdserver.Server
	clusterInfo etcdserver.ClusterInfo
}

func (h *raftHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "POST") {
		return
	}

	wcid := h.clusterInfo.ID().String()
	w.Header().Set("X-Etcd-Cluster-ID", wcid)

	gcid := r.Header.Get("X-Etcd-Cluster-ID")
	if gcid != wcid {
		log.Printf("etcdhttp: request ignored due to cluster ID mismatch got %s want %s", gcid, wcid)
		http.Error(w, "clusterID mismatch", http.StatusPreconditionFailed)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("etcdhttp: error reading raft message:", err)
		http.Error(w, "error reading raft message", http.StatusBadRequest)
		return
	}
	var m raftpb.Message
	if err := m.Unmarshal(b); err != nil {
		log.Println("etcdhttp: error unmarshaling raft message:", err)
		http.Error(w, "error unmarshaling raft message", http.StatusBadRequest)
		return
	}
	if err := h.server.Process(context.TODO(), m); err != nil {
		switch err {
		case etcdserver.ErrRemoved:
			log.Printf("etcdhttp: reject message from removed member %s", types.ID(m.From).String())
			http.Error(w, "cannot process message from removed member", http.StatusForbidden)
		default:
			writeError(w, err)
		}
		return
	}
	if m.Type == raftpb.MsgApp {
		h.stats.UpdateRecvApp(types.ID(m.From), r.ContentLength)
	}
	w.WriteHeader(http.StatusNoContent)
}

type peerMembersHandler struct {
	clusterInfo etcdserver.ClusterInfo
}

func (h *peerMembersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET") {
		return
	}
	w.Header().Set("X-Etcd-Cluster-ID", h.clusterInfo.ID().String())

	if r.URL.Path != peerMembersPrefix {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	ms := h.clusterInfo.Members()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ms); err != nil {
		log.Printf("etcdhttp: %v", err)
	}
}
