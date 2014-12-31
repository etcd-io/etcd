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
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

const (
	streamBufSize = 4096
)

// TODO: a stream might hava one stream server or one stream client, but not both.
type stream struct {
	// the server might be attached asynchronously with the owner of the stream
	// use a mutex to protect it
	sync.Mutex
	server *streamServer

	client *streamClient
}

func (s *stream) open(from, to, cid types.ID, term uint64, tr http.RoundTripper, u string, r Raft) error {
	if s.client != nil {
		panic("open: stream is open")
	}

	c, err := newStreamClient(from, to, cid, term, tr, u, r)
	if err != nil {
		log.Printf("stream: error opening stream: %v", err)
		return err
	}
	s.client = c
	return nil
}

func (s *stream) attach(server *streamServer) error {
	s.Lock()
	defer s.Unlock()
	if s.server != nil {
		// ignore lower-term streaming request
		if server.term < s.server.term {
			return fmt.Errorf("cannot attach out of data stream server [%d / %d]", server.term, s.server.term)
		}
		s.server.stop()
	}
	s.server = server
	return nil
}

func (s *stream) write(m raftpb.Message) bool {
	s.Lock()
	defer s.Unlock()
	if s.server == nil {
		return false
	}
	if m.Term != s.server.term {
		if m.Term > s.server.term {
			panic("expected server to be invalidated when there is a higher term message")
		}
		return false
	}
	// todo: early unlock?
	if err := s.server.send(m.Entries); err != nil {
		log.Printf("stream: error sending message: %v", err)
		log.Printf("stream: stopping the stream server...")
		s.server.stop()
		s.server = nil
		return false
	}
	return true
}

// invalidate stops the sever/client that is running at
// a term lower than the given term.
func (s *stream) invalidate(term uint64) {
	s.Lock()
	defer s.Unlock()

	if s.server != nil {
		if s.server.term < term {
			s.server.stop()
			s.server = nil
		}
	}
	if s.client != nil {
		if s.client.term < term {
			s.client.stop()
			s.client = nil
		}
	}
}

func (s *stream) stop() {
	s.invalidate(math.MaxUint64)
}

func (s *stream) isOpen() bool {
	if s.client != nil && s.client.isStopped() {
		s.client = nil
	}
	return s.client != nil
}

type WriteFlusher interface {
	io.Writer
	http.Flusher
}

// TODO: rename it to streamWriter.
// TODO: replace fs with stream stats
type streamServer struct {
	to   types.ID
	term uint64
	fs   *stats.FollowerStats
	q    chan []raftpb.Entry
	done chan struct{}
}

// newStreamServer starts and returns a new started stream server.
// The caller should call stop when finished, to shut it down.
func newStreamServer(w WriteFlusher, to types.ID, term uint64) *streamServer {
	s := &streamServer{
		to:   to,
		term: term,
		q:    make(chan []raftpb.Entry, streamBufSize),
		done: make(chan struct{}),
	}
	go s.handle(w)
	return s
}

func (s *streamServer) send(ents []raftpb.Entry) error {
	select {
	case <-s.done:
		return fmt.Errorf("stopped")
	default:
	}
	select {
	case s.q <- ents:
		return nil
	default:
		log.Printf("rafthttp: maximum number of stream buffer entries to %d has been reached", s.to)
		return fmt.Errorf("maximum number of stream buffer entries has been reached")
	}
}

func (s *streamServer) handle(w WriteFlusher) {
	defer func() {
		close(s.done)
		log.Printf("rafthttp: server streaming to %s at term %d has been stopped", s.to, s.term)
	}()

	ew := &entryWriter{w: w}
	for ents := range s.q {
		start := time.Now()
		if err := ew.writeEntries(ents); err != nil {
			log.Printf("rafthttp: encountered error writing to server log stream: %v", err)
			return
		}
		w.Flush()
		s.fs.Succ(time.Since(start))
	}
}

func (s *streamServer) stop() {
	close(s.q)
	<-s.done
}

func (s *streamServer) stopNotify() <-chan struct{} { return s.done }

// TODO: rename it to streamReader.
// TODO: move the raft interface out of the reader.
type streamClient struct {
	id   types.ID
	to   types.ID
	term uint64
	r    Raft

	closer io.Closer
	done   chan struct{}
}

// newStreamClient starts and returns a new started stream client.
// The caller should call stop when finished, to shut it down.
func newStreamClient(id, to, cid types.ID, term uint64, tr http.RoundTripper, u string, r Raft) (*streamClient, error) {
	s := &streamClient{
		id:   id,
		to:   to,
		term: term,
		r:    r,
		done: make(chan struct{}),
	}

	uu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("parse url %s error: %v", u, err)
	}
	uu.Path = path.Join(RaftStreamPrefix, s.id.String())
	req, err := http.NewRequest("GET", uu.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request to %s error: %v", u, err)
	}
	req.Header.Set("X-Etcd-Cluster-ID", cid.String())
	req.Header.Set("X-Raft-To", s.to.String())
	req.Header.Set("X-Raft-Term", strconv.FormatUint(s.term, 10))
	resp, err := tr.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("error posting to %q: %v", u, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unhandled http status %d", resp.StatusCode)
	}
	s.closer = resp.Body
	go s.handle(resp.Body)
	log.Printf("rafthttp: starting client stream to %s at term %d", s.to, s.term)
	return s, nil
}

func (s *streamClient) stop() {
	s.closer.Close()
	<-s.done
}

func (s *streamClient) isStopped() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *streamClient) handle(r io.Reader) {
	defer func() {
		close(s.done)
		log.Printf("rafthttp: client streaming to %s at term %d has been stopped", s.to, s.term)
	}()

	er := &entryReader{r: r}
	for {
		ents, err := er.readEntries()
		if err != nil {
			if err != io.EOF {
				log.Printf("rafthttp: encountered error reading the client log stream: %v", err)
			}
			return
		}
		// Considering Commit in MsgApp is not recovered, zero-entry appendEntry
		// messages have no use to raft state machine. Drop it here because
		// we don't have easy way to recover its Index easily.
		if len(ents) == 0 {
			continue
		}
		// The commit index field in appendEntry message is not recovered.
		// The follower updates its commit index through heartbeat.
		msg := raftpb.Message{
			Type:    raftpb.MsgApp,
			From:    uint64(s.to),
			To:      uint64(s.id),
			Term:    s.term,
			LogTerm: s.term,
			Index:   ents[0].Index - 1,
			Entries: ents,
		}
		if err := s.r.Process(context.TODO(), msg); err != nil {
			log.Printf("rafthttp: process raft message error: %v", err)
			return
		}
	}
}

func shouldInitStream(m raftpb.Message) bool {
	return m.Type == raftpb.MsgAppResp && m.Reject == false
}

func canUseStream(m raftpb.Message) bool {
	return m.Type == raftpb.MsgApp && m.Index > 0 && m.Term == m.LogTerm
}
