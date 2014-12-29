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

func NewPeer(tr http.RoundTripper, u string, id types.ID, cid types.ID, r Raft, fs *stats.FollowerStats, shouldstop chan struct{}) *peer {
	p := &peer{
		id:          id,
		active:      true,
		tr:          tr,
		u:           u,
		cid:         cid,
		r:           r,
		fs:          fs,
		shouldstop:  shouldstop,
		batcher:     NewBatcher(100, appRespBatchMs*time.Millisecond),
		propBatcher: NewProposalBatcher(100, propBatchMs*time.Millisecond),
		q:           make(chan *raftpb.Message, senderBufSize),
	}
	p.wg.Add(connPerSender)
	for i := 0; i < connPerSender; i++ {
		go p.handle()
	}
	return p
}

type peer struct {
	id  types.ID
	cid types.ID

	tr         http.RoundTripper
	r          Raft
	fs         *stats.FollowerStats
	shouldstop chan struct{}

	strmCln     *streamClient
	batcher     *Batcher
	propBatcher *ProposalBatcher
	q           chan *raftpb.Message

	strmSrvMu sync.Mutex
	strmSrv   *streamServer

	// wait for the handling routines
	wg sync.WaitGroup

	mu sync.RWMutex
	u  string // the url this sender post to
	// if the last send was successful, thi sender is active.
	// Or it is inactive
	active  bool
	errored error
	paused  bool
}

// StartStreaming enables streaming in the peer using the given writer,
// which provides a fast and efficient way to send appendEntry messages.
func (p *peer) StartStreaming(w WriteFlusher, to types.ID, term uint64) (<-chan struct{}, error) {
	p.strmSrvMu.Lock()
	defer p.strmSrvMu.Unlock()
	if p.strmSrv != nil {
		// ignore lower-term streaming request
		if term < p.strmSrv.term {
			return nil, fmt.Errorf("out of data streaming request: term %d, request term %d", term, p.strmSrv.term)
		}
		// stop the existing one
		p.strmSrv.stop()
		p.strmSrv = nil
	}
	p.strmSrv = startStreamServer(w, to, term, p.fs)
	return p.strmSrv.stopNotify(), nil
}

func (p *peer) Update(u string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.u = u
}

// Send sends the data to the remote node. It is always non-blocking.
// It may be fail to send data if it returns nil error.
// TODO (xiangli): reasonable retry logic
func (p *peer) Send(m raftpb.Message) error {
	p.mu.RLock()
	pause := p.paused
	p.mu.RUnlock()
	if pause {
		return nil
	}

	p.maybeStopStream(m.Term)
	if shouldInitStream(m) && !p.hasStreamClient() {
		p.initStream(types.ID(m.From), types.ID(m.To), m.Term)
		p.batcher.Reset(time.Now())
	}

	var err error
	switch {
	case isProposal(m):
		p.propBatcher.Batch(m)
	case canBatch(m) && p.hasStreamClient():
		if !p.batcher.ShouldBatch(time.Now()) {
			err = p.send(m)
		}
	case canUseStream(m):
		if ok := p.tryStream(m); !ok {
			err = p.send(m)
		}
	default:
		err = p.send(m)
	}
	// send out batched MsgProp if needed
	// TODO: it is triggered by all outcoming send now, and it needs
	// more clear solution. Either use separate goroutine to trigger it
	// or use streaming.
	if !p.propBatcher.IsEmpty() {
		t := time.Now()
		if !p.propBatcher.ShouldBatch(t) {
			p.send(p.propBatcher.Message)
			p.propBatcher.Reset(t)
		}
	}
	return err
}

func (p *peer) send(m raftpb.Message) error {
	// TODO: don't block. we should be able to have 1000s
	// of messages out at a time.
	select {
	case p.q <- &m:
		return nil
	default:
		log.Printf("sender: dropping %s because maximal number %d of sender buffer entries to %s has been reached",
			m.Type, senderBufSize, p.u)
		return fmt.Errorf("reach maximal serving")
	}
}

// Stop performs any necessary finalization and terminates the peer
// elegantly.
func (p *peer) Stop() {
	close(p.q)
	p.wg.Wait()
	p.strmSrvMu.Lock()
	if p.strmSrv != nil {
		p.strmSrv.stop()
		p.strmSrv = nil
	}
	p.strmSrvMu.Unlock()
	if p.strmCln != nil {
		p.strmCln.stop()
	}
}

// Pause pauses the peer. The peer will simply drops all incoming
// messages without retruning an error.
func (p *peer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

// Resume resumes a paused peer.
func (p *peer) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = false
}

func (p *peer) maybeStopStream(term uint64) {
	if p.strmCln != nil && term > p.strmCln.term {
		p.strmCln.stop()
		p.strmCln = nil
	}
	p.strmSrvMu.Lock()
	defer p.strmSrvMu.Unlock()
	if p.strmSrv != nil && term > p.strmSrv.term {
		p.strmSrv.stop()
		p.strmSrv = nil
	}
}

func (p *peer) hasStreamClient() bool {
	return p.strmCln != nil && !p.strmCln.isStopped()
}

func (p *peer) initStream(from, to types.ID, term uint64) {
	strmCln := newStreamClient(from, to, term, p.r)
	p.mu.Lock()
	u := p.u
	p.mu.Unlock()
	if err := strmCln.start(p.tr, u, p.cid); err != nil {
		log.Printf("rafthttp: start stream client error: %v", err)
		return
	}
	p.strmCln = strmCln
}

func (p *peer) tryStream(m raftpb.Message) bool {
	p.strmSrvMu.Lock()
	defer p.strmSrvMu.Unlock()
	if p.strmSrv == nil || m.Term != p.strmSrv.term {
		return false
	}
	if err := p.strmSrv.send(m.Entries); err != nil {
		log.Printf("rafthttp: send stream message error: %v", err)
		p.strmSrv.stop()
		p.strmSrv = nil
		return false
	}
	return true
}

func (p *peer) handle() {
	defer p.wg.Done()
	for m := range p.q {
		start := time.Now()
		err := p.post(pbutil.MustMarshal(m))
		end := time.Now()

		p.mu.Lock()
		if err != nil {
			if p.errored == nil || p.errored.Error() != err.Error() {
				log.Printf("sender: error posting to %s: %v", p.id, err)
				p.errored = err
			}
			if p.active {
				log.Printf("sender: the connection with %s becomes inactive", p.id)
				p.active = false
			}
			if m.Type == raftpb.MsgApp {
				p.fs.Fail()
			}
		} else {
			if !p.active {
				log.Printf("sender: the connection with %s becomes active", p.id)
				p.active = true
				p.errored = nil
			}
			if m.Type == raftpb.MsgApp {
				p.fs.Succ(end.Sub(start))
			}
		}
		p.mu.Unlock()
	}
}

// post POSTs a data payload to a url. Returns nil if the POST succeeds,
// error on any failure.
func (p *peer) post(data []byte) error {
	p.mu.RLock()
	req, err := http.NewRequest("POST", p.u, bytes.NewBuffer(data))
	p.mu.RUnlock()
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Etcd-Cluster-ID", p.cid.String())
	resp, err := p.tr.RoundTrip(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		select {
		case p.shouldstop <- struct{}{}:
		default:
		}
		log.Printf("rafthttp: conflicting cluster ID with the target cluster (%s != %s)", resp.Header.Get("X-Etcd-Cluster-ID"), p.cid)
		return nil
	case http.StatusForbidden:
		select {
		case p.shouldstop <- struct{}{}:
		default:
		}
		log.Println("rafthttp: this member has been permanently removed from the cluster")
		log.Println("rafthttp: the data-dir used by this member must be removed so that this host can be re-added with a new member ID")
		return nil
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}

func isProposal(m raftpb.Message) bool { return m.Type == raftpb.MsgProp }
