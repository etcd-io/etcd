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
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	connPerSender = 4
	// senderBufSize is the size of sender buffer, which helps hold the
	// temporary network latency.
	// The size ensures that sender does not drop messages when the network
	// is out of work for less than 1 second in good path.
	senderBufSize = 64

	appRespBatchMs = 50
	propBatchMs    = 10

	ConnReadTimeout  = 5 * time.Second
	ConnWriteTimeout = 5 * time.Second
)

type Sender interface {
	// StartStreaming enables streaming in the sender using the given writer,
	// which provides a fast and efficient way to send appendEntry messages.
	StartStreaming(w WriteFlusher, to types.ID, term uint64) (done <-chan struct{}, err error)
	Update(u string)
	// Send sends the data to the remote node. It is always non-blocking.
	// It may be fail to send data if it returns nil error.
	Send(m raftpb.Message) error
	// Stop performs any necessary finalization and terminates the Sender
	// elegantly.
	Stop()

	// Pause pauses the sender. The sender will simply drops all incoming
	// messages without retruning an error.
	Pause()

	// Resume resumes a paused sender.
	Resume()
}

func NewSender(tr http.RoundTripper, u string, cid types.ID, p Processor, fs *stats.FollowerStats, shouldstop chan struct{}) *sender {
	s := &sender{
		tr:          tr,
		u:           u,
		cid:         cid,
		p:           p,
		fs:          fs,
		shouldstop:  shouldstop,
		batcher:     NewBatcher(100, appRespBatchMs*time.Millisecond),
		propBatcher: NewProposalBatcher(100, propBatchMs*time.Millisecond),
		q:           make(chan []byte, senderBufSize),
	}
	s.wg.Add(connPerSender)
	for i := 0; i < connPerSender; i++ {
		go s.handle()
	}
	return s
}

type sender struct {
	tr         http.RoundTripper
	u          string
	cid        types.ID
	p          Processor
	fs         *stats.FollowerStats
	shouldstop chan struct{}

	strmCln     *streamClient
	batcher     *Batcher
	propBatcher *ProposalBatcher
	strmSrv     *streamServer
	strmSrvMu   sync.Mutex
	q           chan []byte

	paused bool
	mu     sync.RWMutex
	wg     sync.WaitGroup
}

func (s *sender) StartStreaming(w WriteFlusher, to types.ID, term uint64) (<-chan struct{}, error) {
	s.strmSrvMu.Lock()
	defer s.strmSrvMu.Unlock()
	if s.strmSrv != nil {
		// ignore lower-term streaming request
		if term < s.strmSrv.term {
			return nil, fmt.Errorf("out of data streaming request: term %d, request term %d", term, s.strmSrv.term)
		}
		// stop the existing one
		s.strmSrv.stop()
	}
	s.strmSrv = startStreamServer(w, to, term, s.fs)
	return s.strmSrv.stopNotify(), nil
}

func (s *sender) Update(u string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.u = u
}

// TODO (xiangli): reasonable retry logic
func (s *sender) Send(m raftpb.Message) error {
	s.mu.RLock()
	pause := s.paused
	s.mu.RUnlock()
	if pause {
		return nil
	}

	s.maybeStopStream(m.Term)
	if shouldInitStream(m) && !s.hasStreamClient() {
		s.initStream(types.ID(m.From), types.ID(m.To), m.Term)
		s.batcher.Reset(time.Now())
	}

	var err error
	switch {
	case isProposal(m):
		s.propBatcher.Batch(m)
	case canBatch(m) && s.hasStreamClient():
		if !s.batcher.ShouldBatch(time.Now()) {
			err = s.send(m)
		}
	case canUseStream(m):
		if ok := s.tryStream(m); !ok {
			err = s.send(m)
		}
	default:
		err = s.send(m)
	}
	// send out batched MsgProp if needed
	// TODO: it is triggered by all outcoming send now, and it needs
	// more clear solution. Either use separate goroutine to trigger it
	// or use streaming.
	if !s.propBatcher.IsEmpty() {
		t := time.Now()
		if !s.propBatcher.ShouldBatch(t) {
			s.send(s.propBatcher.Message)
			s.propBatcher.Reset(t)
		}
	}
	return err
}

func (s *sender) send(m raftpb.Message) error {
	// TODO: don't block. we should be able to have 1000s
	// of messages out at a time.
	data := pbutil.MustMarshal(&m)
	select {
	case s.q <- data:
		return nil
	default:
		log.Printf("sender: dropping %s because maximal number %d of sender buffer entries to %s has been reached",
			m.Type, senderBufSize, s.u)
		return fmt.Errorf("reach maximal serving")
	}
}

func (s *sender) Stop() {
	close(s.q)
	s.wg.Wait()
	s.strmSrvMu.Lock()
	if s.strmSrv != nil {
		s.strmSrv.stop()
	}
	s.strmSrvMu.Unlock()
	if s.strmCln != nil {
		s.strmCln.stop()
	}
}

func (s *sender) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
}

func (s *sender) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = false
}

func (s *sender) maybeStopStream(term uint64) {
	if s.strmCln != nil && term > s.strmCln.term {
		s.strmCln.stop()
		s.strmCln = nil
	}
	s.strmSrvMu.Lock()
	defer s.strmSrvMu.Unlock()
	if s.strmSrv != nil && term > s.strmSrv.term {
		s.strmSrv.stop()
		s.strmSrv = nil
	}
}

func (s *sender) hasStreamClient() bool {
	return s.strmCln != nil && !s.strmCln.isStopped()
}

func (s *sender) initStream(from, to types.ID, term uint64) {
	strmCln := newStreamClient(from, to, term, s.p)
	s.mu.Lock()
	u := s.u
	s.mu.Unlock()
	if err := strmCln.start(s.tr, u, s.cid); err != nil {
		log.Printf("rafthttp: start stream client error: %v", err)
		return
	}
	s.strmCln = strmCln
}

func (s *sender) tryStream(m raftpb.Message) bool {
	s.strmSrvMu.Lock()
	defer s.strmSrvMu.Unlock()
	if s.strmSrv == nil || m.Term != s.strmSrv.term {
		return false
	}
	if err := s.strmSrv.send(m.Entries); err != nil {
		log.Printf("rafthttp: send stream message error: %v", err)
		s.strmSrv.stop()
		s.strmSrv = nil
		return false
	}
	return true
}

func (s *sender) handle() {
	defer s.wg.Done()
	for d := range s.q {
		start := time.Now()
		err := s.post(d)
		end := time.Now()
		if err != nil {
			s.fs.Fail()
			log.Printf("sender: %v", err)
			continue
		}
		s.fs.Succ(end.Sub(start))
	}
}

// post POSTs a data payload to a url. Returns nil if the POST succeeds,
// error on any failure.
func (s *sender) post(data []byte) error {
	s.mu.RLock()
	req, err := http.NewRequest("POST", s.u, bytes.NewBuffer(data))
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("new request to %s error: %v", s.u, err)
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Etcd-Cluster-ID", s.cid.String())
	resp, err := s.tr.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("error posting to %q: %v", req.URL.String(), err)
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		select {
		case s.shouldstop <- struct{}{}:
		default:
		}
		log.Printf("etcdserver: conflicting cluster ID with the target cluster (%s != %s)", resp.Header.Get("X-Etcd-Cluster-ID"), s.cid)
		return nil
	case http.StatusForbidden:
		select {
		case s.shouldstop <- struct{}{}:
		default:
		}
		log.Println("etcdserver: this member has been permanently removed from the cluster")
		log.Println("etcdserver: the data-dir used by this member must be removed so that this host can be re-added with a new member ID")
		return nil
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}

func isProposal(m raftpb.Message) bool { return m.Type == raftpb.MsgProp }
