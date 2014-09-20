package etcdserver

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

const machineKVPrefix = "/_etcd/machines"

type PeerInfo struct {
	Name       string
	ID         int64
	PeerURLs   []string
	ClientURLs []string
}

func (i PeerInfo) IsEmpty() bool {
	return i.ID == raft.None
}

type PeerStore struct {
	// TODO: use a particular store
	Store store.Store
}

func NewPeerStore(st store.Store, peers map[int64][]string) *PeerStore {
	ps := &PeerStore{Store: st}
	for id, addrs := range peers {
		urls := make([]string, len(addrs))
		for i := range addrs {
			urls[i] = fmt.Sprintf("http://%s", addrs[i])
		}
		ps.Create(id, PeerInfo{ID: id, PeerURLs: urls})
	}
	return ps
}

// Create creates a new peer.
// The given id MUST NOT exists in peer store.
func (s *PeerStore) Create(id int64, info PeerInfo) {
	p := filepath.Join(machineKVPrefix, strconv.FormatInt(id, 10))
	b, err := json.Marshal(info)
	if err != nil {
		log.Panicf("marshal peer info error: %v", err)
	}
	if _, err := s.Store.Set(p, false, string(b), store.Permanent); err != nil {
		log.Panicf("set peer should never fail: %v", err)
	}
}

// Get gets peer info for the given id.
// If it cannot find peer, it returns an empty PeerInfo.
func (s *PeerStore) Get(id int64) PeerInfo {
	var info PeerInfo
	p := filepath.Join(machineKVPrefix, strconv.FormatInt(id, 10))
	e, err := s.Store.Get(p, false, false)
	if err != nil {
		if v, ok := err.(*etcdErr.Error); !ok || v.ErrorCode != etcdErr.EcodeKeyNotFound {
			log.Panicf("get peer should never fail except not found: %v", err)
		}
		return info
	}
	if err := json.Unmarshal([]byte(*e.Node.Value), &info); err != nil {
		log.Panicf("unmarshal peer info error: %v", err)
	}
	return info
}

func (s *PeerStore) GetAll() []PeerInfo {
	e, err := s.Store.Get(machineKVPrefix, false, false)
	if err != nil {
		log.Panicf("get peers should never fail: %v", err)
	}
	infos := make([]PeerInfo, len(e.Node.Nodes))
	for i, n := range e.Node.Nodes {
		if err := json.Unmarshal([]byte(*n.Value), &infos[i]); err != nil {
			log.Panicf("unmarshal peer info error: %v", err)
		}
	}
	sort.Sort(PeerInfoSlice(infos))
	return infos
}

// Delete deletes peer.
// The given id MUST exist in peer store.
func (s *PeerStore) Delete(id int64) {
	p := filepath.Join(machineKVPrefix, strconv.FormatInt(id, 10))
	if _, err := s.Store.Delete(p, false, false); err != nil {
		log.Panicf("delete peer should never fail: %v", err)
	}
}

type PeerInfoSlice []PeerInfo

func (p PeerInfoSlice) Len() int           { return len(p) }
func (p PeerInfoSlice) Less(i, j int) bool { return p[i].ID < p[j].ID }
func (p PeerInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
