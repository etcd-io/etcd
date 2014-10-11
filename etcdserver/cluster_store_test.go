package etcdserver

import (
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
				membersKVPrefix + "1/raftAttributes",
				false,
				`{"PeerURLs":null}`,
				false,
				store.Permanent,
			},
		},
		{
			name: "Create",
			params: []interface{}{
				membersKVPrefix + "1/attributes",
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
		cs := &clusterStore{Store: newGetAllStore()}
		for _, m := range tt.mems {
			cs.Add(m)
		}
		c := NewCluster()
		if err := c.AddSlice(tt.mems); err != nil {
			t.Fatal(err)
		}
		if g := cs.Get(); !reflect.DeepEqual(&g, c) {
			t.Errorf("#%d: mems = %v, want %v", i, &g, c)
		}
	}
}

func TestClusterStoreDelete(t *testing.T) {
	st := newStoreGetAllAndDeleteRecorder()
	cs := &clusterStore{Store: st}
	cs.Add(newTestMember(1, nil, "node1", nil))
	cs.Remove(1)

	wdeletes := []string{membersKVPrefix + "1"}
	if !reflect.DeepEqual(st.deletes, wdeletes) {
		t.Error("deletes = %v, want %v", st.deletes, wdeletes)
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

type storeGetAllAndDeleteRecorder struct {
	*getAllStore
	deletes []string
}

func newStoreGetAllAndDeleteRecorder() *storeGetAllAndDeleteRecorder {
	return &storeGetAllAndDeleteRecorder{getAllStore: newGetAllStore()}
}

func (s *storeGetAllAndDeleteRecorder) Delete(key string, _, _ bool) (*store.Event, error) {
	s.deletes = append(s.deletes, key)
	return nil, nil
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
