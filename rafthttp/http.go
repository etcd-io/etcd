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
	"path"
	"strconv"
	"strings"

	"github.com/coreos/etcd/pkg/ioutils"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

const (
	ConnReadLimitByte = 64 * 1024
)

var (
	RaftPrefix       = "/raft"
	RaftStreamPrefix = path.Join(RaftPrefix, "stream")
)

type Processor interface {
	Process(ctx context.Context, m raftpb.Message) error
}

type SenderFinder interface {
	// Sender returns the sender of the given id.
	Sender(id types.ID) Sender
}

func NewHandler(p Processor, cid types.ID) http.Handler {
	return &handler{
		p:   p,
		cid: cid,
	}
}

// NewStreamHandler returns a handler which initiates streamer when receiving
// stream request from follower.
func NewStreamHandler(finder SenderFinder, id, cid types.ID) http.Handler {
	return &streamHandler{
		finder: finder,
		id:     id,
		cid:    cid,
	}
}

type handler struct {
	p   Processor
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
	limitedr := ioutils.NewLimitedBufferReader(r.Body, ConnReadLimitByte)
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
	w.WriteHeader(http.StatusNoContent)
}

type streamHandler struct {
	finder SenderFinder
	id     types.ID
	cid    types.ID
}

func (h *streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	fromStr := strings.TrimPrefix(r.URL.Path, RaftStreamPrefix+"/")
	from, err := types.IDFromString(fromStr)
	if err != nil {
		log.Printf("rafthttp: path %s cannot be parsed", fromStr)
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}
	s := h.finder.Sender(from)
	if s == nil {
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

	termStr := r.Header.Get("X-Raft-Term")
	term, err := strconv.ParseUint(termStr, 10, 64)
	if err != nil {
		log.Printf("rafthttp: streaming request ignored due to parse term %s error: %v", termStr, err)
		http.Error(w, "invalid term field", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()

	done, err := s.StartStreaming(w.(WriteFlusher), from, term)
	if err != nil {
		log.Printf("rafthttp: streaming request ignored due to start streaming error: %v", err)
		// TODO: consider http status and info here
		http.Error(w, "error enable streaming", http.StatusInternalServerError)
		return
	}
	<-done
}

type writerToResponse interface {
	WriteTo(w http.ResponseWriter)
}
