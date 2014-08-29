package etcdserver

import (
	"reflect"
	"testing"
	"time"
	"code.google.com/p/go.net/context"

	pb "github.com/coreos/etcd/etcdserver2/etcdserverpb"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

func TestClusterOf1(t *testing.T) { testServer(t, 1) }
func TestClusterOf3(t *testing.T) { testServer(t, 3) }

func testServer(t *testing.T, ns int64) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ss := make([]*Server, ns)

	send := func(msgs []raftpb.Message) {
		for _, m := range msgs {
			t.Logf("sending: %+v\n", m)
			ss[m.To].Node.Step(ctx, m)
		}
	}

	peers := make([]int64, ns)
	for i := int64(0); i < ns; i++ {
		peers[i] = i
	}

	var srv *Server
	for i := int64(0); i < ns; i++ {
		n := raft.Start(ctx, i, peers)

		srv = &Server{
			Node:  n,
			Store: store.New(),
			Send:  send,
			Save:  func(_ raftpb.State, _ []raftpb.Entry) {},
		}
		Start(srv)

		ss[i] = srv
	}

	if err := srv.Node.Campaign(ctx); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 10; i++ {
		r := pb.Request{
			Method: "PUT",
			Id:     1,
			Path:   "/foo",
			Val:    "bar",
		}
		resp, err := srv.Do(ctx, r)
		if err != nil {
			t.Fatal(err)
		}

		g, w := resp.Event.Node, &store.NodeExtern{
			Key:           "/foo",
			ModifiedIndex: uint64(i),
			CreatedIndex:  uint64(i),
			Value:         stringp("bar"),
		}

		if !reflect.DeepEqual(g, w) {
			t.Error("value:", *g.Value)
			t.Errorf("g = %+v, w %+v", g, w)
		}
	}

	time.Sleep(10 * time.Millisecond)

	var last interface{}
	for i, sv := range ss {
		sv.Stop()
		g := store.Root(sv.Store)
		if last != nil && !reflect.DeepEqual(last, g) {
			t.Errorf("server %d: Root = %#v, want %#v", i, g, last)
		}
		last = g
	}
}

func stringp(s string) *string { return &s }
