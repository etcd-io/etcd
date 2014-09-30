package etcdserver

import (
	"reflect"
	"testing"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
)

func TestClusterStoreCreate(t *testing.T) {
	st := &storeRecorder{}
	ps := &clusterStore{Store: st}
	ps.Create(Member{Name: "node", ID: 1})

	wactions := []action{
		{
			name: "Create",
			params: []interface{}{
				machineKVPrefix + "1",
				false,
				`{"ID":1,"Name":"node","PeerURLs":null,"ClientURLs":null}`,
				false,
				store.Permanent,
			},
		},
	}
	if g := st.Action(); !reflect.DeepEqual(g, wactions) {
		t.Error("actions = %v, want %v", g, wactions)
	}
}

func TestClusterStoreGet(t *testing.T) {
	tests := []struct {
		mems  []Member
		wmems []Member
	}{
		{
			[]Member{{Name: "node1", ID: 1}},
			[]Member{{Name: "node1", ID: 1}},
		},
		{
			[]Member{},
			[]Member{},
		},
		{
			[]Member{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
			[]Member{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
		},
		{
			[]Member{{Name: "node2", ID: 2}, {Name: "node1", ID: 1}},
			[]Member{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
		},
	}
	for i, tt := range tests {
		c := Cluster{}
		err := c.AddSlice(tt.mems)
		if err != nil {
			t.Error(err)
		}

		cs := NewClusterStore(&getAllStore{}, c)

		if g := cs.Get(); !reflect.DeepEqual(g, c) {
			t.Errorf("#%d: mems = %v, want %v", i, g, c)
		}
	}
}

func TestClusterStoreDelete(t *testing.T) {
	st := &storeGetAllDeleteRecorder{}
	c := Cluster{}
	c.Add(Member{Name: "node", ID: 1})
	cs := NewClusterStore(st, c)
	cs.Delete(1)

	wdeletes := []string{machineKVPrefix + "1"}
	if !reflect.DeepEqual(st.deletes, wdeletes) {
		t.Error("deletes = %v, want %v", st.deletes, wdeletes)
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

// getAllStore inherits simpleStore, and makes Get return all keys.
type getAllStore struct {
	simpleStore
}

func (s *getAllStore) Get(_ string, _, _ bool) (*store.Event, error) {
	nodes := make([]*store.NodeExtern, 0)
	for k, v := range s.st {
		nodes = append(nodes, &store.NodeExtern{Key: k, Value: stringp(v)})
	}
	return &store.Event{Node: &store.NodeExtern{Nodes: nodes}}, nil
}

type storeDeleteRecorder struct {
	storeRecorder
	deletes []string
}

func (s *storeDeleteRecorder) Delete(key string, _, _ bool) (*store.Event, error) {
	s.deletes = append(s.deletes, key)
	return nil, nil
}

type storeGetAllDeleteRecorder struct {
	getAllStore
	deletes []string
}

func (s *storeGetAllDeleteRecorder) Delete(key string, _, _ bool) (*store.Event, error) {
	s.deletes = append(s.deletes, key)
	return nil, nil
}
