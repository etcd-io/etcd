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
	"github.com/coreos/etcd/pkg/transport"
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
	// Start starts the given Transporter.
	// Start MUST be called before calling other functions in the interface.
	Start() error
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
	// It is expected that caller sends a raft snapshot message with
	// the given index soon, and the accepted snapshot will be sent out
	// together. After sending, snapshot sent status is reported
	// through Raft.SnapshotStatus.
	// SnapshotReady MUST not be called when the snapshot sent status of previous
	// accepted one has not been reported.
	SnapshotReady(rc io.ReadCloser, index uint64)
	// Stop closes the connections and stops the transporter.
	Stop()
}

// Transport implements Transporter interface. It provides the functionality
// to send raft messages to peers, and receive raft messages from peers.
// User should call Handler method to get a handler to serve requests
// received from peerURLs.
// User needs to call Start before calling other functions, and call
// Stop when the Transport is no longer used.
type Transport struct {
	DialTimeout time.Duration     // maximum duration before timing out dial of the request
	TLSInfo     transport.TLSInfo // TLS information used when creating connection

	ID          types.ID           // local member ID
	ClusterID   types.ID           // raft cluster ID for request validation
	Raft        Raft               // raft state machine, to which the Transport forwards received messages and reports status
	SnapSaver   SnapshotSaver      // used to save snapshot in v3 snapshot messages
	ServerStats *stats.ServerStats // used to record general transportation statistics
	// used to record transportation statistics with followers when
	// performing as leader in raft protocol
	LeaderStats *stats.LeaderStats
	// error channel used to report detected critical error, e.g.,
	// the member has been permanently removed from the cluster
	// When an error is received from ErrorC, user should stop raft state
	// machine and thus stop the Transport.
	ErrorC chan error
	V3demo bool

	streamRt   http.RoundTripper // roundTripper used by streams
	pipelineRt http.RoundTripper // roundTripper used by pipelines

	mu      sync.RWMutex         // protect the remote and peer map
	remotes map[types.ID]*remote // remotes map that helps newly joined member to catch up
	peers   map[types.ID]Peer    // peers map

	snapst *snapshotStore

	prober probing.Prober
}

func (t *Transport) Start() error {
	var err error
	// Read/write timeout is set for stream roundTripper to promptly
	// find out broken status, which minimizes the number of messages
	// sent on broken connection.
	t.streamRt, err = transport.NewTimeoutTransport(t.TLSInfo, t.DialTimeout, ConnReadTimeout, ConnWriteTimeout)
	if err != nil {
		return err
	}
	t.pipelineRt, err = transport.NewTransport(t.TLSInfo, t.DialTimeout)
	if err != nil {
		return err
	}
	t.remotes = make(map[types.ID]*remote)
	t.peers = make(map[types.ID]Peer)
	t.snapst = &snapshotStore{}
	t.prober = probing.NewProber(t.pipelineRt)
	return nil
}

func (t *Transport) Handler() http.Handler {
	pipelineHandler := newPipelineHandler(t.Raft, t.ClusterID)
	streamHandler := newStreamHandler(t, t.Raft, t.ID, t.ClusterID)
	snapHandler := newSnapshotHandler(t.Raft, t.SnapSaver, t.ClusterID)
	mux := http.NewServeMux()
	mux.Handle(RaftPrefix, pipelineHandler)
	mux.Handle(RaftStreamPrefix+"/", streamHandler)
	mux.Handle(RaftSnapshotPrefix, snapHandler)
	mux.Handle(ProbingPrefix, probing.NewHandler())
	return mux
}

func (t *Transport) Get(id types.ID) Peer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.peers[id]
}

func (t *Transport) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		if m.To == 0 {
			// ignore intentionally dropped message
			continue
		}
		to := types.ID(m.To)

		t.mu.RLock()
		p, ok := t.peers[to]
		t.mu.RUnlock()

		if ok {
			if m.Type == raftpb.MsgApp {
				t.ServerStats.SendAppendReq(m.Size())
			}
			p.send(m)
			continue
		}

		g, ok := t.remotes[to]
		if ok {
			g.send(m)
			continue
		}

		plog.Debugf("ignored message %s (sent to unknown peer %s)", m.Type, to)
	}
}

func (t *Transport) Stop() {
	for _, r := range t.remotes {
		r.stop()
	}
	for _, p := range t.peers {
		p.stop()
	}
	t.prober.RemoveAll()
	if tr, ok := t.streamRt.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	if tr, ok := t.pipelineRt.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func (t *Transport) AddRemote(id types.ID, us []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.remotes[id]; ok {
		return
	}
	urls, err := types.NewURLs(us)
	if err != nil {
		plog.Panicf("newURLs %+v should never fail: %+v", us, err)
	}
	t.remotes[id] = startRemote(t.pipelineRt, urls, t.ID, id, t.ClusterID, t.Raft, t.ErrorC)
}

func (t *Transport) AddPeer(id types.ID, us []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.peers[id]; ok {
		return
	}
	urls, err := types.NewURLs(us)
	if err != nil {
		plog.Panicf("newURLs %+v should never fail: %+v", us, err)
	}
	fs := t.LeaderStats.Follower(id.String())
	t.peers[id] = startPeer(t.streamRt, t.pipelineRt, urls, t.ID, id, t.ClusterID, t.snapst, t.Raft, fs, t.ErrorC, t.V3demo)
	addPeerToProber(t.prober, id.String(), us)
}

func (t *Transport) RemovePeer(id types.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removePeer(id)
}

func (t *Transport) RemoveAllPeers() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id := range t.peers {
		t.removePeer(id)
	}
}

// the caller of this function must have the peers mutex.
func (t *Transport) removePeer(id types.ID) {
	if peer, ok := t.peers[id]; ok {
		peer.stop()
	} else {
		plog.Panicf("unexpected removal of unknown peer '%d'", id)
	}
	delete(t.peers, id)
	delete(t.LeaderStats.Followers, id.String())
	t.prober.Remove(id.String())
}

func (t *Transport) UpdatePeer(id types.ID, us []string) {
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
	t.peers[id].update(urls)

	t.prober.Remove(id.String())
	addPeerToProber(t.prober, id.String(), us)
}

func (t *Transport) ActiveSince(id types.ID) time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.peers[id]; ok {
		return p.activeSince()
	}
	return time.Time{}
}

func (t *Transport) SnapshotReady(rc io.ReadCloser, index uint64) {
	t.snapst.put(rc, index)
}

type Pausable interface {
	Pause()
	Resume()
}

// for testing
func (t *Transport) Pause() {
	for _, p := range t.peers {
		p.(Pausable).Pause()
	}
}

func (t *Transport) Resume() {
	for _, p := range t.peers {
		p.(Pausable).Resume()
	}
}
