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
	"path"
	"reflect"
	"testing"

	"github.com/coreos/etcd/store"
)

func TestClusterFromString(t *testing.T) {
	tests := []struct {
		f    string
		mems []Member
	}{
		{
			"mem1=http://10.0.0.1:2379,mem1=http://128.193.4.20:2379,mem2=http://10.0.0.2:2379,default=http://127.0.0.1:2379",
			[]Member{
				newTestMember(4322322643958477905, []string{"http://10.0.0.1:2379", "http://128.193.4.20:2379"}, "mem1", nil),
				newTestMember(3141198903430435750, []string{"http://10.0.0.2:2379"}, "mem2", nil),
				newTestMember(12762790032478827328, []string{"http://127.0.0.1:2379"}, "default", nil),
			},
		},
	}
	for i, tt := range tests {
		c, err := NewClusterFromString("abc", tt.f)
		if err != nil {
			t.Fatalf("#%d: unexpected new error: %v", i, err)
		}
		if c.name != "abc" {
			t.Errorf("#%d: name = %v, want abc", i, c.name)
		}
		wc := newTestCluster(tt.mems)
		if !reflect.DeepEqual(c.members, wc.members) {
			t.Errorf("#%d: members = %+v, want %+v", i, c.members, wc.members)
		}
	}
}

func TestClusterFromStringBad(t *testing.T) {
	tests := []string{
		// invalid URL
		"%^",
		// no URL defined for member
		"mem1=,mem2=http://128.193.4.20:2379,mem3=http://10.0.0.2:2379",
		"mem1,mem2=http://128.193.4.20:2379,mem3=http://10.0.0.2:2379",
		// bad URL for member
		"default=http://localhost/",
		// TODO(philips): anyone know of a 64 bit sha1 hash collision
		// "06b2f82fd81b2c20=http://128.193.4.20:2379,02c60cb75083ceef=http://128.193.4.20:2379",
		// the same url for two members
		"mem1=http://128.193.4.20:2379,mem2=http://128.193.4.20:2379",
	}
	for i, tt := range tests {
		if _, err := NewClusterFromString("abc", tt); err == nil {
			t.Errorf("#%d: unexpected successful new, want err", i)
		}
	}
}

func TestClusterFromStore(t *testing.T) {
	tests := []struct {
		mems []Member
	}{
		{
			[]Member{newTestMember(1, nil, "node1", nil)},
		},
		{
			[]Member{},
		},
		{
			[]Member{
				newTestMember(1, nil, "node1", nil),
				newTestMember(2, nil, "node2", nil),
			},
		},
	}
	for i, tt := range tests {
		st := store.New()
		hc := newTestCluster(nil)
		hc.SetStore(st)
		for _, m := range tt.mems {
			hc.AddMember(&m)
		}
		c := NewClusterFromStore("abc", st)
		if c.name != "abc" {
			t.Errorf("#%d: name = %v, want %v", i, c.name, "abc")
		}
		wc := newTestCluster(tt.mems)
		if !reflect.DeepEqual(c.members, wc.members) {
			t.Errorf("#%d: members = %v, want %v", i, c.members, wc.members)
		}
	}
}

func TestClusterMember(t *testing.T) {
	membs := []Member{
		newTestMember(1, nil, "node1", nil),
		newTestMember(2, nil, "node2", nil),
	}
	tests := []struct {
		id    uint64
		match bool
	}{
		{1, true},
		{2, true},
		{3, false},
	}
	for i, tt := range tests {
		c := newTestCluster(membs)
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
	membs := []Member{
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
		c := newTestCluster(membs)
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
	c := newTestCluster([]Member{
		newTestMember(1, nil, "", nil),
		newTestMember(4, nil, "", nil),
		newTestMember(100, nil, "", nil),
	})
	w := []uint64{1, 4, 100}
	g := c.MemberIDs()
	if !reflect.DeepEqual(w, g) {
		t.Errorf("IDs = %+v, want %+v", g, w)
	}
}

func TestClusterPeerURLs(t *testing.T) {
	tests := []struct {
		mems  []Member
		wurls []string
	}{
		// single peer with a single address
		{
			mems: []Member{
				newTestMember(1, []string{"http://192.0.2.1"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				newTestMember(1, []string{"http://192.0.2.1:8001"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				newTestMember(2, []string{"http://192.0.2.3", "http://192.0.2.4"}, "", nil),
				newTestMember(3, []string{"http://192.0.2.5", "http://192.0.2.6"}, "", nil),
				newTestMember(1, []string{"http://192.0.2.1", "http://192.0.2.2"}, "", nil),
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []Member{},
			wurls: []string{},
		},

		// peer with no peer urls
		{
			mems: []Member{
				newTestMember(3, []string{}, "", nil),
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := newTestCluster(tt.mems)
		urls := c.PeerURLs()
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Errorf("#%d: PeerURLs = %v, want %v", i, urls, tt.wurls)
		}
	}
}

func TestClusterClientURLs(t *testing.T) {
	tests := []struct {
		mems  []Member
		wurls []string
	}{
		// single peer with a single address
		{
			mems: []Member{
				newTestMember(1, nil, "", []string{"http://192.0.2.1"}),
			},
			wurls: []string{"http://192.0.2.1"},
		},

		// single peer with a single address with a port
		{
			mems: []Member{
				newTestMember(1, nil, "", []string{"http://192.0.2.1:8001"}),
			},
			wurls: []string{"http://192.0.2.1:8001"},
		},

		// several members explicitly unsorted
		{
			mems: []Member{
				newTestMember(2, nil, "", []string{"http://192.0.2.3", "http://192.0.2.4"}),
				newTestMember(3, nil, "", []string{"http://192.0.2.5", "http://192.0.2.6"}),
				newTestMember(1, nil, "", []string{"http://192.0.2.1", "http://192.0.2.2"}),
			},
			wurls: []string{"http://192.0.2.1", "http://192.0.2.2", "http://192.0.2.3", "http://192.0.2.4", "http://192.0.2.5", "http://192.0.2.6"},
		},

		// no members
		{
			mems:  []Member{},
			wurls: []string{},
		},

		// peer with no client urls
		{
			mems: []Member{
				newTestMember(3, nil, "", []string{}),
			},
			wurls: []string{},
		},
	}

	for i, tt := range tests {
		c := newTestCluster(tt.mems)
		urls := c.ClientURLs()
		if !reflect.DeepEqual(urls, tt.wurls) {
			t.Errorf("#%d: ClientURLs = %v, want %v", i, urls, tt.wurls)
		}
	}
}

func TestClusterValidateAndAssignIDsBad(t *testing.T) {
	tests := []struct {
		clmembs []Member
		membs   []*Member
	}{
		{
			// unmatched length
			[]Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
			},
			[]*Member{},
		},
		{
			// unmatched peer urls
			[]Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
			},
			[]*Member{
				newTestMemberp(1, []string{"http://127.0.0.1:4001"}, "", nil),
			},
		},
		{
			// unmatched peer urls
			[]Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMember(2, []string{"http://127.0.0.2:2379"}, "", nil),
			},
			[]*Member{
				newTestMemberp(1, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMemberp(2, []string{"http://127.0.0.2:4001"}, "", nil),
			},
		},
	}
	for i, tt := range tests {
		cl := newTestCluster(tt.clmembs)
		if err := cl.ValidateAndAssignIDs(tt.membs); err == nil {
			t.Errorf("#%d: unexpected update success", i)
		}
	}
}

func TestClusterValidateAndAssignIDs(t *testing.T) {
	tests := []struct {
		clmembs []Member
		membs   []*Member
		wids    []uint64
	}{
		{
			[]Member{
				newTestMember(1, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMember(2, []string{"http://127.0.0.2:2379"}, "", nil),
			},
			[]*Member{
				newTestMemberp(3, []string{"http://127.0.0.1:2379"}, "", nil),
				newTestMemberp(4, []string{"http://127.0.0.2:2379"}, "", nil),
			},
			[]uint64{3, 4},
		},
	}
	for i, tt := range tests {
		cl := newTestCluster(tt.clmembs)
		if err := cl.ValidateAndAssignIDs(tt.membs); err != nil {
			t.Errorf("#%d: unexpect update error: %v", i, err)
		}
		if !reflect.DeepEqual(cl.MemberIDs(), tt.wids) {
			t.Errorf("#%d: ids = %v, want %v", i, cl.MemberIDs(), tt.wids)
		}
	}
}

func TestClusterGenID(t *testing.T) {
	cs := newTestCluster([]Member{
		newTestMember(1, nil, "", nil),
		newTestMember(2, nil, "", nil),
	})

	cs.genID()
	if cs.ID() == 0 {
		t.Fatalf("cluster.ID = %v, want not 0", cs.ID())
	}
	previd := cs.ID()

	cs.SetStore(&storeRecorder{})
	cs.AddMember(newTestMemberp(3, nil, "", nil))
	cs.genID()
	if cs.ID() == previd {
		t.Fatalf("cluster.ID = %v, want not %v", cs.ID(), previd)
	}
}

func TestNodeToMemberBad(t *testing.T) {
	tests := []*store.NodeExtern{
		{Key: "/1234", Nodes: []*store.NodeExtern{
			{Key: "/1234/strange"},
		}},
		{Key: "/1234", Nodes: []*store.NodeExtern{
			{Key: "/1234/dynamic", Value: stringp("garbage")},
		}},
		{Key: "/1234", Nodes: []*store.NodeExtern{
			{Key: "/1234/dynamic", Value: stringp(`{"PeerURLs":null}`)},
		}},
		{Key: "/1234", Nodes: []*store.NodeExtern{
			{Key: "/1234/dynamic", Value: stringp(`{"PeerURLs":null}`)},
			{Key: "/1234/strange"},
		}},
		{Key: "/1234", Nodes: []*store.NodeExtern{
			{Key: "/1234/dynamic", Value: stringp(`{"PeerURLs":null}`)},
			{Key: "/1234/static", Value: stringp("garbage")},
		}},
		{Key: "/1234", Nodes: []*store.NodeExtern{
			{Key: "/1234/dynamic", Value: stringp(`{"PeerURLs":null}`)},
			{Key: "/1234/static", Value: stringp(`{"Name":"node1","ClientURLs":null}`)},
			{Key: "/1234/strange"},
		}},
	}
	for i, tt := range tests {
		if _, err := nodeToMember(tt); err == nil {
			t.Errorf("#%d: unexpected nil error", i)
		}
	}
}

func TestClusterAddMember(t *testing.T) {
	st := &storeRecorder{}
	c := newTestCluster(nil)
	c.SetStore(st)
	c.AddMember(newTestMemberp(1, nil, "node1", nil))

	wactions := []action{
		{
			name: "Create",
			params: []interface{}{
				path.Join(storeMembersPrefix, "1", "raftAttributes"),
				false,
				`{"PeerURLs":null}`,
				false,
				store.Permanent,
			},
		},
		{
			name: "Create",
			params: []interface{}{
				path.Join(storeMembersPrefix, "1", "attributes"),
				false,
				`{"Name":"node1"}`,
				false,
				store.Permanent,
			},
		},
	}
	if g := st.Action(); !reflect.DeepEqual(g, wactions) {
		t.Errorf("actions = %v, want %v", g, wactions)
	}
}

func TestClusterMembers(t *testing.T) {
	cls := &Cluster{
		members: map[uint64]*Member{
			1:   &Member{ID: 1},
			20:  &Member{ID: 20},
			100: &Member{ID: 100},
			5:   &Member{ID: 5},
			50:  &Member{ID: 50},
		},
	}
	w := []*Member{
		&Member{ID: 1},
		&Member{ID: 5},
		&Member{ID: 20},
		&Member{ID: 50},
		&Member{ID: 100},
	}
	if g := cls.Members(); !reflect.DeepEqual(g, w) {
		t.Fatalf("Members()=%#v, want %#v", g, w)
	}
}

func TestClusterRemoveMember(t *testing.T) {
	st := &storeRecorder{}
	c := newTestCluster(nil)
	c.SetStore(st)
	c.RemoveMember(1)

	wactions := []action{
		{name: "Delete", params: []interface{}{memberStoreKey(1), true, true}},
		{name: "Create", params: []interface{}{removedMemberStoreKey(1), false, "", false, store.Permanent}},
	}
	if !reflect.DeepEqual(st.Action(), wactions) {
		t.Errorf("actions = %v, want %v", st.Action(), wactions)
	}
}

func TestNodeToMember(t *testing.T) {
	n := &store.NodeExtern{Key: "/1234", Nodes: []*store.NodeExtern{
		{Key: "/1234/attributes", Value: stringp(`{"Name":"node1","ClientURLs":null}`)},
		{Key: "/1234/raftAttributes", Value: stringp(`{"PeerURLs":null}`)},
	}}
	wm := &Member{ID: 0x1234, RaftAttributes: RaftAttributes{}, Attributes: Attributes{Name: "node1"}}
	m, err := nodeToMember(n)
	if err != nil {
		t.Fatalf("unexpected nodeToMember error: %v", err)
	}
	if !reflect.DeepEqual(m, wm) {
		t.Errorf("member = %+v, want %+v", m, wm)
	}
}

func newTestCluster(membs []Member) *Cluster {
	c := &Cluster{members: make(map[uint64]*Member), removed: make(map[uint64]bool)}
	for i, m := range membs {
		c.members[m.ID] = &membs[i]
	}
	return c
}

func newTestMember(id uint64, peerURLs []string, name string, clientURLs []string) Member {
	return Member{
		ID:             id,
		RaftAttributes: RaftAttributes{PeerURLs: peerURLs},
		Attributes:     Attributes{Name: name, ClientURLs: clientURLs},
	}
}

func newTestMemberp(id uint64, peerURLs []string, name string, clientURLs []string) *Member {
	m := newTestMember(id, peerURLs, name, clientURLs)
	return &m
}
