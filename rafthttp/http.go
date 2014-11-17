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

package rafthttp

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

type Processor interface {
	Process(ctx context.Context, m raftpb.Message) error
}

type Stats interface {
	UpdateRecvApp(from types.ID, length int64)
}

func NewHandler(p Processor, cid types.ID, ss Stats) http.Handler {
	return &handler{
		p:   p,
		cid: cid,
		ss:  ss,
	}
}

type handler struct {
	p   Processor
	cid types.ID
	ss  Stats
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	wcid := h.cid.String()
	w.Header().Set("X-Etcd-Cluster-ID", wcid)

	gcid := r.Header.Get("X-Etcd-Cluster-ID")
	if gcid != wcid {
		log.Printf("rafthttp: request ignored due to cluster ID mismatch got %s want %s", gcid, wcid)
		http.Error(w, "clusterID mismatch", http.StatusPreconditionFailed)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("rafthttp: error reading raft message:", err)
		http.Error(w, "error reading raft message", http.StatusBadRequest)
		return
	}
	var m raftpb.Message
	if err := m.Unmarshal(b); err != nil {
		log.Println("rafthttp: error unmarshaling raft message:", err)
		http.Error(w, "error unmarshaling raft message", http.StatusBadRequest)
		return
	}
	if err := h.p.Process(context.TODO(), m); err != nil {
		switch v := err.(type) {
		case writerToResponse:
			v.WriteTo(w)
		default:
			log.Printf("rafthttp: error processing raft message: %v", err)
			http.Error(w, "error processing raft message", http.StatusInternalServerError)
		}
		return
	}
	if m.Type == raftpb.MsgApp {
		h.ss.UpdateRecvApp(types.ID(m.From), r.ContentLength)
	}
	w.WriteHeader(http.StatusNoContent)
}

type writerToResponse interface {
	WriteTo(w http.ResponseWriter)
}
