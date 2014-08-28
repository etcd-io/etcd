package etcdserver

import (
	"reflect"
	"testing"
	"code.google.com/p/go.net/context"

	pb "github.com/coreos/etcd/etcdserver2/etcdserverpb"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

func TestServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := raft.Start(ctx, 1, []int64{1})
	n.Campaign(ctx)

	srv := &Server{
		Node:  n,
		Store: store.New(),
		Send:  func(_ []raftpb.Message) {},
		Save:  func(_ raftpb.State, _ []raftpb.Entry) {},
	}
	Start(srv)
	defer srv.Stop()

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
		ModifiedIndex: 1,
		CreatedIndex:  1,
		Value:         stringp("bar"),
	}

	if !reflect.DeepEqual(g, w) {
		t.Error("value:", *g.Value)
		t.Errorf("g = %+v, w %+v", g, w)
	}
}

func stringp(s string) *string { return &s }
