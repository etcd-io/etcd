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
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

const (
	streamBufSize = 4096
)

type WriteFlusher interface {
	io.Writer
	http.Flusher
}

type streamServer struct {
	to   types.ID
	term uint64
	fs   *stats.FollowerStats
	q    chan []raftpb.Entry
	done chan struct{}
}

func startStreamServer(w WriteFlusher, to types.ID, term uint64, fs *stats.FollowerStats) *streamServer {
	s := &streamServer{
		to:   to,
		term: term,
		fs:   fs,
		q:    make(chan []raftpb.Entry, streamBufSize),
		done: make(chan struct{}),
	}
	go s.handle(w)
	log.Printf("rafthttp: starting server stream to %s at term %d", to, term)
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

func (s *streamServer) stop() {
	close(s.q)
	<-s.done
}

func (s *streamServer) stopNotify() <-chan struct{} { return s.done }

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

type streamClient struct {
	id   types.ID
	to   types.ID
	term uint64
	p    Processor

	closer io.Closer
	done   chan struct{}
}

func newStreamClient(id, to types.ID, term uint64, p Processor) *streamClient {
	return &streamClient{
		id:   id,
		to:   to,
		term: term,
		p:    p,
		done: make(chan struct{}),
	}
}

// Dial dials to the remote url, and sends streaming request. If it succeeds,
// it returns nil error, and the caller should call Handle function to keep
// receiving appendEntry messages.
func (s *streamClient) start(tr http.RoundTripper, u string, cid types.ID) error {
	uu, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("parse url %s error: %v", u, err)
	}
	uu.Path = path.Join(RaftStreamPrefix, s.id.String())
	req, err := http.NewRequest("GET", uu.String(), nil)
	if err != nil {
		return fmt.Errorf("new request to %s error: %v", u, err)
	}
	req.Header.Set("X-Etcd-Cluster-ID", cid.String())
	req.Header.Set("X-Raft-To", s.to.String())
	req.Header.Set("X-Raft-Term", strconv.FormatUint(s.term, 10))
	resp, err := tr.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("error posting to %q: %v", u, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unhandled http status %d", resp.StatusCode)
	}
	s.closer = resp.Body
	go s.handle(resp.Body)
	log.Printf("rafthttp: starting client stream to %s at term %d", s.to, s.term)
	return nil
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
		if err := s.p.Process(context.TODO(), msg); err != nil {
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
