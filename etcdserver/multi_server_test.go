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

package etcdserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/etcdserver/idutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestClusterOf1(t *testing.T) { testServer(t, 1) }
func TestClusterOf3(t *testing.T) { testServer(t, 3) }

func testServer(t *testing.T, ns uint64) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stderr)

	ss := make([]*EtcdServer, ns)

	ids := make([]uint64, ns)
	for i := uint64(0); i < ns; i++ {
		ids[i] = i + 1
	}
	members := mustMakePeerSlice(t, ids...)
	for i := uint64(0); i < ns; i++ {
		id := i + 1
		s := raft.NewMemoryStorage()
		n := raft.StartNode(id, members, 10, 1, s)
		tk := time.NewTicker(10 * time.Millisecond)
		defer tk.Stop()
		st := store.New()
		cl := newCluster("abc")
		cl.SetStore(st)
		srv := &EtcdServer{
			node:        n,
			raftStorage: s,
			store:       st,
			transport:   &fakeTransporter{ss: ss},
			storage:     &storageRecorder{},
			Ticker:      tk.C,
			Cluster:     cl,
			reqIDGen:    idutil.NewGenerator(uint8(i), time.Time{}),
		}
		ss[i] = srv
	}

	for i := uint64(0); i < ns; i++ {
		ss[i].start()
	}

	for i := 1; i <= 10; i++ {
		r := pb.Request{
			Method: "PUT",
			Path:   "/foo",
			Val:    "bar",
		}
		j := rand.Intn(len(ss))
		t.Logf("ss = %d", j)
		resp, err := ss[j].Do(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}

		g, w := resp.Event.Node, &store.NodeExtern{
			Key:           "/foo",
			ModifiedIndex: uint64(i) + ns,
			CreatedIndex:  uint64(i) + ns,
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
		g, _ := sv.store.Get("/", true, true)
		if last != nil && !reflect.DeepEqual(last, g) {
			t.Errorf("server %d: Root = %#v, want %#v", i, g, last)
		}
		last = g
	}
}

// TODO: test wait trigger correctness in multi-server case

type fakeTransporter struct {
	nopTransporter
	ss []*EtcdServer
}

func (s *fakeTransporter) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		s.ss[m.To-1].node.Step(context.TODO(), m)
	}
}

func mustMakePeerSlice(t *testing.T, ids ...uint64) []raft.Peer {
	peers := make([]raft.Peer, len(ids))
	for i, id := range ids {
		m := Member{ID: types.ID(id)}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		peers[i] = raft.Peer{ID: id, Context: b}
	}
	return peers
}
