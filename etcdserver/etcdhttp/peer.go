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
	"log"
	"net/http"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/rafthttp"
)

const (
	peerMembersPrefix = "/members"
)

// NewPeerHandler generates an http.Handler to handle etcd peer (raft) requests.
func NewPeerHandler(server *etcdserver.EtcdServer) http.Handler {
	rh := rafthttp.NewHandler(server, server.Cluster.ID())
	rsh := rafthttp.NewStreamHandler(server.SenderFinder(), server.ID(), server.Cluster.ID())
	mh := &peerMembersHandler{
		clusterInfo: server.Cluster,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", http.NotFound)
	mux.Handle(rafthttp.RaftPrefix, rh)
	mux.Handle(rafthttp.RaftStreamPrefix+"/", rsh)
	mux.Handle(peerMembersPrefix, mh)
	return mux
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
