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
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	appRespBatchMs = 50
	propBatchMs    = 10

	ConnReadTimeout  = 5 * time.Second
	ConnWriteTimeout = 5 * time.Second
)

type peer struct {
	sync.Mutex

	id  types.ID
	cid types.ID

	tr http.RoundTripper
	// the url this sender post to
	u  string
	r  Raft
	fs *stats.FollowerStats

	batcher     *Batcher
	propBatcher *ProposalBatcher

	pipeline *pipeline
	stream   *stream

	paused  bool
	stopped bool
}

func NewPeer(tr http.RoundTripper, u string, id types.ID, cid types.ID, r Raft, fs *stats.FollowerStats, errorc chan error) *peer {
	return &peer{
		id:          id,
		cid:         cid,
		tr:          tr,
		u:           u,
		r:           r,
		fs:          fs,
		pipeline:    newPipeline(tr, u, id, cid, fs, errorc),
		stream:      &stream{},
		batcher:     NewBatcher(100, appRespBatchMs*time.Millisecond),
		propBatcher: NewProposalBatcher(100, propBatchMs*time.Millisecond),
	}
}

func (p *peer) Update(u string) {
	p.Lock()
	defer p.Unlock()
	if p.stopped {
		// TODO: not panic here?
		panic("peer: update a stopped peer")
	}
	p.u = u
	p.pipeline.update(u)
}

// Send sends the data to the remote node. It is always non-blocking.
// It may be fail to send data if it returns nil error.
// TODO (xiangli): reasonable retry logic
func (p *peer) Send(m raftpb.Message) error {
	p.Lock()
	defer p.Unlock()
	if p.stopped {
		return errors.New("peer: stopped")
	}
	if p.paused {
		return nil
	}

	// move all the stream related stuff into stream
	p.stream.invalidate(m.Term)
	if shouldInitStream(m) && !p.stream.isOpen() {
		u := p.u
		// todo: steam open should not block.
		p.stream.open(types.ID(m.From), p.id, p.cid, m.Term, p.tr, u, p.r)
		p.batcher.Reset(time.Now())
	}

	var err error
	switch {
	case isProposal(m):
		p.propBatcher.Batch(m)
	case canBatch(m) && p.stream.isOpen():
		if !p.batcher.ShouldBatch(time.Now()) {
			err = p.pipeline.send(m)
		}
	case canUseStream(m):
		if ok := p.stream.write(m); !ok {
			err = p.pipeline.send(m)
		}
	default:
		err = p.pipeline.send(m)
	}
	// send out batched MsgProp if needed
	// TODO: it is triggered by all outcoming send now, and it needs
	// more clear solution. Either use separate goroutine to trigger it
	// or use streaming.
	if !p.propBatcher.IsEmpty() {
		t := time.Now()
		if !p.propBatcher.ShouldBatch(t) {
			p.pipeline.send(p.propBatcher.Message)
			p.propBatcher.Reset(t)
		}
	}
	return err
}

// Stop performs any necessary finalization and terminates the peer
// elegantly.
func (p *peer) Stop() {
	p.Lock()
	defer p.Unlock()
	p.pipeline.stop()
	p.stream.stop()
	p.stopped = true
}

// attachStream attaches a streamSever to the peer.
func (p *peer) attachStream(sw *streamWriter) error {
	p.Lock()
	defer p.Unlock()
	if p.stopped {
		return errors.New("peer: stopped")
	}

	sw.fs = p.fs
	return p.stream.attach(sw)
}

// Pause pauses the peer. The peer will simply drops all incoming
// messages without retruning an error.
func (p *peer) Pause() {
	p.Lock()
	defer p.Unlock()
	p.paused = true
}

// Resume resumes a paused peer.
func (p *peer) Resume() {
	p.Lock()
	defer p.Unlock()
	p.paused = false
}

func isProposal(m raftpb.Message) bool { return m.Type == raftpb.MsgProp }
