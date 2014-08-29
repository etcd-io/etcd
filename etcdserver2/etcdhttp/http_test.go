package etcdhttp

import (
	"code.google.com/p/go.net/context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	etcdserver "github.com/coreos/etcd/etcdserver2"
	"github.com/coreos/etcd/etcdserver2/etcdserverpb"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

func nopSave(st raftpb.State, ents []raftpb.Entry) {}
func nopSend(m []raftpb.Message)                   {}

func TestSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st := store.New()

	n := raft.Start(ctx, 1, []int64{1})
	n.Campaign(ctx)

	srv := &etcdserver.Server{
		Node:  n,
		Store: st,
		Send:  etcdserver.SendFunc(nopSend),
		Save: func(st raftpb.State, ents []raftpb.Entry) {
			for _, e := range ents {
				var r etcdserverpb.Request
				if err := r.Unmarshal(e.Data); err != nil {
					t.Fatal(err)
				}
			}
		},
	}
	etcdserver.Start(srv)
	defer srv.Stop()

	h := Handler{
		Timeout: time.Hour,
		Server:  srv,
	}

	s := httptest.NewServer(h)
	defer s.Close()

	resp, err := http.PostForm(s.URL+"/keys/foo", url.Values{"value": {"bar"}})
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("StatusCode = %d, expected %d", 201, resp.StatusCode)
	}

	g := new(store.Event)
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		t.Fatal(err)
	}

	w := &store.NodeExtern{
		Key:           "/foo/1",
		Value:         stringp("bar"),
		ModifiedIndex: 1,
		CreatedIndex:  1,
	}
	if !reflect.DeepEqual(g.Node, w) {
		t.Errorf("g = %+v, want %+v", g.Node, w)
	}
}

func stringp(s string) *string { return &s }
