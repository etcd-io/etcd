package rafthttp

import (
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

const (
	raftPrefix = "/raft"
)

type Raft interface {
	Process(ctx context.Context, m raftpb.Message) error
}

type Transporter interface {
	Handler() http.Handler
	Send(m []raftpb.Message)
	AddPeer(id types.ID, urls []string)
	RemovePeer(id types.ID)
	UpdatePeer(id types.ID, urls []string)
	Stop()
	ShouldStopNotify() <-chan struct{}
}

type Transport struct {
	RoundTripper http.RoundTripper
	ID           types.ID
	ClusterID    types.ID
	Raft         Raft
	ServerStats  *stats.ServerStats
	LeaderStats  *stats.LeaderStats

	mu         sync.RWMutex       // protect the peer map
	peers      map[types.ID]*peer // remote peers
	shouldstop chan struct{}
}

func (t *Transport) Start() {
	t.peers = make(map[types.ID]*peer)
	t.shouldstop = make(chan struct{}, 1)
}

func (t *Transport) Handler() http.Handler {
	h := NewHandler(t.Raft, t.ClusterID)
	sh := NewStreamHandler(t, t.ID, t.ClusterID)
	mux := http.NewServeMux()
	mux.Handle(RaftPrefix, h)
	mux.Handle(RaftStreamPrefix+"/", sh)
	return mux
}

func (t *Transport) Peer(id types.ID) *peer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.peers[id]
}

func (t *Transport) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		// intentionally dropped message
		if m.To == 0 {
			continue
		}
		to := types.ID(m.To)
		p, ok := t.peers[to]
		if !ok {
			log.Printf("etcdserver: send message to unknown receiver %s", to)
			continue
		}

		if m.Type == raftpb.MsgApp {
			t.ServerStats.SendAppendReq(m.Size())
		}

		p.Send(m)
	}
}

func (t *Transport) Stop() {
	for _, p := range t.peers {
		p.Stop()
	}
	if tr, ok := t.RoundTripper.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func (t *Transport) ShouldStopNotify() <-chan struct{} {
	return t.shouldstop
}

func (t *Transport) AddPeer(id types.ID, urls []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.peers[id]; ok {
		return
	}
	// TODO: considering how to switch between all available peer urls
	peerURL := urls[0]
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, raftPrefix)
	fs := t.LeaderStats.Follower(id.String())
	t.peers[id] = NewPeer(t.RoundTripper, u.String(), id, t.ClusterID,
		t.Raft, fs, t.shouldstop)
}

func (t *Transport) RemovePeer(id types.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.peers[id].Stop()
	delete(t.peers, id)
}

func (t *Transport) UpdatePeer(id types.ID, urls []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// TODO: return error or just panic?
	if _, ok := t.peers[id]; !ok {
		return
	}
	peerURL := urls[0]
	u, err := url.Parse(peerURL)
	if err != nil {
		log.Panicf("unexpect peer url %s", peerURL)
	}
	u.Path = path.Join(u.Path, raftPrefix)
	t.peers[id].Update(u.String())
}

// for testing
func (t *Transport) Pause() {
	for _, p := range t.peers {
		p.Pause()
	}
}

func (t *Transport) Resume() {
	for _, p := range t.peers {
		p.Resume()
	}
}
