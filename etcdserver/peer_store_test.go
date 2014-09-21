package etcdserver

import (
	"reflect"
	"testing"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
)

func TestPeerStoreCreate(t *testing.T) {
	st := &storeCreateRecorder{}
	ps := &PeerStore{Store: st}
	ps.Create(1, PeerInfo{Name: "node", ID: 1})

	wcreates := []pair{
		{
			machineKVPrefix + "1",
			`{"Name":"node","ID":1,"PeerURLs":null,"ClientURLs":null}`,
		},
	}
	if !reflect.DeepEqual(st.creates, wcreates) {
		t.Error("creates = %v, want %v", st.creates, wcreates)
	}
}

func TestPeerStoreGet(t *testing.T) {
	ps := &PeerStore{Store: &simpleStore{}}
	ps.Create(1, PeerInfo{Name: "node", ID: 1})

	tests := []struct {
		id    int64
		winfo PeerInfo
	}{
		{1, PeerInfo{Name: "node", ID: 1}},
		{2, PeerInfo{}},
	}
	for i, tt := range tests {
		info := ps.Get(tt.id)
		if !reflect.DeepEqual(info, tt.winfo) {
			t.Errorf("#%d: info = %v, want %v", i, info, tt.winfo)
		}
	}
}

func TestPeerStoreGetAll(t *testing.T) {
	tests := []struct {
		infos  []PeerInfo
		winfos []PeerInfo
	}{
		{
			[]PeerInfo{{Name: "node1", ID: 1}},
			[]PeerInfo{{Name: "node1", ID: 1}},
		},
		{
			[]PeerInfo{},
			[]PeerInfo{},
		},
		{
			[]PeerInfo{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
			[]PeerInfo{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
		},
		{
			[]PeerInfo{{Name: "node2", ID: 2}, {Name: "node1", ID: 1}},
			[]PeerInfo{{Name: "node1", ID: 1}, {Name: "node2", ID: 2}},
		},
	}
	for i, tt := range tests {
		ps := &PeerStore{Store: &getAllStore{}}
		for _, info := range tt.infos {
			ps.Create(info.ID, info)
		}
		if g := ps.GetAll(); !reflect.DeepEqual(g, tt.winfos) {
			t.Errorf("#%d: infos = %v, want %v", i, g, tt.winfos)
		}
	}
}

func TestPeerStoreDelete(t *testing.T) {
	st := &storeDeleteRecorder{}
	ps := &PeerStore{Store: st}
	ps.Delete(1)

	wdeletes := []string{machineKVPrefix + "1"}
	if !reflect.DeepEqual(st.deletes, wdeletes) {
		t.Error("deletes = %v, want %v", st.deletes, wdeletes)
	}
}

type pair struct {
	key   string
	value string
}
type storeCreateRecorder struct {
	storeRecorder
	creates []pair
}

func (s *storeCreateRecorder) Create(key string, _ bool, value string, _ bool, _ time.Time) (*store.Event, error) {
	s.creates = append(s.creates, pair{key, value})
	return nil, nil
}

// simpleStore implementes basic create and get.
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
