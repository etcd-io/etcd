package rafthttp

import (
	"net/http"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

type Processor interface {
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
	Processor    Processor
	ServerStats  *stats.ServerStats
	LeaderStats  *stats.LeaderStats

	*sendHub
	handler http.Handler
}

func (t *Transport) Start() {
	t.sendHub = newSendHub(t.RoundTripper, t.ClusterID, t.Processor, t.ServerStats, t.LeaderStats)
	h := NewHandler(t.Processor, t.ClusterID)
	sh := NewStreamHandler(t.sendHub, t.ID, t.ClusterID)
	mux := http.NewServeMux()
	mux.Handle(RaftPrefix, h)
	mux.Handle(RaftStreamPrefix+"/", sh)
	t.handler = mux
}

func (t *Transport) Handler() http.Handler { return t.handler }
