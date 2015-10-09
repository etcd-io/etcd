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
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/xiang90/probing"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
)

var plog = capnslog.NewPackageLogger("github.com/coreos/etcd", "rafthttp")

type Raft interface {
	Process(ctx context.Context, m raftpb.Message) error
	IsIDRemoved(id uint64) bool
	ReportUnreachable(id uint64)
	ReportSnapshot(id uint64, status raft.SnapshotStatus)
}

// SnapshotSaver is the interface that wraps the SaveFrom method.
type SnapshotSaver interface {
	// SaveFrom saves the snapshot data at the given index from the given reader.
	SaveFrom(r io.Reader, index uint64) error
}

type Transporter interface {
	// Handler returns the HTTP handler of the transporter.
	// A transporter HTTP handler handles the HTTP requests
	// from remote peers.
	// The handler MUST be used to handle RaftPrefix(/raft)
	// endpoint.
	Handler() http.Handler
	// Send sends out the given messages to the remote peers.
	// Each message has a To field, which is an id that maps
	// to an existing peer in the transport.
	// If the id cannot be found in the transport, the message
	// will be ignored.
	Send(m []raftpb.Message)
	// SetLocalPeerURLs sets the peer urls of local member to the given urls,
	// which specifies the address to receive requests at Transporter's Handler.
	SetLocalPeerURLs(urls []string)
	// AddRemote adds a remote with given peer urls into the transport.
	// A remote helps newly joined member to catch up the progress of cluster,
	// and will not be used after that.
	// It is the caller's responsibility to ensure the urls are all valid,
	// or it panics.
	AddRemote(id types.ID, urls []string)
	// AddPeer adds a peer with given peer urls into the transport.
	// It is the caller's responsibility to ensure the urls are all valid,
	// or it panics.
	// Peer urls are used to connect to the remote peer.
	AddPeer(id types.ID, urls []string)
	// RemovePeer removes the peer with given id.
	RemovePeer(id types.ID)
	// RemoveAllPeers removes all the existing peers in the transport.
	RemoveAllPeers()
	// UpdatePeer updates the peer urls of the peer with the given id.
	// It is the caller's responsibility to ensure the urls are all valid,
	// or it panics.
	UpdatePeer(id types.ID, urls []string)
	// ActiveSince returns the time that the connection with the peer
	// of the given id becomes active.
	// If the connection is active since peer was added, it returns the adding time.
	// If the connection is currently inactive, it returns zero time.
	ActiveSince(id types.ID) time.Time
	// SnapshotReady accepts a snapshot at the given index that is ready to send out.
	// SnapshotReady MUST not be called when the snapshot sent result of previous
	// accepted one has not been reported.
	SnapshotReady(index uint64, w io.WriterTo)
	// Stop closes the connections and stops the transporter.
	Stop()
}

// member represents member information, including member ID and its
// peer URLs.
type member struct {
	id types.ID

	mu sync.Mutex
	us types.URLs
}

func newMember(id types.ID) *member { return &member{id: id} }

func (l *member) urls() types.URLs {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.us
}

func (l *member) setURLs(us types.URLs) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.us = us
}

type transport struct {
	roundTripper http.RoundTripper
	local        *member
	clusterID    types.ID
	raft         Raft
	snapSaver    SnapshotSaver
	serverStats  *stats.ServerStats
	leaderStats  *stats.LeaderStats
	// snapTimeout is the timeout before the kept snapshot is auto released
	snapTimeout time.Duration
	v3demo      bool

	mu      sync.RWMutex         // protect the term, remote and peer map
	term    uint64               // the latest term that has been observed
	remotes map[types.ID]*remote // remotes map that helps newly joined member to catch up
	peers   map[types.ID]Peer    // peers map

	snapKeeper *snapshotKeeper
	prober     probing.Prober
	errorc     chan error
}

func NewTransporter(rt http.RoundTripper, id, cid types.ID, r Raft, snapSaver SnapshotSaver, errorc chan error, ss *stats.ServerStats, ls *stats.LeaderStats, maxRTT time.Duration, v3demo bool) Transporter {
	return &transport{
		roundTripper: rt,
		local:        newMember(id),
		clusterID:    cid,
		raft:         r,
		snapSaver:    snapSaver,
		serverStats:  ss,
		leaderStats:  ls,
		// The timeout needs to be greater than the maximal time that may take
		// before the kept snapshot is retrieved.
		// Before a snapshot is retrieved, its raft snapshot message needs to
		// be sent out, and snapHandler waits remote member requesting
		// snapshot data. The process needs to take two maximal RTT.
		snapTimeout: 2 * maxRTT,
		v3demo:      v3demo,
		remotes:     make(map[types.ID]*remote),
		peers:       make(map[types.ID]Peer),

		snapKeeper: newSnapshotKeeper(r),
		prober:     probing.NewProber(rt),
		errorc:     errorc,
	}
}

func (t *transport) Handler() http.Handler {
	snapProcessor := newSnapshotProcessor(t.roundTripper, t.raft, t.snapSaver)
	pipelineHandler := NewHandler(t.raft, snapProcessor, t.clusterID, t.v3demo)
	streamHandler := newStreamHandler(t, t.raft, t.local.id, t.clusterID)
	snapHandler := newSnapHandler(t.snapKeeper, t.raft)
	mux := http.NewServeMux()
	mux.Handle(RaftPrefix, pipelineHandler)
	mux.Handle(RaftStreamPrefix+"/", streamHandler)
	mux.Handle(RaftSnapPrefix, snapHandler)
	mux.Handle(ProbingPrefix, probing.NewHandler())
	return mux
}

func (t *transport) Get(id types.ID) Peer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.peers[id]
}

func (t *transport) maybeUpdatePeersTerm(term uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term >= term {
		return
	}
	t.term = term
	for _, p := range t.peers {
		p.setTerm(term)
	}
}

func (t *transport) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		// intentionally dropped message
		if m.To == 0 {
			continue
		}
		to := types.ID(m.To)

		if t.v3demo && m.Type == raftpb.MsgSnap {
			if t.snapKeeper.keptIndex() != m.Snapshot.Metadata.Index {
				plog.Panicf("unexpected kept snapshot index = %d, want %d", t.snapKeeper.keptIndex(), m.Snapshot.Metadata.Index)
			}
			t.snapKeeper.attachRemote(to)
			t.snapKeeper.autoRelease(t.snapTimeout)
		}

		if m.Type != raftpb.MsgProp { // proposal message does not have a valid term
			t.maybeUpdatePeersTerm(m.Term)
		}

		p, ok := t.peers[to]
		if ok {
			if m.Type == raftpb.MsgApp {
				t.serverStats.SendAppendReq(m.Size())
			}
			p.Send(m)
			continue
		}

		g, ok := t.remotes[to]
		if ok {
			g.Send(m)
			continue
		}

		plog.Debugf("ignored message %s (sent to unknown peer %s)", m.Type, to)
	}
}

func (t *transport) Stop() {
	for _, r := range t.remotes {
		r.Stop()
	}
	for _, p := range t.peers {
		p.Stop()
	}
	t.prober.RemoveAll()
	if tr, ok := t.roundTripper.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func (t *transport) SetLocalPeerURLs(urls []string) {
	us, err := types.NewURLs(urls)
	if err != nil {
		plog.Panicf("newURLs %+v should never fail: %+v", urls, err)
	}
	t.local.setURLs(us)
}

func (t *transport) AddRemote(id types.ID, us []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.remotes[id]; ok {
		return
	}
	urls, err := types.NewURLs(us)
	if err != nil {
		plog.Panicf("newURLs %+v should never fail: %+v", us, err)
	}
	t.remotes[id] = startRemote(t.roundTripper, urls, t.local, id, t.clusterID, t.raft, t.errorc, t.v3demo)
}

func (t *transport) AddPeer(id types.ID, us []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.peers[id]; ok {
		return
	}
	urls, err := types.NewURLs(us)
	if err != nil {
		plog.Panicf("newURLs %+v should never fail: %+v", us, err)
	}
	fs := t.leaderStats.Follower(id.String())
	t.peers[id] = startPeer(t.roundTripper, urls, t.local, id, t.clusterID, t.raft, fs, t.errorc, t.term, t.v3demo)
	addPeerToProber(t.prober, id.String(), us)
}

func (t *transport) RemovePeer(id types.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removePeer(id)
}

func (t *transport) RemoveAllPeers() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id := range t.peers {
		t.removePeer(id)
	}
}

// the caller of this function must have the peers mutex.
func (t *transport) removePeer(id types.ID) {
	if peer, ok := t.peers[id]; ok {
		peer.Stop()
	} else {
		plog.Panicf("unexpected removal of unknown peer '%d'", id)
	}
	delete(t.peers, id)
	delete(t.leaderStats.Followers, id.String())
	t.prober.Remove(id.String())
}

func (t *transport) UpdatePeer(id types.ID, us []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// TODO: return error or just panic?
	if _, ok := t.peers[id]; !ok {
		return
	}
	urls, err := types.NewURLs(us)
	if err != nil {
		plog.Panicf("newURLs %+v should never fail: %+v", us, err)
	}
	t.peers[id].Update(urls)

	t.prober.Remove(id.String())
	addPeerToProber(t.prober, id.String(), us)
}

func (t *transport) ActiveSince(id types.ID) time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.peers[id]; ok {
		return p.activeSince()
	}
	return time.Time{}
}

func (t *transport) SnapshotReady(index uint64, w io.WriterTo) {
	t.snapKeeper.keep(index, w)
}

type Pausable interface {
	Pause()
	Resume()
}

// for testing
func (t *transport) Pause() {
	for _, p := range t.peers {
		p.(Pausable).Pause()
	}
}

func (t *transport) Resume() {
	for _, p := range t.peers {
		p.(Pausable).Resume()
	}
}
