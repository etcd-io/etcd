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
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"path"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
	"github.com/coreos/etcd/store"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func TestGetExpirationTime(t *testing.T) {
	tests := []struct {
		r    pb.Request
		want time.Time
	}{
		{
			pb.Request{Expiration: 0},
			time.Time{},
		},
		{
			pb.Request{Expiration: 60000},
			time.Unix(0, 60000),
		},
		{
			pb.Request{Expiration: -60000},
			time.Unix(0, -60000),
		},
	}

	for i, tt := range tests {
		got := getExpirationTime(&tt.r)
		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("#%d: incorrect expiration time: want=%v got=%v", i, tt.want, got)
		}
	}
}

// TestDoLocalAction tests requests which do not need to go through raft to be applied,
// and are served through local data.
func TestDoLocalAction(t *testing.T) {
	tests := []struct {
		req pb.Request

		wresp    Response
		werr     error
		wactions []action
	}{
		{
			pb.Request{Method: "GET", ID: 1, Wait: true},
			Response{Watcher: &stubWatcher{}}, nil, []action{action{name: "Watch"}},
		},
		{
			pb.Request{Method: "GET", ID: 1},
			Response{Event: &store.Event{}}, nil,
			[]action{
				action{
					name:   "Get",
					params: []interface{}{"", false, false},
				},
			},
		},
		{
			pb.Request{Method: "HEAD", ID: 1},
			Response{Event: &store.Event{}}, nil,
			[]action{
				action{
					name:   "Get",
					params: []interface{}{"", false, false},
				},
			},
		},
		{
			pb.Request{Method: "BADMETHOD", ID: 1},
			Response{}, ErrUnknownMethod, []action{},
		},
	}
	for i, tt := range tests {
		st := &storeRecorder{}
		srv := &EtcdServer{store: st}
		resp, err := srv.Do(context.TODO(), tt.req)

		if err != tt.werr {
			t.Fatalf("#%d: err = %+v, want %+v", i, err, tt.werr)
		}
		if !reflect.DeepEqual(resp, tt.wresp) {
			t.Errorf("#%d: resp = %+v, want %+v", i, resp, tt.wresp)
		}
		gaction := st.Action()
		if !reflect.DeepEqual(gaction, tt.wactions) {
			t.Errorf("#%d: action = %+v, want %+v", i, gaction, tt.wactions)
		}
	}
}

// TestDoBadLocalAction tests server requests which do not need to go through consensus,
// and return errors when they fetch from local data.
func TestDoBadLocalAction(t *testing.T) {
	storeErr := fmt.Errorf("bah")
	tests := []struct {
		req pb.Request

		wactions []action
	}{
		{
			pb.Request{Method: "GET", ID: 1, Wait: true},
			[]action{action{name: "Watch"}},
		},
		{
			pb.Request{Method: "GET", ID: 1},
			[]action{action{name: "Get"}},
		},
		{
			pb.Request{Method: "HEAD", ID: 1},
			[]action{action{name: "Get"}},
		},
	}
	for i, tt := range tests {
		st := &errStoreRecorder{err: storeErr}
		srv := &EtcdServer{store: st}
		resp, err := srv.Do(context.Background(), tt.req)

		if err != storeErr {
			t.Fatalf("#%d: err = %+v, want %+v", i, err, storeErr)
		}
		if !reflect.DeepEqual(resp, Response{}) {
			t.Errorf("#%d: resp = %+v, want %+v", i, resp, Response{})
		}
		gaction := st.Action()
		if !reflect.DeepEqual(gaction, tt.wactions) {
			t.Errorf("#%d: action = %+v, want %+v", i, gaction, tt.wactions)
		}
	}
}

func TestApplyRequest(t *testing.T) {
	tests := []struct {
		req pb.Request

		wresp    Response
		wactions []action
	}{
		// POST ==> Create
		{
			pb.Request{Method: "POST", ID: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Create",
					params: []interface{}{"", false, "", true, time.Time{}},
				},
			},
		},
		// POST ==> Create, with expiration
		{
			pb.Request{Method: "POST", ID: 1, Expiration: 1337},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Create",
					params: []interface{}{"", false, "", true, time.Unix(0, 1337)},
				},
			},
		},
		// POST ==> Create, with dir
		{
			pb.Request{Method: "POST", ID: 1, Dir: true},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Create",
					params: []interface{}{"", true, "", true, time.Time{}},
				},
			},
		},
		// PUT ==> Set
		{
			pb.Request{Method: "PUT", ID: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Set",
					params: []interface{}{"", false, "", time.Time{}},
				},
			},
		},
		// PUT ==> Set, with dir
		{
			pb.Request{Method: "PUT", ID: 1, Dir: true},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Set",
					params: []interface{}{"", true, "", time.Time{}},
				},
			},
		},
		// PUT with PrevExist=true ==> Update
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: boolp(true)},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Update",
					params: []interface{}{"", "", time.Time{}},
				},
			},
		},
		// PUT with PrevExist=false ==> Create
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: boolp(false)},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Create",
					params: []interface{}{"", false, "", false, time.Time{}},
				},
			},
		},
		// PUT with PrevExist=true *and* PrevIndex set ==> Update
		// TODO(jonboulle): is this expected?!
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: boolp(true), PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Update",
					params: []interface{}{"", "", time.Time{}},
				},
			},
		},
		// PUT with PrevExist=false *and* PrevIndex set ==> Create
		// TODO(jonboulle): is this expected?!
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: boolp(false), PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Create",
					params: []interface{}{"", false, "", false, time.Time{}},
				},
			},
		},
		// PUT with PrevIndex set ==> CompareAndSwap
		{
			pb.Request{Method: "PUT", ID: 1, PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "CompareAndSwap",
					params: []interface{}{"", "", uint64(1), "", time.Time{}},
				},
			},
		},
		// PUT with PrevValue set ==> CompareAndSwap
		{
			pb.Request{Method: "PUT", ID: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "CompareAndSwap",
					params: []interface{}{"", "bar", uint64(0), "", time.Time{}},
				},
			},
		},
		// PUT with PrevIndex and PrevValue set ==> CompareAndSwap
		{
			pb.Request{Method: "PUT", ID: 1, PrevIndex: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "CompareAndSwap",
					params: []interface{}{"", "bar", uint64(1), "", time.Time{}},
				},
			},
		},
		// DELETE ==> Delete
		{
			pb.Request{Method: "DELETE", ID: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Delete",
					params: []interface{}{"", false, false},
				},
			},
		},
		// DELETE with PrevIndex set ==> CompareAndDelete
		{
			pb.Request{Method: "DELETE", ID: 1, PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "CompareAndDelete",
					params: []interface{}{"", "", uint64(1)},
				},
			},
		},
		// DELETE with PrevValue set ==> CompareAndDelete
		{
			pb.Request{Method: "DELETE", ID: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "CompareAndDelete",
					params: []interface{}{"", "bar", uint64(0)},
				},
			},
		},
		// DELETE with PrevIndex *and* PrevValue set ==> CompareAndDelete
		{
			pb.Request{Method: "DELETE", ID: 1, PrevIndex: 5, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "CompareAndDelete",
					params: []interface{}{"", "bar", uint64(5)},
				},
			},
		},
		// QGET ==> Get
		{
			pb.Request{Method: "QGET", ID: 1},
			Response{Event: &store.Event{}},
			[]action{
				action{
					name:   "Get",
					params: []interface{}{"", false, false},
				},
			},
		},
		// SYNC ==> DeleteExpiredKeys
		{
			pb.Request{Method: "SYNC", ID: 1},
			Response{},
			[]action{
				action{
					name:   "DeleteExpiredKeys",
					params: []interface{}{time.Unix(0, 0)},
				},
			},
		},
		{
			pb.Request{Method: "SYNC", ID: 1, Time: 12345},
			Response{},
			[]action{
				action{
					name:   "DeleteExpiredKeys",
					params: []interface{}{time.Unix(0, 12345)},
				},
			},
		},
		// Unknown method - error
		{
			pb.Request{Method: "BADMETHOD", ID: 1},
			Response{err: ErrUnknownMethod},
			[]action{},
		},
	}

	for i, tt := range tests {
		st := &storeRecorder{}
		srv := &EtcdServer{store: st}
		resp := srv.applyRequest(tt.req)

		if !reflect.DeepEqual(resp, tt.wresp) {
			t.Errorf("#%d: resp = %+v, want %+v", i, resp, tt.wresp)
		}
		gaction := st.Action()
		if !reflect.DeepEqual(gaction, tt.wactions) {
			t.Errorf("#%d: action = %#v, want %#v", i, gaction, tt.wactions)
		}
	}
}

func TestApplyRequestOnAdminMemberAttributes(t *testing.T) {
	cl := newTestCluster([]*Member{{ID: 1}})
	srv := &EtcdServer{
		store:   &storeRecorder{},
		Cluster: cl,
	}
	req := pb.Request{
		Method: "PUT",
		ID:     1,
		Path:   path.Join(storeMembersPrefix, strconv.FormatUint(1, 16), attributesSuffix),
		Val:    `{"Name":"abc","ClientURLs":["http://127.0.0.1:4001"]}`,
	}
	srv.applyRequest(req)
	w := Attributes{Name: "abc", ClientURLs: []string{"http://127.0.0.1:4001"}}
	if g := cl.Member(1).Attributes; !reflect.DeepEqual(g, w) {
		t.Errorf("attributes = %v, want %v", g, w)
	}
}

// TODO: test ErrIDRemoved
func TestApplyConfChangeError(t *testing.T) {
	cl := newCluster("")
	cl.SetStore(store.New())
	for i := 1; i <= 4; i++ {
		cl.AddMember(&Member{ID: types.ID(i)})
	}
	cl.RemoveMember(4)

	tests := []struct {
		cc   raftpb.ConfChange
		werr error
	}{
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeAddNode,
				NodeID: 4,
			},
			ErrIDRemoved,
		},
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeUpdateNode,
				NodeID: 4,
			},
			ErrIDRemoved,
		},
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeAddNode,
				NodeID: 1,
			},
			ErrIDExists,
		},
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeRemoveNode,
				NodeID: 5,
			},
			ErrIDNotFound,
		},
	}
	for i, tt := range tests {
		n := &nodeRecorder{}
		srv := &EtcdServer{
			node:    n,
			Cluster: cl,
		}
		_, err := srv.applyConfChange(tt.cc, nil)
		if err != tt.werr {
			t.Errorf("#%d: applyConfChange error = %v, want %v", i, err, tt.werr)
		}
		cc := raftpb.ConfChange{Type: tt.cc.Type, NodeID: raft.None}
		w := []action{
			{
				name:   "ApplyConfChange",
				params: []interface{}{cc},
			},
		}
		if g := n.Action(); !reflect.DeepEqual(g, w) {
			t.Errorf("#%d: action = %+v, want %+v", i, g, w)
		}
	}
}

func TestApplyConfChangeShouldStop(t *testing.T) {
	cl := newCluster("")
	cl.SetStore(store.New())
	for i := 1; i <= 3; i++ {
		cl.AddMember(&Member{ID: types.ID(i)})
	}
	srv := &EtcdServer{
		id:      1,
		node:    &nodeRecorder{},
		Cluster: cl,
		sendhub: &nopSender{},
	}
	cc := raftpb.ConfChange{
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: 2,
	}
	// remove non-local member
	shouldStop, err := srv.applyConfChange(cc, &raftpb.ConfState{})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if shouldStop != false {
		t.Errorf("shouldStop = %t, want %t", shouldStop, false)
	}

	// remove local member
	cc.NodeID = 1
	shouldStop, err = srv.applyConfChange(cc, &raftpb.ConfState{})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if shouldStop != true {
		t.Errorf("shouldStop = %t, want %t", shouldStop, true)
	}
}

func TestClusterOf1(t *testing.T) { testServer(t, 1) }
func TestClusterOf3(t *testing.T) { testServer(t, 3) }

type fakeSender struct {
	ss []*EtcdServer
}

func (s *fakeSender) Sender(id types.ID) rafthttp.Sender { return nil }
func (s *fakeSender) Send(msgs []raftpb.Message) {
	for _, m := range msgs {
		s.ss[m.To-1].node.Step(context.TODO(), m)
	}
}
func (s *fakeSender) Add(m *Member)                     {}
func (s *fakeSender) Update(m *Member)                  {}
func (s *fakeSender) Remove(id types.ID)                {}
func (s *fakeSender) Stop()                             {}
func (s *fakeSender) ShouldStopNotify() <-chan struct{} { return nil }

func testServer(t *testing.T, ns uint64) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			sendhub:     &fakeSender{ss},
			storage:     &storageRecorder{},
			Ticker:      tk.C,
			Cluster:     cl,
		}
		ss[i] = srv
	}

	// Start the servers after they're all created to avoid races in send().
	for i := uint64(0); i < ns; i++ {
		ss[i].start()
	}

	for i := 1; i <= 10; i++ {
		r := pb.Request{
			Method: "PUT",
			ID:     uint64(i),
			Path:   "/foo",
			Val:    "bar",
		}
		j := rand.Intn(len(ss))
		t.Logf("ss = %d", j)
		resp, err := ss[j].Do(ctx, r)
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

func TestDoProposal(t *testing.T) {
	tests := []pb.Request{
		pb.Request{Method: "POST", ID: 1},
		pb.Request{Method: "PUT", ID: 1},
		pb.Request{Method: "DELETE", ID: 1},
		pb.Request{Method: "GET", ID: 1, Quorum: true},
	}

	for i, tt := range tests {
		ctx, _ := context.WithCancel(context.Background())
		s := raft.NewMemoryStorage()
		n := raft.StartNode(0xBAD0, mustMakePeerSlice(t, 0xBAD0), 10, 1, s)
		st := &storeRecorder{}
		tk := make(chan time.Time)
		// this makes <-tk always successful, which accelerates internal clock
		close(tk)
		cl := newCluster("abc")
		cl.SetStore(store.New())
		srv := &EtcdServer{
			node:        n,
			raftStorage: s,
			store:       st,
			sendhub:     &nopSender{},
			storage:     &storageRecorder{},
			Ticker:      tk,
			Cluster:     cl,
		}
		srv.start()
		resp, err := srv.Do(ctx, tt)
		srv.Stop()

		action := st.Action()
		if len(action) != 1 {
			t.Errorf("#%d: len(action) = %d, want 1", i, len(action))
		}
		if err != nil {
			t.Fatalf("#%d: err = %v, want nil", i, err)
		}
		wresp := Response{Event: &store.Event{}}
		if !reflect.DeepEqual(resp, wresp) {
			t.Errorf("#%d: resp = %v, want %v", i, resp, wresp)
		}
	}
}

func TestDoProposalCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// node cannot make any progress because there are two nodes
	s := raft.NewMemoryStorage()
	n := raft.StartNode(0xBAD0, mustMakePeerSlice(t, 0xBAD0, 0xBAD1), 10, 1, s)
	st := &storeRecorder{}
	wait := &waitRecorder{}
	srv := &EtcdServer{
		// TODO: use fake node for better testability
		node:        n,
		raftStorage: s,
		store:       st,
		w:           wait,
	}

	done := make(chan struct{})
	var err error
	go func() {
		_, err = srv.Do(ctx, pb.Request{Method: "PUT", ID: 1})
		close(done)
	}()
	cancel()
	<-done

	gaction := st.Action()
	if len(gaction) != 0 {
		t.Errorf("len(action) = %v, want 0", len(gaction))
	}
	if err != ErrCanceled {
		t.Fatalf("err = %v, want %v", err, ErrCanceled)
	}
	w := []action{action{name: "Register1"}, action{name: "Trigger1"}}
	if !reflect.DeepEqual(wait.action, w) {
		t.Errorf("wait.action = %+v, want %+v", wait.action, w)
	}
}

func TestDoProposalTimeout(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), 0)
	srv := &EtcdServer{
		node: &nodeRecorder{},
		w:    &waitRecorder{},
	}
	_, err := srv.Do(ctx, pb.Request{Method: "PUT", ID: 1})
	if err != ErrTimeout {
		t.Fatalf("err = %v, want %v", err, ErrTimeout)
	}
}

func TestDoProposalStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// node cannot make any progress because there are two nodes
	s := raft.NewMemoryStorage()
	n := raft.StartNode(0xBAD0, mustMakePeerSlice(t, 0xBAD0, 0xBAD1), 10, 1, s)
	st := &storeRecorder{}
	tk := make(chan time.Time)
	// this makes <-tk always successful, which accelarates internal clock
	close(tk)
	cl := newCluster("abc")
	cl.SetStore(store.New())
	srv := &EtcdServer{
		// TODO: use fake node for better testability
		node:        n,
		raftStorage: s,
		store:       st,
		sendhub:     &nopSender{},
		storage:     &storageRecorder{},
		Ticker:      tk,
		Cluster:     cl,
	}
	srv.start()

	done := make(chan struct{})
	var err error
	go func() {
		_, err = srv.Do(ctx, pb.Request{Method: "PUT", ID: 1})
		close(done)
	}()
	srv.Stop()
	<-done

	action := st.Action()
	if len(action) != 0 {
		t.Errorf("len(action) = %v, want 0", len(action))
	}
	if err != ErrStopped {
		t.Errorf("err = %v, want %v", err, ErrStopped)
	}
}

// TestSync tests sync 1. is nonblocking 2. sends out SYNC request.
func TestSync(t *testing.T) {
	n := &nodeProposeDataRecorder{}
	srv := &EtcdServer{
		node: n,
	}
	done := make(chan struct{})
	go func() {
		srv.sync(10 * time.Second)
		close(done)
	}()

	// check that sync is non-blocking
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("sync should be non-blocking but did not return after 1s!")
	}

	testutil.ForceGosched()
	data := n.data()
	if len(data) != 1 {
		t.Fatalf("len(proposeData) = %d, want 1", len(data))
	}
	var r pb.Request
	if err := r.Unmarshal(data[0]); err != nil {
		t.Fatalf("unmarshal request error: %v", err)
	}
	if r.Method != "SYNC" {
		t.Errorf("method = %s, want SYNC", r.Method)
	}
}

// TestSyncTimeout tests the case that sync 1. is non-blocking 2. cancel request
// after timeout
func TestSyncTimeout(t *testing.T) {
	n := &nodeProposalBlockerRecorder{}
	srv := &EtcdServer{
		node: n,
	}
	done := make(chan struct{})
	go func() {
		srv.sync(0)
		close(done)
	}()

	// check that sync is non-blocking
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("sync should be non-blocking but did not return after 1s!")
	}

	// give time for goroutine in sync to cancel
	// TODO: use fake clock
	testutil.ForceGosched()
	w := []action{action{name: "Propose blocked"}}
	if g := n.Action(); !reflect.DeepEqual(g, w) {
		t.Errorf("action = %v, want %v", g, w)
	}
}

// TODO: TestNoSyncWhenNoLeader

// blockingNodeProposer implements the node interface to allow users to
// block until Propose has been called and then verify the Proposed data
type blockingNodeProposer struct {
	ch chan []byte
	readyNode
}

func (n *blockingNodeProposer) Propose(_ context.Context, data []byte) error {
	n.ch <- data
	return nil
}

// TestSyncTrigger tests that the server proposes a SYNC request when its sync timer ticks
func TestSyncTrigger(t *testing.T) {
	n := &blockingNodeProposer{
		ch:        make(chan []byte),
		readyNode: *newReadyNode(),
	}
	st := make(chan time.Time, 1)
	srv := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       &storeRecorder{},
		sendhub:     &nopSender{},
		storage:     &storageRecorder{},
		SyncTicker:  st,
	}
	srv.start()
	// trigger the server to become a leader and accept sync requests
	n.readyc <- raft.Ready{
		SoftState: &raft.SoftState{
			RaftState: raft.StateLeader,
		},
	}
	// trigger a sync request
	st <- time.Time{}
	var data []byte
	select {
	case <-time.After(time.Second):
		t.Fatalf("did not receive proposed request as expected!")
	case data = <-n.ch:
	}
	srv.Stop()
	var req pb.Request
	if err := req.Unmarshal(data); err != nil {
		t.Fatalf("error unmarshalling data: %v", err)
	}
	if req.Method != "SYNC" {
		t.Fatalf("unexpected proposed request: %#v", req.Method)
	}
}

// snapshot should snapshot the store and cut the persistent
// TODO: node.Compact is called... we need to make the node an interface
func TestSnapshot(t *testing.T) {
	s := raft.NewMemoryStorage()
	n := raft.StartNode(0xBAD0, mustMakePeerSlice(t, 0xBAD0), 10, 1, s)
	defer n.Stop()

	// Progress the node to the point where it has something to snapshot.
	// TODO(bdarnell): this could be improved with changes in the raft internals.
	// First, we must apply the initial conf changes so we can have an election.
	rd := <-n.Ready()
	s.Append(rd.Entries)
	for _, e := range rd.CommittedEntries {
		if e.Type == raftpb.EntryConfChange {
			var cc raftpb.ConfChange
			err := cc.Unmarshal(e.Data)
			if err != nil {
				t.Fatal(err)
			}
			n.ApplyConfChange(cc)
		}
	}
	n.Advance()

	// Now we can have an election and persist the rest of the log.
	// This causes HardState.Commit to advance. HardState.Commit must
	// be > 0 to snapshot.
	n.Campaign(context.Background())
	rd = <-n.Ready()
	s.Append(rd.Entries)
	n.Advance()

	st := &storeRecorder{}
	p := &storageRecorder{}
	srv := &EtcdServer{
		store:       st,
		storage:     p,
		node:        n,
		raftStorage: s,
	}

	srv.snapshot(1, &raftpb.ConfState{Nodes: []uint64{1}})
	gaction := st.Action()
	if len(gaction) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(gaction))
	}
	if !reflect.DeepEqual(gaction[0], action{name: "Save"}) {
		t.Errorf("action = %s, want Save", gaction[0])
	}

	gaction = p.Action()
	if len(gaction) != 2 {
		t.Fatalf("len(action) = %d, want 2", len(gaction))
	}
	if !reflect.DeepEqual(gaction[0], action{name: "Cut"}) {
		t.Errorf("action = %s, want Cut", gaction[0])
	}
	if !reflect.DeepEqual(gaction[1], action{name: "SaveSnap"}) {
		t.Errorf("action = %s, want SaveSnap", gaction[1])
	}
}

// Applied > SnapCount should trigger a SaveSnap event
func TestTriggerSnap(t *testing.T) {
	ctx := context.Background()
	s := raft.NewMemoryStorage()
	n := raft.StartNode(0xBAD0, mustMakePeerSlice(t, 0xBAD0), 10, 1, s)
	n.Campaign(ctx)
	st := &storeRecorder{}
	p := &storageRecorder{}
	cl := newCluster("abc")
	cl.SetStore(store.New())
	srv := &EtcdServer{
		store:       st,
		sendhub:     &nopSender{},
		storage:     p,
		node:        n,
		raftStorage: s,
		snapCount:   10,
		Cluster:     cl,
	}

	srv.start()
	// wait for saving nop
	time.Sleep(time.Millisecond)
	for i := 0; uint64(i) < srv.snapCount-1; i++ {
		srv.Do(ctx, pb.Request{Method: "PUT", ID: 1})
	}
	// wait for saving the last entry
	time.Sleep(time.Millisecond)
	srv.Stop()

	gaction := p.Action()
	// each operation is recorded as a Save
	// BootstrapConfig/Nop + (SnapCount - 1) * Puts + Cut + SaveSnap = Save + (SnapCount - 1) * Save + Cut + SaveSnap
	wcnt := 2 + int(srv.snapCount)
	if len(gaction) != wcnt {
		t.Fatalf("len(action) = %d, want %d", len(gaction), wcnt)
	}
	if !reflect.DeepEqual(gaction[wcnt-1], action{name: "SaveSnap"}) {
		t.Errorf("action = %s, want SaveSnap", gaction[wcnt-1])
	}
}

// TestRecvSnapshot tests when it receives a snapshot from raft leader,
// it should trigger storage.SaveSnap and also store.Recover.
func TestRecvSnapshot(t *testing.T) {
	n := newReadyNode()
	st := &storeRecorder{}
	p := &storageRecorder{}
	cl := newCluster("abc")
	cl.SetStore(store.New())
	s := &EtcdServer{
		store:       st,
		sendhub:     &nopSender{},
		storage:     p,
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		Cluster:     cl,
	}

	s.start()
	n.readyc <- raft.Ready{Snapshot: raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 1}}}
	// make goroutines move forward to receive snapshot
	testutil.ForceGosched()
	s.Stop()

	wactions := []action{action{name: "Recovery"}}
	if g := st.Action(); !reflect.DeepEqual(g, wactions) {
		t.Errorf("store action = %v, want %v", g, wactions)
	}
	wactions = []action{action{name: "SaveSnap"}, action{name: "Save"}}
	if g := p.Action(); !reflect.DeepEqual(g, wactions) {
		t.Errorf("storage action = %v, want %v", g, wactions)
	}
}

// TestRecvSlowSnapshot tests that slow snapshot will not be applied
// to store. The case could happen when server compacts the log and
// raft returns the compacted snapshot.
func TestRecvSlowSnapshot(t *testing.T) {
	n := newReadyNode()
	st := &storeRecorder{}
	cl := newCluster("abc")
	cl.SetStore(store.New())
	s := &EtcdServer{
		store:       st,
		sendhub:     &nopSender{},
		storage:     &storageRecorder{},
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		Cluster:     cl,
	}

	s.start()
	n.readyc <- raft.Ready{Snapshot: raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 1}}}
	// make goroutines move forward to receive snapshot
	testutil.ForceGosched()
	action := st.Action()

	n.readyc <- raft.Ready{Snapshot: raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 1}}}
	// make goroutines move forward to receive snapshot
	testutil.ForceGosched()
	s.Stop()

	if g := st.Action(); !reflect.DeepEqual(g, action) {
		t.Errorf("store action = %v, want %v", g, action)
	}
}

// TestApplySnapshotAndCommittedEntries tests that server applies snapshot
// first and then committed entries.
func TestApplySnapshotAndCommittedEntries(t *testing.T) {
	n := newReadyNode()
	st := &storeRecorder{}
	cl := newCluster("abc")
	cl.SetStore(store.New())
	storage := raft.NewMemoryStorage()
	s := &EtcdServer{
		store:       st,
		sendhub:     &nopSender{},
		storage:     &storageRecorder{},
		node:        n,
		raftStorage: storage,
		Cluster:     cl,
	}

	s.start()
	req := &pb.Request{Method: "QGET"}
	n.readyc <- raft.Ready{
		Snapshot: raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 1}},
		CommittedEntries: []raftpb.Entry{
			{Index: 2, Data: pbutil.MustMarshal(req)},
		},
	}
	// make goroutines move forward to receive snapshot
	testutil.ForceGosched()
	s.Stop()

	actions := st.Action()
	if len(actions) != 2 {
		t.Fatalf("len(action) = %d, want 2", len(actions))
	}
	if actions[0].name != "Recovery" {
		t.Errorf("actions[0] = %s, want %s", actions[0].name, "Recovery")
	}
	if actions[1].name != "Get" {
		t.Errorf("actions[1] = %s, want %s", actions[1].name, "Get")
	}
}

// TestAddMember tests AddMember can propose and perform node addition.
func TestAddMember(t *testing.T) {
	n := newNodeConfChangeCommitterRecorder()
	n.readyc <- raft.Ready{
		SoftState: &raft.SoftState{RaftState: raft.StateLeader},
	}
	cl := newTestCluster(nil)
	cl.SetStore(store.New())
	s := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       &storeRecorder{},
		sendhub:     &nopSender{},
		storage:     &storageRecorder{},
		Cluster:     cl,
	}
	s.start()
	m := Member{ID: 1234, RaftAttributes: RaftAttributes{PeerURLs: []string{"foo"}}}
	err := s.AddMember(context.TODO(), m)
	gaction := n.Action()
	s.Stop()

	if err != nil {
		t.Fatalf("AddMember error: %v", err)
	}
	wactions := []action{action{name: "ProposeConfChange:ConfChangeAddNode"}, action{name: "ApplyConfChange:ConfChangeAddNode"}}
	if !reflect.DeepEqual(gaction, wactions) {
		t.Errorf("action = %v, want %v", gaction, wactions)
	}
	if cl.Member(1234) == nil {
		t.Errorf("member with id 1234 is not added")
	}
}

// TestRemoveMember tests RemoveMember can propose and perform node removal.
func TestRemoveMember(t *testing.T) {
	n := newNodeConfChangeCommitterRecorder()
	n.readyc <- raft.Ready{
		SoftState: &raft.SoftState{RaftState: raft.StateLeader},
	}
	cl := newTestCluster(nil)
	cl.SetStore(store.New())
	cl.AddMember(&Member{ID: 1234})
	s := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       &storeRecorder{},
		sendhub:     &nopSender{},
		storage:     &storageRecorder{},
		Cluster:     cl,
	}
	s.start()
	err := s.RemoveMember(context.TODO(), 1234)
	gaction := n.Action()
	s.Stop()

	if err != nil {
		t.Fatalf("RemoveMember error: %v", err)
	}
	wactions := []action{action{name: "ProposeConfChange:ConfChangeRemoveNode"}, action{name: "ApplyConfChange:ConfChangeRemoveNode"}}
	if !reflect.DeepEqual(gaction, wactions) {
		t.Errorf("action = %v, want %v", gaction, wactions)
	}
	if cl.Member(1234) != nil {
		t.Errorf("member with id 1234 is not removed")
	}
}

// TestUpdateMember tests RemoveMember can propose and perform node update.
func TestUpdateMember(t *testing.T) {
	n := newNodeConfChangeCommitterRecorder()
	n.readyc <- raft.Ready{
		SoftState: &raft.SoftState{RaftState: raft.StateLeader},
	}
	cl := newTestCluster(nil)
	cl.SetStore(store.New())
	cl.AddMember(&Member{ID: 1234})
	s := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       &storeRecorder{},
		sendhub:     &nopSender{},
		storage:     &storageRecorder{},
		Cluster:     cl,
	}
	s.start()
	wm := Member{ID: 1234, RaftAttributes: RaftAttributes{PeerURLs: []string{"http://127.0.0.1:1"}}}
	err := s.UpdateMember(context.TODO(), wm)
	gaction := n.Action()
	s.Stop()

	if err != nil {
		t.Fatalf("UpdateMember error: %v", err)
	}
	wactions := []action{action{name: "ProposeConfChange:ConfChangeUpdateNode"}, action{name: "ApplyConfChange:ConfChangeUpdateNode"}}
	if !reflect.DeepEqual(gaction, wactions) {
		t.Errorf("action = %v, want %v", gaction, wactions)
	}
	if !reflect.DeepEqual(cl.Member(1234), &wm) {
		t.Errorf("member = %v, want %v", cl.Member(1234), &wm)
	}
}

// TODO: test server could stop itself when being removed

// TODO: test wait trigger correctness in multi-server case

func TestPublish(t *testing.T) {
	n := &nodeProposeDataRecorder{}
	ch := make(chan interface{}, 1)
	// simulate that request has gone through consensus
	ch <- Response{}
	w := &waitWithResponse{ch: ch}
	srv := &EtcdServer{
		id:         1,
		attributes: Attributes{Name: "node1", ClientURLs: []string{"http://a", "http://b"}},
		Cluster:    &Cluster{},
		node:       n,
		w:          w,
	}
	srv.publish(time.Hour)

	data := n.data()
	if len(data) != 1 {
		t.Fatalf("len(proposeData) = %d, want 1", len(data))
	}
	var r pb.Request
	if err := r.Unmarshal(data[0]); err != nil {
		t.Fatalf("unmarshal request error: %v", err)
	}
	if r.Method != "PUT" {
		t.Errorf("method = %s, want PUT", r.Method)
	}
	wm := Member{ID: 1, Attributes: Attributes{Name: "node1", ClientURLs: []string{"http://a", "http://b"}}}
	if w := path.Join(memberStoreKey(wm.ID), attributesSuffix); r.Path != w {
		t.Errorf("path = %s, want %s", r.Path, w)
	}
	var gattr Attributes
	if err := json.Unmarshal([]byte(r.Val), &gattr); err != nil {
		t.Fatalf("unmarshal val error: %v", err)
	}
	if !reflect.DeepEqual(gattr, wm.Attributes) {
		t.Errorf("member = %v, want %v", gattr, wm.Attributes)
	}
}

// TestPublishStopped tests that publish will be stopped if server is stopped.
func TestPublishStopped(t *testing.T) {
	srv := &EtcdServer{
		node:    &nodeRecorder{},
		sendhub: &nopSender{},
		Cluster: &Cluster{},
		w:       &waitRecorder{},
		done:    make(chan struct{}),
		stop:    make(chan struct{}),
	}
	close(srv.done)
	srv.publish(time.Hour)
}

// TestPublishRetry tests that publish will keep retry until success.
func TestPublishRetry(t *testing.T) {
	n := &nodeRecorder{}
	srv := &EtcdServer{
		node: n,
		w:    &waitRecorder{},
		done: make(chan struct{}),
	}
	time.AfterFunc(500*time.Microsecond, func() { close(srv.done) })
	srv.publish(10 * time.Nanosecond)

	action := n.Action()
	// multiple Proposes
	if n := len(action); n < 2 {
		t.Errorf("len(action) = %d, want >= 2", n)
	}
}

func TestStopNotify(t *testing.T) {
	s := &EtcdServer{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go func() {
		<-s.stop
		close(s.done)
	}()

	notifier := s.StopNotify()
	select {
	case <-notifier:
		t.Fatalf("received unexpected stop notification")
	default:
	}
	s.Stop()
	select {
	case <-notifier:
	default:
		t.Fatalf("cannot receive stop notification")
	}
}

func TestGetOtherPeerURLs(t *testing.T) {
	tests := []struct {
		membs []*Member
		self  string
		wurls []string
	}{
		{
			[]*Member{
				newTestMember(1, []string{"http://10.0.0.1"}, "a", nil),
			},
			"a",
			[]string{},
		},
		{
			[]*Member{
				newTestMember(1, []string{"http://10.0.0.1"}, "a", nil),
				newTestMember(2, []string{"http://10.0.0.2"}, "b", nil),
				newTestMember(3, []string{"http://10.0.0.3"}, "c", nil),
			},
			"a",
			[]string{"http://10.0.0.2", "http://10.0.0.3"},
		},
		{
			[]*Member{
				newTestMember(1, []string{"http://10.0.0.1"}, "a", nil),
				newTestMember(3, []string{"http://10.0.0.3"}, "c", nil),
				newTestMember(2, []string{"http://10.0.0.2"}, "b", nil),
			},
			"a",
			[]string{"http://10.0.0.2", "http://10.0.0.3"},
		},
	}
	for i, tt := range tests {
		cl := NewClusterFromMembers("", types.ID(0), tt.membs)
		urls := getOtherPeerURLs(cl, tt.self)
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Errorf("#%d: urls = %+v, want %+v", i, urls, tt.wurls)
		}
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		b    *bool
		wb   bool
		wset bool
	}{
		{nil, false, false},
		{boolp(true), true, true},
		{boolp(false), false, true},
	}
	for i, tt := range tests {
		b, set := getBool(tt.b)
		if b != tt.wb {
			t.Errorf("#%d: value = %v, want %v", i, b, tt.wb)
		}
		if set != tt.wset {
			t.Errorf("#%d: set = %v, want %v", i, set, tt.wset)
		}
	}
}

func TestGenID(t *testing.T) {
	// Sanity check that the GenID function has been seeded appropriately
	// (math/rand is seeded with 1 by default)
	r := rand.NewSource(int64(1))
	var n uint64
	for n == 0 {
		n = uint64(r.Int63())
	}
	if n == GenID() {
		t.Fatalf("GenID's rand seeded with 1!")
	}
}

type action struct {
	name   string
	params []interface{}
}

type recorder struct {
	sync.Mutex
	actions []action
}

func (r *recorder) record(a action) {
	r.Lock()
	r.actions = append(r.actions, a)
	r.Unlock()
}
func (r *recorder) Action() []action {
	r.Lock()
	cpy := make([]action, len(r.actions))
	copy(cpy, r.actions)
	r.Unlock()
	return cpy
}

type storeRecorder struct {
	recorder
}

func (s *storeRecorder) Version() int  { return 0 }
func (s *storeRecorder) Index() uint64 { return 0 }
func (s *storeRecorder) Get(path string, recursive, sorted bool) (*store.Event, error) {
	s.record(action{
		name:   "Get",
		params: []interface{}{path, recursive, sorted},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Set(path string, dir bool, val string, expr time.Time) (*store.Event, error) {
	s.record(action{
		name:   "Set",
		params: []interface{}{path, dir, val, expr},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Update(path, val string, expr time.Time) (*store.Event, error) {
	s.record(action{
		name:   "Update",
		params: []interface{}{path, val, expr},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Create(path string, dir bool, val string, uniq bool, exp time.Time) (*store.Event, error) {
	s.record(action{
		name:   "Create",
		params: []interface{}{path, dir, val, uniq, exp},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) CompareAndSwap(path, prevVal string, prevIdx uint64, val string, expr time.Time) (*store.Event, error) {
	s.record(action{
		name:   "CompareAndSwap",
		params: []interface{}{path, prevVal, prevIdx, val, expr},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Delete(path string, dir, recursive bool) (*store.Event, error) {
	s.record(action{
		name:   "Delete",
		params: []interface{}{path, dir, recursive},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) CompareAndDelete(path, prevVal string, prevIdx uint64) (*store.Event, error) {
	s.record(action{
		name:   "CompareAndDelete",
		params: []interface{}{path, prevVal, prevIdx},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Watch(_ string, _, _ bool, _ uint64) (store.Watcher, error) {
	s.record(action{name: "Watch"})
	return &stubWatcher{}, nil
}
func (s *storeRecorder) Save() ([]byte, error) {
	s.record(action{name: "Save"})
	return nil, nil
}
func (s *storeRecorder) Recovery(b []byte) error {
	s.record(action{name: "Recovery"})
	return nil
}
func (s *storeRecorder) JsonStats() []byte { return nil }
func (s *storeRecorder) DeleteExpiredKeys(cutoff time.Time) {
	s.record(action{
		name:   "DeleteExpiredKeys",
		params: []interface{}{cutoff},
	})
}

type stubWatcher struct{}

func (w *stubWatcher) EventChan() chan *store.Event { return nil }
func (w *stubWatcher) StartIndex() uint64           { return 0 }
func (w *stubWatcher) Remove()                      {}

// errStoreRecorder returns an store error on Get, Watch request
type errStoreRecorder struct {
	storeRecorder
	err error
}

func (s *errStoreRecorder) Get(_ string, _, _ bool) (*store.Event, error) {
	s.record(action{name: "Get"})
	return nil, s.err
}
func (s *errStoreRecorder) Watch(_ string, _, _ bool, _ uint64) (store.Watcher, error) {
	s.record(action{name: "Watch"})
	return nil, s.err
}

type waitRecorder struct {
	action []action
}

func (w *waitRecorder) Register(id uint64) <-chan interface{} {
	w.action = append(w.action, action{name: fmt.Sprint("Register", id)})
	return nil
}
func (w *waitRecorder) Trigger(id uint64, x interface{}) {
	w.action = append(w.action, action{name: fmt.Sprint("Trigger", id)})
}

func boolp(b bool) *bool { return &b }

func stringp(s string) *string { return &s }

type storageRecorder struct {
	recorder
}

func (p *storageRecorder) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	p.record(action{name: "Save"})
	return nil
}
func (p *storageRecorder) Cut() error {
	p.record(action{name: "Cut"})
	return nil
}
func (p *storageRecorder) SaveSnap(st raftpb.Snapshot) error {
	if !raft.IsEmptySnap(st) {
		p.record(action{name: "SaveSnap"})
	}
	return nil
}
func (p *storageRecorder) Close() error { return nil }

type readyNode struct {
	readyc chan raft.Ready
}

func newReadyNode() *readyNode {
	readyc := make(chan raft.Ready, 1)
	return &readyNode{readyc: readyc}
}
func (n *readyNode) Tick()                                          {}
func (n *readyNode) Campaign(ctx context.Context) error             { return nil }
func (n *readyNode) Propose(ctx context.Context, data []byte) error { return nil }
func (n *readyNode) ProposeConfChange(ctx context.Context, conf raftpb.ConfChange) error {
	return nil
}
func (n *readyNode) Step(ctx context.Context, msg raftpb.Message) error       { return nil }
func (n *readyNode) Ready() <-chan raft.Ready                                 { return n.readyc }
func (n *readyNode) Advance()                                                 {}
func (n *readyNode) ApplyConfChange(conf raftpb.ConfChange) *raftpb.ConfState { return nil }
func (n *readyNode) Stop()                                                    {}
func (n *readyNode) Compact(index uint64, nodes []uint64, d []byte)           {}

type nodeRecorder struct {
	recorder
}

func (n *nodeRecorder) Tick() { n.record(action{name: "Tick"}) }

func (n *nodeRecorder) Campaign(ctx context.Context) error {
	n.record(action{name: "Campaign"})
	return nil
}
func (n *nodeRecorder) Propose(ctx context.Context, data []byte) error {
	n.record(action{name: "Propose"})
	return nil
}
func (n *nodeRecorder) ProposeConfChange(ctx context.Context, conf raftpb.ConfChange) error {
	n.record(action{name: "ProposeConfChange"})
	return nil
}
func (n *nodeRecorder) Step(ctx context.Context, msg raftpb.Message) error {
	n.record(action{name: "Step"})
	return nil
}
func (n *nodeRecorder) Ready() <-chan raft.Ready { return nil }
func (n *nodeRecorder) Advance()                 {}
func (n *nodeRecorder) ApplyConfChange(conf raftpb.ConfChange) *raftpb.ConfState {
	n.record(action{name: "ApplyConfChange", params: []interface{}{conf}})
	return &raftpb.ConfState{}
}
func (n *nodeRecorder) Stop() {
	n.record(action{name: "Stop"})
}
func (n *nodeRecorder) Compact(index uint64, nodes []uint64, d []byte) {
	n.record(action{name: "Compact"})
}

type nodeProposeDataRecorder struct {
	nodeRecorder
	sync.Mutex
	d [][]byte
}

func (n *nodeProposeDataRecorder) data() [][]byte {
	n.Lock()
	d := n.d
	n.Unlock()
	return d
}
func (n *nodeProposeDataRecorder) Propose(ctx context.Context, data []byte) error {
	n.nodeRecorder.Propose(ctx, data)
	n.Lock()
	n.d = append(n.d, data)
	n.Unlock()
	return nil
}

type nodeProposalBlockerRecorder struct {
	nodeRecorder
}

func (n *nodeProposalBlockerRecorder) Propose(ctx context.Context, data []byte) error {
	<-ctx.Done()
	n.record(action{name: "Propose blocked"})
	return nil
}

type nodeConfChangeCommitterRecorder struct {
	nodeRecorder
	readyc chan raft.Ready
	index  uint64
}

func newNodeConfChangeCommitterRecorder() *nodeConfChangeCommitterRecorder {
	readyc := make(chan raft.Ready, 1)
	return &nodeConfChangeCommitterRecorder{readyc: readyc}
}
func (n *nodeConfChangeCommitterRecorder) ProposeConfChange(ctx context.Context, conf raftpb.ConfChange) error {
	data, err := conf.Marshal()
	if err != nil {
		return err
	}
	n.index++
	n.readyc <- raft.Ready{CommittedEntries: []raftpb.Entry{{Index: n.index, Type: raftpb.EntryConfChange, Data: data}}}
	n.record(action{name: "ProposeConfChange:" + conf.Type.String()})
	return nil
}
func (n *nodeConfChangeCommitterRecorder) Ready() <-chan raft.Ready {
	return n.readyc
}
func (n *nodeConfChangeCommitterRecorder) ApplyConfChange(conf raftpb.ConfChange) *raftpb.ConfState {
	n.record(action{name: "ApplyConfChange:" + conf.Type.String()})
	return &raftpb.ConfState{}
}

type waitWithResponse struct {
	ch <-chan interface{}
}

func (w *waitWithResponse) Register(id uint64) <-chan interface{} {
	return w.ch
}
func (w *waitWithResponse) Trigger(id uint64, x interface{}) {}

type nopSender struct{}

func (s *nopSender) Sender(id types.ID) rafthttp.Sender { return nil }
func (s *nopSender) Send(m []raftpb.Message)            {}
func (s *nopSender) Add(m *Member)                      {}
func (s *nopSender) Remove(id types.ID)                 {}
func (s *nopSender) Update(m *Member)                   {}
func (s *nopSender) Stop()                              {}
func (s *nopSender) ShouldStopNotify() <-chan struct{}  { return nil }

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
