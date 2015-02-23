// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rafthttp

import (
	"io/ioutil"
	"log"
	"net/http"
	"path"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pioutil "github.com/coreos/etcd/pkg/ioutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	ConnReadLimitByte = 64 * 1024
)

var (
	RaftPrefix       = "/raft"
	RaftStreamPrefix = path.Join(RaftPrefix, "stream")
)

func NewHandler(r Raft, cid types.ID) http.Handler {
	return &handler{
		r:   r,
		cid: cid,
	}
}

func newStreamHandler(tr *transport, id, cid types.ID) http.Handler {
	return &streamHandler{
		tr:  tr,
		id:  id,
		cid: cid,
	}
}

type writerToResponse interface {
	WriteTo(w http.ResponseWriter)
}

type handler struct {
	r   Raft
	cid types.ID
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

	// Limit the data size that could be read from the request body, which ensures that read from
	// connection will not time out accidentally due to possible block in underlying implementation.
	limitedr := pioutil.NewLimitedBufferReader(r.Body, ConnReadLimitByte)
	b, err := ioutil.ReadAll(limitedr)
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
	if err := h.r.Process(context.TODO(), m); err != nil {
		switch v := err.(type) {
		case writerToResponse:
			v.WriteTo(w)
		default:
			log.Printf("rafthttp: error processing raft message: %v", err)
			http.Error(w, "error processing raft message", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type streamHandler struct {
	tr  *transport
	id  types.ID
	cid types.ID
}

func (h *streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var t streamType
	switch path.Dir(r.URL.Path) {
	case path.Join(RaftStreamPrefix, string(streamTypeMsgApp)):
		t = streamTypeMsgApp
	case path.Join(RaftStreamPrefix, string(streamTypeMessage)):
		t = streamTypeMessage
	// backward compatibility
	case RaftStreamPrefix:
		t = streamTypeMsgApp
	default:
		log.Printf("rafthttp: ignored unexpected streaming request path %s", r.URL.Path)
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}

	fromStr := path.Base(r.URL.Path)
	from, err := types.IDFromString(fromStr)
	if err != nil {
		log.Printf("rafthttp: failed to parse from %s into ID", fromStr)
		http.Error(w, "invalid from", http.StatusNotFound)
		return
	}
	p := h.tr.Peer(from)
	if p == nil {
		log.Printf("rafthttp: fail to find sender %s", from)
		http.Error(w, "error sender not found", http.StatusNotFound)
		return
	}

	wcid := h.cid.String()
	if gcid := r.Header.Get("X-Etcd-Cluster-ID"); gcid != wcid {
		log.Printf("rafthttp: streaming request ignored due to cluster ID mismatch got %s want %s", gcid, wcid)
		http.Error(w, "clusterID mismatch", http.StatusPreconditionFailed)
		return
	}

	wto := h.id.String()
	if gto := r.Header.Get("X-Raft-To"); gto != wto {
		log.Printf("rafthttp: streaming request ignored due to ID mismatch got %s want %s", gto, wto)
		http.Error(w, "to field mismatch", http.StatusPreconditionFailed)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()

	c := newCloseNotifier()
	conn := &outgoingConn{
		t:       t,
		termStr: r.Header.Get("X-Raft-Term"),
		Writer:  w,
		Flusher: w.(http.Flusher),
		Closer:  c,
	}
	p.attachOutgoingConn(conn)
	<-c.closeNotify()
}

type closeNotifier struct {
	done chan struct{}
}

func newCloseNotifier() *closeNotifier {
	return &closeNotifier{
		done: make(chan struct{}),
	}
}

func (n *closeNotifier) Close() error {
	close(n.done)
	return nil
}

func (n *closeNotifier) closeNotify() <-chan struct{} { return n.done }
