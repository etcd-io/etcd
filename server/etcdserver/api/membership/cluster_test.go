// Copyright 2015 The etcd Authors
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

package membership

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	betesting "go.etcd.io/etcd/server/v3/mvcc/backend/testing"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/mock/mockstore"
)

func TestClusterMember(t *testing.T) {
	membs := []*Member{
		newTestMember(1, nil, "node1", nil),
		newTestMember(2, nil, "node2", nil),
	}
	tests := []struct {
		id    types.ID
		match bool
	}{
		{1, true},
		{2, true},
		{3, false},
	}
	for i, tt := range tests {
		c := newTestCluster(t, membs)
		m := c.Member(tt.id)
		if g := m != nil; g != tt.match {
			t.Errorf("#%d: find member = %v, want %v", i, g, tt.match)
		}
		if m != nil && m.ID != tt.id {
			t.Errorf("#%d: id = %x, want %x", i, m.ID, tt.id)
		}
	}
}

func TestClusterMemberByName(t *testing.T) {
	membs := []*Member{
		newTestMember(1, nil, "node1", nil),
		newTestMember(2, nil, "node2", nil),
	}
	tests := []struct {
		name  string
		match bool
	}{
		{"node1", true},
		{"node2", true},
		{"node3", false},
	}
	for i, tt := range tests {
		c := newTestCluster(t, membs)
		m := c.MemberByName(tt.name)
		if g := m != nil; g != tt.match {
			t.Errorf("#%d: find member = %v, want %v", i, g, tt.match)
		}
		if m != nil && m.Name != tt.name {
			t.Errorf("#%d: name = %v, want %v", i, m.Name, tt.name)
		}
	}
}

func TestClusterMemberIDs(t *testing.T) {
	c := newTestCluster(t, []*Member{
		newTestMember(1, nil, "", nil),
		newTestMember(4, nil, "", nil),
		newTestMember(100, nil, "", nil),
	})
	w := []types.ID{1, 4, 100}
	g := c.MemberIDs()
	if !reflect.DeepEqual(w, g) {
		t.Errorf("IDs = %+v, want %+v", g, w)
	}
}

func TestClusterPeerURLs(t *testing.T) {
	tests := []struct {
		mems  []*Member
		wurls []string
	}{
		// single peer with a single address
		{
			mems: []*Member{
				newTestMember(1, []string{"http://192.0.2.1"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []*Member{
				newTestMember(1, []string{"http://192.0.2.1:8001"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []*Member{
				newTestMember(2, []string{"http://192.0.2.3", "http://192.0.2.4"}, "", nil),
				newTestMember(3, []string{"http://192.0.2.5", "http://192.0.2.6"}, "", nil),
				newTestMember(1, []string{"http://192.0.2.1", "http://192.0.2.2"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []*Member{},
			wurls: []string{},
		},

		// peer with no peer urls
		{
			mems: []*Member{
				newTestMember(3, []string{}, "", nil),
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := newTestCluster(t, tt.mems)
		urls := c.PeerURLs()
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Errorf("#%d: PeerURLs = %v, want %v", i, urls, tt.wurls)
		}
	}
}

func TestClusterClientURLs(t *testing.T) {
	tests := []struct {
		mems  []*Member
		wurls []string
	}{
		// single peer with a single address
		{
			mems: []*Member{
				newTestMember(1, nil, "", []string{"http://192.0.2.1"}),
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []*Member{
				newTestMember(1, nil, "", []string{"http://192.0.2.1:8001"}),
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []*Member{
				newTestMember(2, nil, "", []string{"http://192.0.2.3", "http://192.0.2.4"}),
				newTestMember(3, nil, "", []string{"http://192.0.2.5", "http://192.0.2.6"}),
				newTestMember(1, nil, "", []string{"http://192.0.2.1", "http://192.0.2.2"}),
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []*Member{},
			wurls: []string{},
		},

		// peer with no client urls
		{
			mems: []*Member{
				newTestMember(3, nil, "", []string{}),
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := newTestCluster(t, tt.mems)
		urls := c.ClientURLs()
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Errorf("#%d: ClientURLs = %v, want %v", i, urls, tt.wurls)
		}
	}
}

func TestClusterValidateAndAssignIDsBad(t *testing.T) {
	tests := []struct {
		clmembs []*Member
		membs   []*Member
	}{
		{
			// unmatched length
			[]*Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
			},
			[]*Member{},
		},
		{
			// unmatched peer urls
			[]*Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
			},
			[]*Member{
				newTestMember(1, []string{"http://127.0.0.1:4001"}, "", nil),
			},
		},
		{
			// unmatched peer urls
			[]*Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMember(2, []string{"http://127.0.0.2:2379"}, "", nil),
			},
			[]*Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMember(2, []string{"http://127.0.0.2:4001"}, "", nil),
			},
		},
	}
	for i, tt := range tests {
		ecl := newTestCluster(t, tt.clmembs)
		lcl := newTestCluster(t, tt.membs)
		if err := ValidateClusterAndAssignIDs(zap.NewExample(), lcl, ecl); err == nil {
			t.Errorf("#%d: unexpected update success", i)
		}
	}
}

func TestClusterValidateAndAssignIDs(t *testing.T) {
	tests := []struct {
		clmembs []*Member
		membs   []*Member
		wids    []types.ID
	}{
		{
			[]*Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMember(2, []string{"http://127.0.0.2:2379"}, "", nil),
			},
			[]*Member{
				newTestMember(3, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMember(4, []string{"http://127.0.0.2:2379"}, "", nil),
			},
			[]types.ID{3, 4},
		},
	}
	for i, tt := range tests {
		lcl := newTestCluster(t, tt.clmembs)
		ecl := newTestCluster(t, tt.membs)
		if err := ValidateClusterAndAssignIDs(zap.NewExample(), lcl, ecl); err != nil {
			t.Errorf("#%d: unexpect update error: %v", i, err)
		}
		if !reflect.DeepEqual(lcl.MemberIDs(), tt.wids) {
			t.Errorf("#%d: ids = %v, want %v", i, lcl.MemberIDs(), tt.wids)
		}
	}
}

func TestClusterValidateConfigurationChange(t *testing.T) {
	cl := NewCluster(zaptest.NewLogger(t))
	cl.SetStore(v2store.New())
	for i := 1; i <= 4; i++ {
		attr := RaftAttributes{PeerURLs: []string{fmt.Sprintf("http://127.0.0.1:%d", i)}}
		cl.AddMember(&Member{ID: types.ID(i), RaftAttributes: attr}, true)
	}
	cl.RemoveMember(4, true)

	attr := RaftAttributes{PeerURLs: []string{fmt.Sprintf("http://127.0.0.1:%d", 1)}}
	ctx, err := json.Marshal(&Member{ID: types.ID(5), RaftAttributes: attr})
	if err != nil {
		t.Fatal(err)
	}

	attr = RaftAttributes{PeerURLs: []string{fmt.Sprintf("http://127.0.0.1:%d", 1)}}
	ctx1, err := json.Marshal(&Member{ID: types.ID(1), RaftAttributes: attr})
	if err != nil {
		t.Fatal(err)
	}

	attr = RaftAttributes{PeerURLs: []string{fmt.Sprintf("http://127.0.0.1:%d", 5)}}
	ctx5, err := json.Marshal(&Member{ID: types.ID(5), RaftAttributes: attr})
	if err != nil {
		t.Fatal(err)
	}

	attr = RaftAttributes{PeerURLs: []string{fmt.Sprintf("http://127.0.0.1:%d", 3)}}
	ctx2to3, err := json.Marshal(&Member{ID: types.ID(2), RaftAttributes: attr})
	if err != nil {
		t.Fatal(err)
	}

	attr = RaftAttributes{PeerURLs: []string{fmt.Sprintf("http://127.0.0.1:%d", 5)}}
	ctx2to5, err := json.Marshal(&Member{ID: types.ID(2), RaftAttributes: attr})
	if err != nil {
		t.Fatal(err)
	}

	ctx3, err := json.Marshal(&ConfigChangeContext{Member: Member{ID: types.ID(3), RaftAttributes: attr}, IsPromote: true})
	if err != nil {
		t.Fatal(err)
	}

	ctx6, err := json.Marshal(&ConfigChangeContext{Member: Member{ID: types.ID(6), RaftAttributes: attr}, IsPromote: true})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		cc   raftpb.ConfChange
		werr error
	}{
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeRemoveNode,
				NodeID: 3,
			},
			nil,
		},
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeAddNode,
				NodeID: 4,
			},
			ErrIDRemoved,
		},
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeRemoveNode,
				NodeID: 4,
			},
			ErrIDRemoved,
		},
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeAddNode,
				NodeID:  1,
				Context: ctx1,
			},
			ErrIDExists,
		},
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeAddNode,
				NodeID:  5,
				Context: ctx,
			},
			ErrPeerURLexists,
		},
		{
			raftpb.ConfChange{
				Type:   raftpb.ConfChangeRemoveNode,
				NodeID: 5,
			},
			ErrIDNotFound,
		},
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeAddNode,
				NodeID:  5,
				Context: ctx5,
			},
			nil,
		},
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeUpdateNode,
				NodeID:  5,
				Context: ctx,
			},
			ErrIDNotFound,
		},
		// try to change the peer url of 2 to the peer url of 3
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeUpdateNode,
				NodeID:  2,
				Context: ctx2to3,
			},
			ErrPeerURLexists,
		},
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeUpdateNode,
				NodeID:  2,
				Context: ctx2to5,
			},
			nil,
		},
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeAddNode,
				NodeID:  3,
				Context: ctx3,
			},
			ErrMemberNotLearner,
		},
		{
			raftpb.ConfChange{
				Type:    raftpb.ConfChangeAddNode,
				NodeID:  6,
				Context: ctx6,
			},
			ErrIDNotFound,
		},
	}
	for i, tt := range tests {
		err := cl.ValidateConfigurationChange(tt.cc)
		if err != tt.werr {
			t.Errorf("#%d: validateConfigurationChange error = %v, want %v", i, err, tt.werr)
		}
	}
}

func TestClusterGenID(t *testing.T) {
	cs := newTestCluster(t, []*Member{
		newTestMember(1, nil, "", nil),
		newTestMember(2, nil, "", nil),
	})

	cs.genID()
	if cs.ID() == 0 {
		t.Fatalf("cluster.ID = %v, want not 0", cs.ID())
	}
	previd := cs.ID()

	cs.SetStore(mockstore.NewNop())
	cs.AddMember(newTestMember(3, nil, "", nil), true)
	cs.genID()
	if cs.ID() == previd {
		t.Fatalf("cluster.ID = %v, want not %v", cs.ID(), previd)
	}
}

func TestNodeToMemberBad(t *testing.T) {
	tests := []*v2store.NodeExtern{
		{Key: "/1234", Nodes: []*v2store.NodeExtern{
			{Key: "/1234/strange"},
		}},
		{Key: "/1234", Nodes: []*v2store.NodeExtern{
			{Key: "/1234/raftAttributes", Value: stringp("garbage")},
		}},
		{Key: "/1234", Nodes: []*v2store.NodeExtern{
			{Key: "/1234/attributes", Value: stringp(`{"name":"node1","clientURLs":null}`)},
		}},
		{Key: "/1234", Nodes: []*v2store.NodeExtern{
			{Key: "/1234/raftAttributes", Value: stringp(`{"peerURLs":null}`)},
			{Key: "/1234/strange"},
		}},
		{Key: "/1234", Nodes: []*v2store.NodeExtern{
			{Key: "/1234/raftAttributes", Value: stringp(`{"peerURLs":null}`)},
			{Key: "/1234/attributes", Value: stringp("garbage")},
		}},
		{Key: "/1234", Nodes: []*v2store.NodeExtern{
			{Key: "/1234/raftAttributes", Value: stringp(`{"peerURLs":null}`)},
			{Key: "/1234/attributes", Value: stringp(`{"name":"node1","clientURLs":null}`)},
			{Key: "/1234/strange"},
		}},
	}
	for i, tt := range tests {
		if _, err := nodeToMember(zap.NewExample(), tt); err == nil {
			t.Errorf("#%d: unexpected nil error", i)
		}
	}
}

func TestClusterAddMember(t *testing.T) {
	st := mockstore.NewRecorder()
	c := newTestCluster(t, nil)
	c.SetStore(st)
	c.AddMember(newTestMember(1, nil, "node1", nil), true)

	wactions := []testutil.Action{
		{
			Name: "Create",
			Params: []interface{}{
				path.Join(StoreMembersPrefix, "1", "raftAttributes"),
				false,
				`{"peerURLs":null}`,
				false,
				v2store.TTLOptionSet{ExpireTime: v2store.Permanent},
			},
		},
	}
	if g := st.Action(); !reflect.DeepEqual(g, wactions) {
		t.Errorf("actions = %v, want %v", g, wactions)
	}
}

func TestClusterAddMemberAsLearner(t *testing.T) {
	st := mockstore.NewRecorder()
	c := newTestCluster(t, nil)
	c.SetStore(st)
	c.AddMember(newTestMemberAsLearner(1, nil, "node1", nil), true)

	wactions := []testutil.Action{
		{
			Name: "Create",
			Params: []interface{}{
				path.Join(StoreMembersPrefix, "1", "raftAttributes"),
				false,
				`{"peerURLs":null,"isLearner":true}`,
				false,
				v2store.TTLOptionSet{ExpireTime: v2store.Permanent},
			},
		},
	}
	if g := st.Action(); !reflect.DeepEqual(g, wactions) {
		t.Errorf("actions = %v, want %v", g, wactions)
	}
}

func TestClusterMembers(t *testing.T) {
	cls := newTestCluster(t, []*Member{
		{ID: 1},
		{ID: 20},
		{ID: 100},
		{ID: 5},
		{ID: 50},
	})
	w := []*Member{
		{ID: 1},
		{ID: 5},
		{ID: 20},
		{ID: 50},
		{ID: 100},
	}
	if g := cls.Members(); !reflect.DeepEqual(g, w) {
		t.Fatalf("Members()=%#v, want %#v", g, w)
	}
}

func TestClusterRemoveMember(t *testing.T) {
	st := mockstore.NewRecorder()
	c := newTestCluster(t, nil)
	c.SetStore(st)
	c.RemoveMember(1, true)

	wactions := []testutil.Action{
		{Name: "Delete", Params: []interface{}{MemberStoreKey(1), true, true}},
		{Name: "Create", Params: []interface{}{RemovedMemberStoreKey(1), false, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent}}},
	}
	if !reflect.DeepEqual(st.Action(), wactions) {
		t.Errorf("actions = %v, want %v", st.Action(), wactions)
	}
}

func TestClusterUpdateAttributes(t *testing.T) {
	name := "etcd"
	clientURLs := []string{"http://127.0.0.1:4001"}
	tests := []struct {
		mems    []*Member
		removed map[types.ID]bool
		wmems   []*Member
	}{
		// update attributes of existing member
		{
			[]*Member{
				newTestMember(1, nil, "", nil),
			},
			nil,
			[]*Member{
				newTestMember(1, nil, name, clientURLs),
			},
		},
		// update attributes of removed member
		{
			nil,
			map[types.ID]bool{types.ID(1): true},
			nil,
		},
	}
	for i, tt := range tests {
		c := newTestCluster(t, tt.mems)
		c.removed = tt.removed

		c.UpdateAttributes(types.ID(1), Attributes{Name: name, ClientURLs: clientURLs}, true)
		if g := c.Members(); !reflect.DeepEqual(g, tt.wmems) {
			t.Errorf("#%d: members = %+v, want %+v", i, g, tt.wmems)
		}
	}
}

func TestNodeToMember(t *testing.T) {
	n := &v2store.NodeExtern{Key: "/1234", Nodes: []*v2store.NodeExtern{
		{Key: "/1234/attributes", Value: stringp(`{"name":"node1","clientURLs":null}`)},
		{Key: "/1234/raftAttributes", Value: stringp(`{"peerURLs":null}`)},
	}}
	wm := &Member{ID: 0x1234, RaftAttributes: RaftAttributes{}, Attributes: Attributes{Name: "node1"}}
	m, err := nodeToMember(zap.NewExample(), n)
	if err != nil {
		t.Fatalf("unexpected nodeToMember error: %v", err)
	}
	if !reflect.DeepEqual(m, wm) {
		t.Errorf("member = %+v, want %+v", m, wm)
	}
}

func newTestCluster(t testing.TB, membs []*Member) *RaftCluster {
	c := &RaftCluster{lg: zaptest.NewLogger(t), members: make(map[types.ID]*Member), removed: make(map[types.ID]bool)}
	for _, m := range membs {
		c.members[m.ID] = m
	}
	return c
}

func stringp(s string) *string { return &s }

func TestIsReadyToAddVotingMember(t *testing.T) {
	tests := []struct {
		members []*Member
		want    bool
	}{
		{
			// 0/3 members ready, should fail
			[]*Member{
				newTestMember(1, nil, "", nil),
				newTestMember(2, nil, "", nil),
				newTestMember(3, nil, "", nil),
			},
			false,
		},
		{
			// 1/2 members ready, should fail
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
			},
			false,
		},
		{
			// 1/3 members ready, should fail
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
				newTestMember(3, nil, "", nil),
			},
			false,
		},
		{
			// 1/1 members ready, should succeed (special case of 1-member cluster for recovery)
			[]*Member{
				newTestMember(1, nil, "1", nil),
			},
			true,
		},
		{
			// 2/3 members ready, should fail
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "", nil),
			},
			false,
		},
		{
			// 3/3 members ready, should be fine to add one member and retain quorum
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "3", nil),
			},
			true,
		},
		{
			// 3/4 members ready, should be fine to add one member and retain quorum
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "3", nil),
				newTestMember(4, nil, "", nil),
			},
			true,
		},
		{
			// empty cluster, it is impossible but should fail
			[]*Member{},
			false,
		},
		{
			// 2 voting members ready in cluster with 2 voting members and 2 unstarted learner member, should succeed
			// (the status of learner members does not affect the readiness of adding voting member)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMemberAsLearner(3, nil, "", nil),
				newTestMemberAsLearner(4, nil, "", nil),
			},
			true,
		},
		{
			// 1 voting member ready in cluster with 2 voting members and 2 ready learner member, should fail
			// (the status of learner members does not affect the readiness of adding voting member)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
				newTestMemberAsLearner(3, nil, "3", nil),
				newTestMemberAsLearner(4, nil, "4", nil),
			},
			false,
		},
	}
	for i, tt := range tests {
		c := newTestCluster(t, tt.members)
		if got := c.IsReadyToAddVotingMember(); got != tt.want {
			t.Errorf("%d: isReadyToAddNewMember returned %t, want %t", i, got, tt.want)
		}
	}
}

func TestIsReadyToRemoveVotingMember(t *testing.T) {
	tests := []struct {
		members  []*Member
		removeID uint64
		want     bool
	}{
		{
			// 1/1 members ready, should fail
			[]*Member{
				newTestMember(1, nil, "1", nil),
			},
			1,
			false,
		},
		{
			// 0/3 members ready, should fail
			[]*Member{
				newTestMember(1, nil, "", nil),
				newTestMember(2, nil, "", nil),
				newTestMember(3, nil, "", nil),
			},
			1,
			false,
		},
		{
			// 1/2 members ready, should be fine to remove unstarted member
			// (isReadyToRemoveMember() logic should return success, but operation itself would fail)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
			},
			2,
			true,
		},
		{
			// 2/3 members ready, should fail
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "", nil),
			},
			2,
			false,
		},
		{
			// 3/3 members ready, should be fine to remove one member and retain quorum
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "3", nil),
			},
			3,
			true,
		},
		{
			// 3/4 members ready, should be fine to remove one member
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "3", nil),
				newTestMember(4, nil, "", nil),
			},
			3,
			true,
		},
		{
			// 3/4 members ready, should be fine to remove unstarted member
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "3", nil),
				newTestMember(4, nil, "", nil),
			},
			4,
			true,
		},
		{
			// 1 voting members ready in cluster with 1 voting member and 1 ready learner,
			// removing voting member should fail
			// (the status of learner members does not affect the readiness of removing voting member)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMemberAsLearner(2, nil, "2", nil),
			},
			1,
			false,
		},
		{
			// 1 voting members ready in cluster with 2 voting member and 1 ready learner,
			// removing ready voting member should fail
			// (the status of learner members does not affect the readiness of removing voting member)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
				newTestMemberAsLearner(3, nil, "3", nil),
			},
			1,
			false,
		},
		{
			// 1 voting members ready in cluster with 2 voting member and 1 ready learner,
			// removing unstarted voting member should be fine. (Actual operation will fail)
			// (the status of learner members does not affect the readiness of removing voting member)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
				newTestMemberAsLearner(3, nil, "3", nil),
			},
			2,
			true,
		},
		{
			// 1 voting members ready in cluster with 2 voting member and 1 unstarted learner,
			// removing not-ready voting member should be fine. (Actual operation will fail)
			// (the status of learner members does not affect the readiness of removing voting member)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
				newTestMemberAsLearner(3, nil, "", nil),
			},
			2,
			true,
		},
	}
	for i, tt := range tests {
		c := newTestCluster(t, tt.members)
		if got := c.IsReadyToRemoveVotingMember(tt.removeID); got != tt.want {
			t.Errorf("%d: isReadyToAddNewMember returned %t, want %t", i, got, tt.want)
		}
	}
}

func TestIsReadyToPromoteMember(t *testing.T) {
	tests := []struct {
		members   []*Member
		promoteID uint64
		want      bool
	}{
		{
			// 1/1 members ready, should succeed (quorum = 1, new quorum = 2)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMemberAsLearner(2, nil, "2", nil),
			},
			2,
			true,
		},
		{
			// 0/1 members ready, should fail (quorum = 1)
			[]*Member{
				newTestMember(1, nil, "", nil),
				newTestMemberAsLearner(2, nil, "2", nil),
			},
			2,
			false,
		},
		{
			// 2/2 members ready, should succeed (quorum = 2)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMemberAsLearner(3, nil, "3", nil),
			},
			3,
			true,
		},
		{
			// 1/2 members ready, should succeed (quorum = 2)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
				newTestMemberAsLearner(3, nil, "3", nil),
			},
			3,
			true,
		},
		{
			// 1/3 members ready, should fail (quorum = 2)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "", nil),
				newTestMember(3, nil, "", nil),
				newTestMemberAsLearner(4, nil, "4", nil),
			},
			4,
			false,
		},
		{
			// 2/3 members ready, should succeed (quorum = 2, new quorum = 3)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "", nil),
				newTestMemberAsLearner(4, nil, "4", nil),
			},
			4,
			true,
		},
		{
			// 2/4 members ready, should succeed (quorum = 3)
			[]*Member{
				newTestMember(1, nil, "1", nil),
				newTestMember(2, nil, "2", nil),
				newTestMember(3, nil, "", nil),
				newTestMember(4, nil, "", nil),
				newTestMemberAsLearner(5, nil, "5", nil),
			},
			5,
			true,
		},
	}
	for i, tt := range tests {
		c := newTestCluster(t, tt.members)
		if got := c.IsReadyToPromoteMember(tt.promoteID); got != tt.want {
			t.Errorf("%d: isReadyToPromoteMember returned %t, want %t", i, got, tt.want)
		}
	}
}

func TestIsVersionChangable(t *testing.T) {
	v0 := semver.Must(semver.NewVersion("2.4.0"))
	v1 := semver.Must(semver.NewVersion("3.4.0"))
	v2 := semver.Must(semver.NewVersion("3.5.0"))
	v3 := semver.Must(semver.NewVersion("3.5.1"))
	v4 := semver.Must(semver.NewVersion("3.6.0"))

	tests := []struct {
		name           string
		currentVersion *semver.Version
		localVersion   *semver.Version
		expectedResult bool
	}{
		{
			name:           "When local version is one minor lower than cluster version",
			currentVersion: v2,
			localVersion:   v1,
			expectedResult: true,
		},
		{
			name:           "When local version is one minor and one patch lower than cluster version",
			currentVersion: v3,
			localVersion:   v1,
			expectedResult: true,
		},
		{
			name:           "When local version is one minor higher than cluster version",
			currentVersion: v1,
			localVersion:   v2,
			expectedResult: true,
		},
		{
			name:           "When local version is two minor higher than cluster version",
			currentVersion: v1,
			localVersion:   v4,
			expectedResult: true,
		},
		{
			name:           "When local version is one major higher than cluster version",
			currentVersion: v0,
			localVersion:   v1,
			expectedResult: false,
		},
		{
			name:           "When local version is equal to cluster version",
			currentVersion: v1,
			localVersion:   v1,
			expectedResult: false,
		},
		{
			name:           "When local version is one patch higher than cluster version",
			currentVersion: v2,
			localVersion:   v3,
			expectedResult: false,
		},
		{
			name:           "When local version is two minor lower than cluster version",
			currentVersion: v4,
			localVersion:   v1,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ret := IsValidVersionChange(tt.currentVersion, tt.localVersion); ret != tt.expectedResult {
				t.Errorf("Expected %v; Got %v", tt.expectedResult, ret)
			}
		})
	}
}

func TestAddMemberSyncsBackendAndStoreV2(t *testing.T) {
	now := time.Now()
	alice := NewMember("", nil, "alice", &now)

	tcs := []struct {
		name string

		storeV2Nil     bool
		backendNil     bool
		storeV2Members []*Member
		backendMembers []*Member

		expectPanics  bool
		expectMembers map[types.ID]*Member
	}{
		{
			name: "Adding new member should succeed",
		},
		{
			name:           "Adding member should succeed if it was only in storeV2",
			storeV2Members: []*Member{alice},
		},
		{
			name:           "Adding member should succeed if it was only in backend",
			backendMembers: []*Member{alice},
		},
		{
			name:           "Adding member should fail if it exists in both",
			storeV2Members: []*Member{alice},
			backendMembers: []*Member{alice},
			expectPanics:   true,
		},
		{
			name:           "Adding member should fail if it exists in storeV2 and backend is nil",
			storeV2Members: []*Member{alice},
			backendNil:     true,
			expectPanics:   true,
		},
		{
			name:           "Adding member should succeed if it exists in backend and storageV2 is nil",
			storeV2Nil:     true,
			backendMembers: []*Member{alice},
		},
		{
			name:           "Adding new member should succeed if backend is nil",
			storeV2Members: []*Member{},
			backendNil:     true,
		},
		{
			name:           "Adding new member should fail if storageV2 is nil",
			storeV2Nil:     true,
			backendMembers: []*Member{},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, _ := betesting.NewDefaultTmpBackend(t)
			defer be.Close()
			mustCreateBackendBuckets(be)
			st := v2store.New()
			for _, m := range tc.backendMembers {
				unsafeSaveMemberToBackend(lg, be, m)
			}
			be.ForceCommit()
			for _, m := range tc.storeV2Members {
				mustSaveMemberToStore(lg, st, m)
			}
			cluster := NewCluster(lg)
			if !tc.backendNil {
				cluster.SetBackend(be)
			}
			if !tc.storeV2Nil {
				cluster.SetStore(st)
			}
			if tc.expectPanics {
				assert.Panics(t, func() {
					cluster.AddMember(alice, ApplyBoth)
				})
			} else {
				cluster.AddMember(alice, ApplyBoth)
			}
			if !tc.storeV2Nil {
				storeV2Members, _ := membersFromStore(lg, st)
				assert.Equal(t, map[types.ID]*Member{alice.ID: alice}, storeV2Members)
			}
			if !tc.backendNil {
				be.ForceCommit()
				beMembers, _ := mustReadMembersFromBackend(lg, be)
				assert.Equal(t, map[types.ID]*Member{alice.ID: alice}, beMembers)
			}
		})
	}
}

func TestRemoveMemberSyncsBackendAndStoreV2(t *testing.T) {
	now := time.Now()
	alice := NewMember("", nil, "alice", &now)

	tcs := []struct {
		name string

		storeV2Nil     bool
		backendNil     bool
		storeV2Members []*Member
		backendMembers []*Member

		expectMembers []*Member
		expectPanics  bool
	}{
		{
			name:         "Removing new member should fail",
			expectPanics: true,
		},
		{
			name:           "Removing member should succeed if it was only in storeV2",
			storeV2Members: []*Member{alice},
		},
		{
			name:           "Removing member should succeed if it was only in backend",
			backendMembers: []*Member{alice},
		},
		{
			name:           "Removing member should succeed if it exists in both",
			storeV2Members: []*Member{alice},
			backendMembers: []*Member{alice},
		},
		{
			name:           "Removing new member should fail if backend is nil",
			storeV2Members: []*Member{},
			backendNil:     true,
			expectPanics:   true,
		},
		{
			name:           "Removing new member should succeed if storageV2 is nil",
			storeV2Nil:     true,
			backendMembers: []*Member{},
		},
		{
			name:           "Removing member should succeed if it exists in v2storage and backend is nil",
			storeV2Members: []*Member{alice},
			backendNil:     true,
		},
		{
			name:           "Removing member should succeed if it exists in backend and storageV2 is nil",
			storeV2Nil:     true,
			backendMembers: []*Member{alice},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			be, _ := betesting.NewDefaultTmpBackend(t)
			defer be.Close()
			mustCreateBackendBuckets(be)
			st := v2store.New()
			for _, m := range tc.backendMembers {
				unsafeSaveMemberToBackend(lg, be, m)
			}
			be.ForceCommit()
			for _, m := range tc.storeV2Members {
				mustSaveMemberToStore(lg, st, m)
			}
			cluster := NewCluster(lg)
			if !tc.backendNil {
				cluster.SetBackend(be)
			}
			if !tc.storeV2Nil {
				cluster.SetStore(st)
			}
			if tc.expectPanics {
				assert.Panics(t, func() {
					cluster.RemoveMember(alice.ID, ApplyBoth)
				})
			} else {
				cluster.RemoveMember(alice.ID, ApplyBoth)
			}
			if !tc.storeV2Nil {
				storeV2Members, _ := membersFromStore(lg, st)
				assert.Equal(t, map[types.ID]*Member{}, storeV2Members)
			}
			if !tc.backendNil {
				be.ForceCommit()
				beMembers, _ := mustReadMembersFromBackend(lg, be)
				assert.Equal(t, map[types.ID]*Member{}, beMembers)
			}
		})
	}
}
