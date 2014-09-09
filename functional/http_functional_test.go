package functional

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdhttp"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

func nopSave(st raftpb.State, ents []raftpb.Entry) {}
func nopSend(m []raftpb.Message)                   {}

func TestSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st := store.New()

	n := raft.Start(1, []int64{1}, 0, 0)
	n.Campaign(ctx)

	srv := &etcdserver.Server{
		Node:    n,
		Store:   st,
		Send:    etcdserver.SendFunc(nopSend),
		Storage: NopStorage{},
	}
	etcdserver.Start(srv)
	defer srv.Stop()

	h := etcdhttp.Handler{
		Timeout: time.Hour,
		Server:  srv,
	}

	s := httptest.NewServer(h)
	defer s.Close()

	resp, err := http.PostForm(s.URL+"/v2/keys/foo", url.Values{"value": {"bar"}})
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

type NopStorage struct{}

func (ns NopStorage) Save(st raftpb.State, ents []raftpb.Entry) {}
func (ns NopStorage) Cut(index int64) error                     { return nil }
func (ns NopStorage) SaveSnap(st raftpb.Snapshot)               {}
