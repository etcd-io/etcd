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
	"net/http"
	"os"
	"path"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/idutil"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

// TestDoLocalAction tests requests which do not need to go through raft to be applied,
// and are served through local data.
func TestDoLocalAction(t *testing.T) {
	tests := []struct {
		req pb.Request

		wresp    Response
		werr     error
		wactions []testutil.Action
	}{
		{
			pb.Request{Method: "GET", ID: 1, Wait: true},
			Response{Watcher: &nopWatcher{}}, nil, []testutil.Action{{Name: "Watch"}},
		},
		{
			pb.Request{Method: "GET", ID: 1},
			Response{Event: &store.Event{}}, nil,
			[]testutil.Action{
				{
					Name:   "Get",
					Params: []interface{}{"", false, false},
				},
			},
		},
		{
			pb.Request{Method: "HEAD", ID: 1},
			Response{Event: &store.Event{}}, nil,
			[]testutil.Action{
				{
					Name:   "Get",
					Params: []interface{}{"", false, false},
				},
			},
		},
		{
			pb.Request{Method: "BADMETHOD", ID: 1},
			Response{}, ErrUnknownMethod, []testutil.Action{},
		},
	}
	for i, tt := range tests {
		st := &storeRecorder{}
		srv := &EtcdServer{
			store:    st,
			reqIDGen: idutil.NewGenerator(0, time.Time{}),
		}
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

		wactions []testutil.Action
	}{
		{
			pb.Request{Method: "GET", ID: 1, Wait: true},
			[]testutil.Action{{Name: "Watch"}},
		},
		{
			pb.Request{Method: "GET", ID: 1},
			[]testutil.Action{
				{
					Name:   "Get",
					Params: []interface{}{"", false, false},
				},
			},
		},
		{
			pb.Request{Method: "HEAD", ID: 1},
			[]testutil.Action{
				{
					Name:   "Get",
					Params: []interface{}{"", false, false},
				},
			},
		},
	}
	for i, tt := range tests {
		st := &errStoreRecorder{err: storeErr}
		srv := &EtcdServer{
			store:    st,
			reqIDGen: idutil.NewGenerator(0, time.Time{}),
		}
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
		wactions []testutil.Action
	}{
		// POST ==> Create
		{
			pb.Request{Method: "POST", ID: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Create",
					Params: []interface{}{"", false, "", true, time.Time{}},
				},
			},
		},
		// POST ==> Create, with expiration
		{
			pb.Request{Method: "POST", ID: 1, Expiration: 1337},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Create",
					Params: []interface{}{"", false, "", true, time.Unix(0, 1337)},
				},
			},
		},
		// POST ==> Create, with dir
		{
			pb.Request{Method: "POST", ID: 1, Dir: true},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Create",
					Params: []interface{}{"", true, "", true, time.Time{}},
				},
			},
		},
		// PUT ==> Set
		{
			pb.Request{Method: "PUT", ID: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Set",
					Params: []interface{}{"", false, "", time.Time{}},
				},
			},
		},
		// PUT ==> Set, with dir
		{
			pb.Request{Method: "PUT", ID: 1, Dir: true},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Set",
					Params: []interface{}{"", true, "", time.Time{}},
				},
			},
		},
		// PUT with PrevExist=true ==> Update
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: pbutil.Boolp(true)},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Update",
					Params: []interface{}{"", "", time.Time{}},
				},
			},
		},
		// PUT with PrevExist=false ==> Create
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: pbutil.Boolp(false)},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Create",
					Params: []interface{}{"", false, "", false, time.Time{}},
				},
			},
		},
		// PUT with PrevExist=true *and* PrevIndex set ==> Update
		// TODO(jonboulle): is this expected?!
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: pbutil.Boolp(true), PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Update",
					Params: []interface{}{"", "", time.Time{}},
				},
			},
		},
		// PUT with PrevExist=false *and* PrevIndex set ==> Create
		// TODO(jonboulle): is this expected?!
		{
			pb.Request{Method: "PUT", ID: 1, PrevExist: pbutil.Boolp(false), PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Create",
					Params: []interface{}{"", false, "", false, time.Time{}},
				},
			},
		},
		// PUT with PrevIndex set ==> CompareAndSwap
		{
			pb.Request{Method: "PUT", ID: 1, PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "CompareAndSwap",
					Params: []interface{}{"", "", uint64(1), "", time.Time{}},
				},
			},
		},
		// PUT with PrevValue set ==> CompareAndSwap
		{
			pb.Request{Method: "PUT", ID: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "CompareAndSwap",
					Params: []interface{}{"", "bar", uint64(0), "", time.Time{}},
				},
			},
		},
		// PUT with PrevIndex and PrevValue set ==> CompareAndSwap
		{
			pb.Request{Method: "PUT", ID: 1, PrevIndex: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "CompareAndSwap",
					Params: []interface{}{"", "bar", uint64(1), "", time.Time{}},
				},
			},
		},
		// DELETE ==> Delete
		{
			pb.Request{Method: "DELETE", ID: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Delete",
					Params: []interface{}{"", false, false},
				},
			},
		},
		// DELETE with PrevIndex set ==> CompareAndDelete
		{
			pb.Request{Method: "DELETE", ID: 1, PrevIndex: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "CompareAndDelete",
					Params: []interface{}{"", "", uint64(1)},
				},
			},
		},
		// DELETE with PrevValue set ==> CompareAndDelete
		{
			pb.Request{Method: "DELETE", ID: 1, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "CompareAndDelete",
					Params: []interface{}{"", "bar", uint64(0)},
				},
			},
		},
		// DELETE with PrevIndex *and* PrevValue set ==> CompareAndDelete
		{
			pb.Request{Method: "DELETE", ID: 1, PrevIndex: 5, PrevValue: "bar"},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "CompareAndDelete",
					Params: []interface{}{"", "bar", uint64(5)},
				},
			},
		},
		// QGET ==> Get
		{
			pb.Request{Method: "QGET", ID: 1},
			Response{Event: &store.Event{}},
			[]testutil.Action{
				{
					Name:   "Get",
					Params: []interface{}{"", false, false},
				},
			},
		},
		// SYNC ==> DeleteExpiredKeys
		{
			pb.Request{Method: "SYNC", ID: 1},
			Response{},
			[]testutil.Action{
				{
					Name:   "DeleteExpiredKeys",
					Params: []interface{}{time.Unix(0, 0)},
				},
			},
		},
		{
			pb.Request{Method: "SYNC", ID: 1, Time: 12345},
			Response{},
			[]testutil.Action{
				{
					Name:   "DeleteExpiredKeys",
					Params: []interface{}{time.Unix(0, 12345)},
				},
			},
		},
		// Unknown method - error
		{
			pb.Request{Method: "BADMETHOD", ID: 1},
			Response{err: ErrUnknownMethod},
			[]testutil.Action{},
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
		w := []testutil.Action{
			{
				Name:   "ApplyConfChange",
				Params: []interface{}{cc},
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
		id:        1,
		node:      &nodeRecorder{},
		Cluster:   cl,
		transport: &nopTransporter{},
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

func TestDoProposal(t *testing.T) {
	tests := []pb.Request{
		pb.Request{Method: "POST", ID: 1},
		pb.Request{Method: "PUT", ID: 1},
		pb.Request{Method: "DELETE", ID: 1},
		pb.Request{Method: "GET", ID: 1, Quorum: true},
	}
	for i, tt := range tests {
		st := &storeRecorder{}
		srv := &EtcdServer{
			node:        newNodeCommitter(),
			raftStorage: raft.NewMemoryStorage(),
			store:       st,
			transport:   &nopTransporter{},
			storage:     &storageRecorder{},
			reqIDGen:    idutil.NewGenerator(0, time.Time{}),
		}
		srv.start()
		resp, err := srv.Do(context.Background(), tt)
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
	wait := &waitRecorder{}
	srv := &EtcdServer{
		node:     &nodeRecorder{},
		w:        wait,
		reqIDGen: idutil.NewGenerator(0, time.Time{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := srv.Do(ctx, pb.Request{Method: "PUT"})

	if err != ErrCanceled {
		t.Fatalf("err = %v, want %v", err, ErrCanceled)
	}
	w := []testutil.Action{{Name: "Register"}, {Name: "Trigger"}}
	if !reflect.DeepEqual(wait.action, w) {
		t.Errorf("wait.action = %+v, want %+v", wait.action, w)
	}
}

func TestDoProposalTimeout(t *testing.T) {
	srv := &EtcdServer{
		node:     &nodeRecorder{},
		w:        &waitRecorder{},
		reqIDGen: idutil.NewGenerator(0, time.Time{}),
	}
	ctx, _ := context.WithTimeout(context.Background(), 0)
	_, err := srv.Do(ctx, pb.Request{Method: "PUT"})
	if err != ErrTimeout {
		t.Fatalf("err = %v, want %v", err, ErrTimeout)
	}
}

func TestDoProposalStopped(t *testing.T) {
	srv := &EtcdServer{
		node:     &nodeRecorder{},
		w:        &waitRecorder{},
		reqIDGen: idutil.NewGenerator(0, time.Time{}),
	}
	srv.done = make(chan struct{})
	close(srv.done)
	_, err := srv.Do(context.Background(), pb.Request{Method: "PUT", ID: 1})
	if err != ErrStopped {
		t.Errorf("err = %v, want %v", err, ErrStopped)
	}
}

// TestSync tests sync 1. is nonblocking 2. proposes SYNC request.
func TestSync(t *testing.T) {
	n := &nodeRecorder{}
	srv := &EtcdServer{
		node:     n,
		reqIDGen: idutil.NewGenerator(0, time.Time{}),
	}
	// check that sync is non-blocking
	timer := time.AfterFunc(time.Second, func() {
		t.Fatalf("sync should be non-blocking but did not return after 1s!")
	})
	srv.sync(10 * time.Second)
	timer.Stop()
	testutil.ForceGosched()

	action := n.Action()
	if len(action) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(action))
	}
	if action[0].Name != "Propose" {
		t.Fatalf("action = %s, want Propose", action[0].Name)
	}
	data := action[0].Params[0].([]byte)
	var r pb.Request
	if err := r.Unmarshal(data); err != nil {
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
		node:     n,
		reqIDGen: idutil.NewGenerator(0, time.Time{}),
	}
	// check that sync is non-blocking
	timer := time.AfterFunc(time.Second, func() {
		t.Fatalf("sync should be non-blocking but did not return after 1s!")
	})
	srv.sync(0)
	timer.Stop()

	// give time for goroutine in sync to cancel
	testutil.ForceGosched()
	w := []testutil.Action{{Name: "Propose blocked"}}
	if g := n.Action(); !reflect.DeepEqual(g, w) {
		t.Errorf("action = %v, want %v", g, w)
	}
}

// TODO: TestNoSyncWhenNoLeader

// TestSyncTrigger tests that the server proposes a SYNC request when its sync timer ticks
func TestSyncTrigger(t *testing.T) {
	n := newReadyNode()
	st := make(chan time.Time, 1)
	srv := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       &storeRecorder{},
		transport:   &nopTransporter{},
		storage:     &storageRecorder{},
		SyncTicker:  st,
		reqIDGen:    idutil.NewGenerator(0, time.Time{}),
	}
	srv.start()
	defer srv.Stop()
	// trigger the server to become a leader and accept sync requests
	n.readyc <- raft.Ready{
		SoftState: &raft.SoftState{
			RaftState: raft.StateLeader,
		},
	}
	// trigger a sync request
	st <- time.Time{}
	testutil.ForceGosched()

	action := n.Action()
	if len(action) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(action))
	}
	if action[0].Name != "Propose" {
		t.Fatalf("action = %s, want Propose", action[0].Name)
	}
	data := action[0].Params[0].([]byte)
	var req pb.Request
	if err := req.Unmarshal(data); err != nil {
		t.Fatalf("error unmarshalling data: %v", err)
	}
	if req.Method != "SYNC" {
		t.Fatalf("unexpected proposed request: %#v", req.Method)
	}
}

// snapshot should snapshot the store and cut the persistent
func TestSnapshot(t *testing.T) {
	s := raft.NewMemoryStorage()
	s.Append([]raftpb.Entry{{Index: 1}})
	st := &storeRecorder{}
	p := &storageRecorder{}
	srv := &EtcdServer{
		node:        &nodeRecorder{},
		raftStorage: s,
		store:       st,
		storage:     p,
	}
	srv.snapshot(1, &raftpb.ConfState{Nodes: []uint64{1}})
	gaction := st.Action()
	if len(gaction) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(gaction))
	}
	if !reflect.DeepEqual(gaction[0], testutil.Action{Name: "Save"}) {
		t.Errorf("action = %s, want Save", gaction[0])
	}
	gaction = p.Action()
	if len(gaction) != 2 {
		t.Fatalf("len(action) = %d, want 2", len(gaction))
	}
	if !reflect.DeepEqual(gaction[0], testutil.Action{Name: "Cut"}) {
		t.Errorf("action = %s, want Cut", gaction[0])
	}
	if !reflect.DeepEqual(gaction[1], testutil.Action{Name: "SaveSnap"}) {
		t.Errorf("action = %s, want SaveSnap", gaction[1])
	}
}

// Applied > SnapCount should trigger a SaveSnap event
func TestTriggerSnap(t *testing.T) {
	snapc := 10
	st := &storeRecorder{}
	p := &storageRecorder{}
	srv := &EtcdServer{
		node:        newNodeCommitter(),
		raftStorage: raft.NewMemoryStorage(),
		store:       st,
		transport:   &nopTransporter{},
		storage:     p,
		snapCount:   uint64(snapc),
		reqIDGen:    idutil.NewGenerator(0, time.Time{}),
	}
	srv.start()
	for i := 0; i < snapc+1; i++ {
		srv.Do(context.Background(), pb.Request{Method: "PUT"})
	}
	srv.Stop()

	gaction := p.Action()
	// each operation is recorded as a Save
	// (SnapCount+1) * Puts + Cut + SaveSnap = (SnapCount+1) * Save + Cut + SaveSnap
	wcnt := 3 + snapc
	if len(gaction) != wcnt {
		t.Fatalf("len(action) = %d, want %d", len(gaction), wcnt)
	}
	if !reflect.DeepEqual(gaction[wcnt-1], testutil.Action{Name: "SaveSnap"}) {
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
		transport:   &nopTransporter{},
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

	wactions := []testutil.Action{{Name: "Recovery"}}
	if g := st.Action(); !reflect.DeepEqual(g, wactions) {
		t.Errorf("store action = %v, want %v", g, wactions)
	}
	wactions = []testutil.Action{{Name: "SaveSnap"}, {Name: "Save"}}
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
		transport:   &nopTransporter{},
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
		transport:   &nopTransporter{},
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
	if actions[0].Name != "Recovery" {
		t.Errorf("actions[0] = %s, want %s", actions[0].Name, "Recovery")
	}
	if actions[1].Name != "Get" {
		t.Errorf("actions[1] = %s, want %s", actions[1].Name, "Get")
	}
}

// TestAddMember tests AddMember can propose and perform node addition.
func TestAddMember(t *testing.T) {
	n := newNodeConfChangeCommitterRecorder()
	n.readyc <- raft.Ready{
		SoftState: &raft.SoftState{RaftState: raft.StateLeader},
	}
	cl := newTestCluster(nil)
	st := store.New()
	cl.SetStore(st)
	s := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       st,
		transport:   &nopTransporter{},
		storage:     &storageRecorder{},
		Cluster:     cl,
		reqIDGen:    idutil.NewGenerator(0, time.Time{}),
	}
	s.start()
	m := Member{ID: 1234, RaftAttributes: RaftAttributes{PeerURLs: []string{"foo"}}}
	err := s.AddMember(context.TODO(), m)
	gaction := n.Action()
	s.Stop()

	if err != nil {
		t.Fatalf("AddMember error: %v", err)
	}
	wactions := []testutil.Action{{Name: "ProposeConfChange:ConfChangeAddNode"}, {Name: "ApplyConfChange:ConfChangeAddNode"}}
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
	st := store.New()
	cl.SetStore(store.New())
	cl.AddMember(&Member{ID: 1234})
	s := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       st,
		transport:   &nopTransporter{},
		storage:     &storageRecorder{},
		Cluster:     cl,
		reqIDGen:    idutil.NewGenerator(0, time.Time{}),
	}
	s.start()
	err := s.RemoveMember(context.TODO(), 1234)
	gaction := n.Action()
	s.Stop()

	if err != nil {
		t.Fatalf("RemoveMember error: %v", err)
	}
	wactions := []testutil.Action{{Name: "ProposeConfChange:ConfChangeRemoveNode"}, {Name: "ApplyConfChange:ConfChangeRemoveNode"}}
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
	st := store.New()
	cl.SetStore(st)
	cl.AddMember(&Member{ID: 1234})
	s := &EtcdServer{
		node:        n,
		raftStorage: raft.NewMemoryStorage(),
		store:       st,
		transport:   &nopTransporter{},
		storage:     &storageRecorder{},
		Cluster:     cl,
		reqIDGen:    idutil.NewGenerator(0, time.Time{}),
	}
	s.start()
	wm := Member{ID: 1234, RaftAttributes: RaftAttributes{PeerURLs: []string{"http://127.0.0.1:1"}}}
	err := s.UpdateMember(context.TODO(), wm)
	gaction := n.Action()
	s.Stop()

	if err != nil {
		t.Fatalf("UpdateMember error: %v", err)
	}
	wactions := []testutil.Action{{Name: "ProposeConfChange:ConfChangeUpdateNode"}, {Name: "ApplyConfChange:ConfChangeUpdateNode"}}
	if !reflect.DeepEqual(gaction, wactions) {
		t.Errorf("action = %v, want %v", gaction, wactions)
	}
	if !reflect.DeepEqual(cl.Member(1234), &wm) {
		t.Errorf("member = %v, want %v", cl.Member(1234), &wm)
	}
}

// TODO: test server could stop itself when being removed

func TestPublish(t *testing.T) {
	n := &nodeRecorder{}
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
		reqIDGen:   idutil.NewGenerator(0, time.Time{}),
	}
	srv.publish(time.Hour)

	action := n.Action()
	if len(action) != 1 {
		t.Fatalf("len(action) = %d, want 1", len(action))
	}
	if action[0].Name != "Propose" {
		t.Fatalf("action = %s, want Propose", action[0].Name)
	}
	data := action[0].Params[0].([]byte)
	var r pb.Request
	if err := r.Unmarshal(data); err != nil {
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
		node:      &nodeRecorder{},
		transport: &nopTransporter{},
		Cluster:   &Cluster{},
		w:         &waitRecorder{},
		done:      make(chan struct{}),
		stop:      make(chan struct{}),
		reqIDGen:  idutil.NewGenerator(0, time.Time{}),
	}
	close(srv.done)
	srv.publish(time.Hour)
}

// TestPublishRetry tests that publish will keep retry until success.
func TestPublishRetry(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stderr)
	n := &nodeRecorder{}
	srv := &EtcdServer{
		node:     n,
		w:        &waitRecorder{},
		done:     make(chan struct{}),
		reqIDGen: idutil.NewGenerator(0, time.Time{}),
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

// storeRecorder records all the methods it receives.
// storeRecorder DOES NOT work as a actual store.
// It always returns invaild empty response and no error.
type storeRecorder struct{ testutil.Recorder }

func (s *storeRecorder) Version() int  { return 0 }
func (s *storeRecorder) Index() uint64 { return 0 }
func (s *storeRecorder) Get(path string, recursive, sorted bool) (*store.Event, error) {
	s.Record(testutil.Action{
		Name:   "Get",
		Params: []interface{}{path, recursive, sorted},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Set(path string, dir bool, val string, expr time.Time) (*store.Event, error) {
	s.Record(testutil.Action{
		Name:   "Set",
		Params: []interface{}{path, dir, val, expr},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Update(path, val string, expr time.Time) (*store.Event, error) {
	s.Record(testutil.Action{
		Name:   "Update",
		Params: []interface{}{path, val, expr},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Create(path string, dir bool, val string, uniq bool, exp time.Time) (*store.Event, error) {
	s.Record(testutil.Action{
		Name:   "Create",
		Params: []interface{}{path, dir, val, uniq, exp},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) CompareAndSwap(path, prevVal string, prevIdx uint64, val string, expr time.Time) (*store.Event, error) {
	s.Record(testutil.Action{
		Name:   "CompareAndSwap",
		Params: []interface{}{path, prevVal, prevIdx, val, expr},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Delete(path string, dir, recursive bool) (*store.Event, error) {
	s.Record(testutil.Action{
		Name:   "Delete",
		Params: []interface{}{path, dir, recursive},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) CompareAndDelete(path, prevVal string, prevIdx uint64) (*store.Event, error) {
	s.Record(testutil.Action{
		Name:   "CompareAndDelete",
		Params: []interface{}{path, prevVal, prevIdx},
	})
	return &store.Event{}, nil
}
func (s *storeRecorder) Watch(_ string, _, _ bool, _ uint64) (store.Watcher, error) {
	s.Record(testutil.Action{Name: "Watch"})
	return &nopWatcher{}, nil
}
func (s *storeRecorder) Save() ([]byte, error) {
	s.Record(testutil.Action{Name: "Save"})
	return nil, nil
}
func (s *storeRecorder) Recovery(b []byte) error {
	s.Record(testutil.Action{Name: "Recovery"})
	return nil
}
func (s *storeRecorder) JsonStats() []byte { return nil }
func (s *storeRecorder) DeleteExpiredKeys(cutoff time.Time) {
	s.Record(testutil.Action{
		Name:   "DeleteExpiredKeys",
		Params: []interface{}{cutoff},
	})
}

type nopWatcher struct{}

func (w *nopWatcher) EventChan() chan *store.Event { return nil }
func (w *nopWatcher) StartIndex() uint64           { return 0 }
func (w *nopWatcher) Remove()                      {}

// errStoreRecorder is a storeRecorder, but returns the given error on
// Get, Watch methods.
type errStoreRecorder struct {
	storeRecorder
	err error
}

func (s *errStoreRecorder) Get(path string, recursive, sorted bool) (*store.Event, error) {
	s.storeRecorder.Get(path, recursive, sorted)
	return nil, s.err
}
func (s *errStoreRecorder) Watch(path string, recursive, sorted bool, index uint64) (store.Watcher, error) {
	s.storeRecorder.Watch(path, recursive, sorted, index)
	return nil, s.err
}

type waitRecorder struct {
	action []testutil.Action
}

func (w *waitRecorder) Register(id uint64) <-chan interface{} {
	w.action = append(w.action, testutil.Action{Name: "Register"})
	return nil
}
func (w *waitRecorder) Trigger(id uint64, x interface{}) {
	w.action = append(w.action, testutil.Action{Name: "Trigger"})
}

type waitWithResponse struct {
	ch <-chan interface{}
}

func (w *waitWithResponse) Register(id uint64) <-chan interface{} {
	return w.ch
}
func (w *waitWithResponse) Trigger(id uint64, x interface{}) {}

type storageRecorder struct{ testutil.Recorder }

func (p *storageRecorder) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	p.Record(testutil.Action{Name: "Save"})
	return nil
}
func (p *storageRecorder) Cut() error {
	p.Record(testutil.Action{Name: "Cut"})
	return nil
}
func (p *storageRecorder) SaveSnap(st raftpb.Snapshot) error {
	if !raft.IsEmptySnap(st) {
		p.Record(testutil.Action{Name: "SaveSnap"})
	}
	return nil
}
func (p *storageRecorder) Close() error { return nil }

type nodeRecorder struct{ testutil.Recorder }

func (n *nodeRecorder) Tick() { n.Record(testutil.Action{Name: "Tick"}) }
func (n *nodeRecorder) Campaign(ctx context.Context) error {
	n.Record(testutil.Action{Name: "Campaign"})
	return nil
}
func (n *nodeRecorder) Propose(ctx context.Context, data []byte) error {
	n.Record(testutil.Action{Name: "Propose", Params: []interface{}{data}})
	return nil
}
func (n *nodeRecorder) ProposeConfChange(ctx context.Context, conf raftpb.ConfChange) error {
	n.Record(testutil.Action{Name: "ProposeConfChange"})
	return nil
}
func (n *nodeRecorder) Step(ctx context.Context, msg raftpb.Message) error {
	n.Record(testutil.Action{Name: "Step"})
	return nil
}
func (n *nodeRecorder) Ready() <-chan raft.Ready { return nil }
func (n *nodeRecorder) Advance()                 {}
func (n *nodeRecorder) ApplyConfChange(conf raftpb.ConfChange) *raftpb.ConfState {
	n.Record(testutil.Action{Name: "ApplyConfChange", Params: []interface{}{conf}})
	return &raftpb.ConfState{}
}
func (n *nodeRecorder) Stop() {
	n.Record(testutil.Action{Name: "Stop"})
}
func (n *nodeRecorder) Compact(index uint64, nodes []uint64, d []byte) {
	n.Record(testutil.Action{Name: "Compact"})
}

type nodeProposalBlockerRecorder struct {
	nodeRecorder
}

func (n *nodeProposalBlockerRecorder) Propose(ctx context.Context, data []byte) error {
	<-ctx.Done()
	n.Record(testutil.Action{Name: "Propose blocked"})
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
	n.Record(testutil.Action{Name: "ProposeConfChange:" + conf.Type.String()})
	return nil
}
func (n *nodeConfChangeCommitterRecorder) Ready() <-chan raft.Ready {
	return n.readyc
}
func (n *nodeConfChangeCommitterRecorder) ApplyConfChange(conf raftpb.ConfChange) *raftpb.ConfState {
	n.Record(testutil.Action{Name: "ApplyConfChange:" + conf.Type.String()})
	return &raftpb.ConfState{}
}

// nodeCommitter commits proposed data immediately.
type nodeCommitter struct {
	nodeRecorder
	readyc chan raft.Ready
	index  uint64
}

func newNodeCommitter() *nodeCommitter {
	readyc := make(chan raft.Ready, 1)
	return &nodeCommitter{readyc: readyc}
}
func (n *nodeCommitter) Propose(ctx context.Context, data []byte) error {
	n.index++
	ents := []raftpb.Entry{{Index: n.index, Data: data}}
	n.readyc <- raft.Ready{
		Entries:          ents,
		CommittedEntries: ents,
	}
	return nil
}
func (n *nodeCommitter) Ready() <-chan raft.Ready {
	return n.readyc
}

type readyNode struct {
	nodeRecorder
	readyc chan raft.Ready
}

func newReadyNode() *readyNode {
	readyc := make(chan raft.Ready, 1)
	return &readyNode{readyc: readyc}
}
func (n *readyNode) Ready() <-chan raft.Ready { return n.readyc }

type nopTransporter struct{}

func (s *nopTransporter) Handler() http.Handler               { return nil }
func (s *nopTransporter) Send(m []raftpb.Message)             {}
func (s *nopTransporter) AddPeer(id types.ID, us []string)    {}
func (s *nopTransporter) RemovePeer(id types.ID)              {}
func (s *nopTransporter) UpdatePeer(id types.ID, us []string) {}
func (s *nopTransporter) Stop()                               {}
func (s *nopTransporter) Pause()                              {}
func (s *nopTransporter) Resume()                             {}
