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
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
)

func TestClusterStoreAdd(t *testing.T) {
	st := &storeRecorder{}
	ps := &clusterStore{Store: st}
	ps.Add(newTestMember(1, nil, "node1", nil))

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
				`{"Name":"node1","ClientURLs":null}`,
				false,
				store.Permanent,
			},
		},
	}
	if g := st.Action(); !reflect.DeepEqual(g, wactions) {
		t.Errorf("actions = %v, want %v", g, wactions)
	}
}

func TestClusterStoreGet(t *testing.T) {
	tests := []struct {
		mems  []Member
		wmems []Member
	}{
		{
			[]Member{newTestMember(1, nil, "node1", nil)},
			[]Member{newTestMember(1, nil, "node1", nil)},
		},
		{
			[]Member{},
			[]Member{},
		},
		{
			[]Member{
				newTestMember(1, nil, "node1", nil),
				newTestMember(2, nil, "node2", nil),
			},
			[]Member{
				newTestMember(1, nil, "node1", nil),
				newTestMember(2, nil, "node2", nil),
			},
		},
		{
			[]Member{
				newTestMember(2, nil, "node2", nil),
				newTestMember(1, nil, "node1", nil),
			},
			[]Member{
				newTestMember(1, nil, "node1", nil),
				newTestMember(2, nil, "node2", nil),
			},
		},
	}
	for i, tt := range tests {
		c := NewCluster("")
		if err := c.AddSlice(tt.mems); err != nil {
			t.Fatal(err)
		}
		c.GenID(nil)
		cs := &clusterStore{Store: newGetAllStore(), id: c.id}
		for _, m := range tt.mems {
			cs.Add(m)
		}
		if g := cs.Get(); !reflect.DeepEqual(&g, c) {
			t.Errorf("#%d: mems = %v, want %v", i, &g, c)
		}
	}
}

func TestClusterStoreRemove(t *testing.T) {
	st := &storeRecorder{}
	cs := &clusterStore{Store: st}
	cs.Remove(1)

	wactions := []action{
		{name: "Delete", params: []interface{}{memberStoreKey(1), true, true}},
		{name: "Create", params: []interface{}{removedMemberStoreKey(1), false, "", false, store.Permanent}},
	}
	if !reflect.DeepEqual(st.Action(), wactions) {
		t.Errorf("actions = %v, want %v", st.Action(), wactions)
	}
}

func TestClusterStoreIsRemovedFalse(t *testing.T) {
	st := &errStoreRecorder{err: etcdErr.NewError(etcdErr.EcodeKeyNotFound, "", 0)}
	cs := clusterStore{Store: st}
	if ok := cs.IsRemoved(1); ok != false {
		t.Errorf("IsRemoved = %v, want %v", ok, false)
	}
}

func TestClusterStoreIsRemovedTrue(t *testing.T) {
	st := &storeRecorder{}
	cs := &clusterStore{Store: st}
	if ok := cs.IsRemoved(1); ok != true {
		t.Errorf("IsRemoved = %v, want %v", ok, true)
	}
	wactions := []action{
		{name: "Get", params: []interface{}{removedMemberStoreKey(1), false, false}},
	}
	if !reflect.DeepEqual(st.Action(), wactions) {
		t.Errorf("actions = %v, want %v", st.Action(), wactions)
	}
}

func TestNodeToMemberFail(t *testing.T) {
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

func TestNodeToMember(t *testing.T) {
	n := &store.NodeExtern{Key: "/1234", Nodes: []*store.NodeExtern{
		{Key: "/1234/attributes", Value: stringp(`{"Name":"node1","ClientURLs":null}`)},
		{Key: "/1234/raftAttributes", Value: stringp(`{"PeerURLs":null}`)},
	}}
	wm := Member{ID: 0x1234, RaftAttributes: RaftAttributes{}, Attributes: Attributes{Name: "node1"}}
	m, err := nodeToMember(n)
	if err != nil {
		t.Fatalf("unexpected nodeToMember error: %v", err)
	}
	if !reflect.DeepEqual(m, wm) {
		t.Errorf("member = %+v, want %+v", m, wm)
	}
}

// simpleStore implements basic create and get.
type simpleStore struct {
	storeRecorder
	st map[string]string
}

func (s *simpleStore) Create(key string, _ bool, value string, _ bool, _ time.Time) (*store.Event, error) {
	if s.st == nil {
		s.st = make(map[string]string)
	}
	s.st[key] = value
	return nil, nil
}
func (s *simpleStore) Get(key string, _, _ bool) (*store.Event, error) {
	val, ok := s.st[key]
	if !ok {
		return nil, etcdErr.NewError(etcdErr.EcodeKeyNotFound, "", 0)
	}
	ev := &store.Event{Node: &store.NodeExtern{Key: key, Value: stringp(val)}}
	return ev, nil
}

// getAllStore embeds simpleStore, and makes Get return all keys sorted.
// It uses real store because it uses lots of logic in store and is not easy
// to mock.
// TODO: use mock one to do testing
type getAllStore struct {
	store.Store
}

func newGetAllStore() *getAllStore {
	return &getAllStore{store.New()}
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
