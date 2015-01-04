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
	"errors"
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

type peer struct {
	sync.Mutex

	id  types.ID
	cid types.ID

	tr         http.RoundTripper
	r          Raft
	fs         *stats.FollowerStats
	shouldstop chan struct{}

	batcher     *Batcher
	propBatcher *ProposalBatcher
	q           chan *raftpb.Message

	stream *stream

	// wait for the handling routines
	wg sync.WaitGroup

	// the url this sender post to
	u string
	// if the last send was successful, the sender is active.
	// Or it is inactive
	active  bool
	errored error
	paused  bool
	stopped bool
}

func NewPeer(tr http.RoundTripper, u string, id types.ID, cid types.ID, r Raft, fs *stats.FollowerStats, shouldstop chan struct{}) *peer {
	p := &peer{
		id:          id,
		active:      true,
		tr:          tr,
		u:           u,
		cid:         cid,
		r:           r,
		fs:          fs,
		stream:      &stream{},
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

func (p *peer) Update(u string) {
	p.Lock()
	defer p.Unlock()
	if p.stopped {
		// TODO: not panic here?
		panic("peer: update a stopped peer")
	}
	p.u = u
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
			err = p.send(m)
		}
	case canUseStream(m):
		if ok := p.stream.write(m); !ok {
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

	p.Lock()
	defer p.Unlock()
	p.stream.stop()
	p.stopped = true
}

func (p *peer) handle() {
	defer p.wg.Done()
	for m := range p.q {
		start := time.Now()
		err := p.post(pbutil.MustMarshal(m))
		end := time.Now()

		p.Lock()
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
		p.Unlock()
	}
}

// post POSTs a data payload to a url. Returns nil if the POST succeeds,
// error on any failure.
func (p *peer) post(data []byte) error {
	p.Lock()
	req, err := http.NewRequest("POST", p.u, bytes.NewBuffer(data))
	p.Unlock()
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
