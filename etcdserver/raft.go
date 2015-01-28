// Copyright 2015 CoreOS, Inc.
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

package etcdserver

import (
	"encoding/json"
	"expvar"
	"log"
	"os"
	"sort"
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
)

var (
	// indirection for expvar func interface
	// expvar panics when publishing duplicate name
	// expvar does not support remove a registered name
	// so only register a func that calls raftStatus
	// and change raftStatus as we need.
	raftStatus func() raft.Status
)

func init() {
	expvar.Publish("raft.status", expvar.Func(func() interface{} { return raftStatus() }))
}

type RaftTimer interface {
	Index() uint64
	Term() uint64
}

type raftNode struct {
	raft.Node

	// config
	snapCount uint64 // number of entries to trigger a snapshot

	// utility
	ticker      <-chan time.Time
	raftStorage *raft.MemoryStorage
	storage     Storage
	// transport specifies the transport to send and receive msgs to members.
	// Sending messages MUST NOT block. It is okay to drop messages, since
	// clients should timeout and reissue their messages.
	// If transport is nil, server will panic.
	transport rafthttp.Transporter

	// Cache of the latest raft index and raft term the server has seen
	index uint64
	term  uint64
	lead  uint64
}

// for testing
func (r *raftNode) pauseSending() {
	p := r.transport.(rafthttp.Pausable)
	p.Pause()
}

func (r *raftNode) resumeSending() {
	p := r.transport.(rafthttp.Pausable)
	p.Resume()
}

func startNode(cfg *ServerConfig, ids []types.ID) (id types.ID, n raft.Node, s *raft.MemoryStorage, w *wal.WAL) {
	var err error
	member := cfg.Cluster.MemberByName(cfg.Name)
	metadata := pbutil.MustMarshal(
		&pb.Metadata{
			NodeID:    uint64(member.ID),
			ClusterID: uint64(cfg.Cluster.ID()),
		},
	)
	if err := os.MkdirAll(cfg.SnapDir(), privateDirMode); err != nil {
		log.Fatalf("etcdserver create snapshot directory error: %v", err)
	}
	if w, err = wal.Create(cfg.WALDir(), metadata); err != nil {
		log.Fatalf("etcdserver: create wal error: %v", err)
	}
	peers := make([]raft.Peer, len(ids))
	for i, id := range ids {
		ctx, err := json.Marshal((*cfg.Cluster).Member(id))
		if err != nil {
			log.Panicf("marshal member should never fail: %v", err)
		}
		peers[i] = raft.Peer{ID: uint64(id), Context: ctx}
	}
	id = member.ID
	log.Printf("etcdserver: start member %s in cluster %s", id, cfg.Cluster.ID())
	s = raft.NewMemoryStorage()
	n = raft.StartNode(uint64(id), peers, cfg.ElectionTicks, 1, s)
	raftStatus = n.Status
	return
}

func restartNode(cfg *ServerConfig, snapshot *raftpb.Snapshot) (types.ID, raft.Node, *raft.MemoryStorage, *wal.WAL) {
	var walsnap walpb.Snapshot
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	w, id, cid, st, ents := readWAL(cfg.WALDir(), walsnap)
	cfg.Cluster.SetID(cid)

	log.Printf("etcdserver: restart member %s in cluster %s at commit index %d", id, cfg.Cluster.ID(), st.Commit)
	s := raft.NewMemoryStorage()
	if snapshot != nil {
		s.ApplySnapshot(*snapshot)
	}
	s.SetHardState(st)
	s.Append(ents)
	n := raft.RestartNode(uint64(id), cfg.ElectionTicks, 1, s, 0)
	raftStatus = n.Status
	return id, n, s, w
}

func restartAsStandaloneNode(cfg *ServerConfig, snapshot *raftpb.Snapshot) (types.ID, raft.Node, *raft.MemoryStorage, *wal.WAL) {
	var walsnap walpb.Snapshot
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	w, id, cid, st, ents := readWAL(cfg.WALDir(), walsnap)
	cfg.Cluster.SetID(cid)

	// discard the previously uncommitted entries
	for i, ent := range ents {
		if ent.Index > st.Commit {
			log.Printf("etcdserver: discarding %d uncommitted WAL entries ", len(ents)-i)
			ents = ents[:i]
			break
		}
	}

	// force append the configuration change entries
	toAppEnts := createConfigChangeEnts(getIDs(snapshot, ents), uint64(id), st.Term, st.Commit)
	ents = append(ents, toAppEnts...)

	// force commit newly appended entries
	err := w.Save(raftpb.HardState{}, toAppEnts)
	if err != nil {
		log.Fatalf("etcdserver: %v", err)
	}
	if len(ents) != 0 {
		st.Commit = ents[len(ents)-1].Index
	}

	log.Printf("etcdserver: forcing restart of member %s in cluster %s at commit index %d", id, cfg.Cluster.ID(), st.Commit)
	s := raft.NewMemoryStorage()
	if snapshot != nil {
		s.ApplySnapshot(*snapshot)
	}
	s.SetHardState(st)
	s.Append(ents)
	n := raft.RestartNode(uint64(id), cfg.ElectionTicks, 1, s, 0)
	raftStatus = n.Status
	return id, n, s, w
}

// getIDs returns an ordered set of IDs included in the given snapshot and
// the entries. The given snapshot/entries can contain two kinds of
// ID-related entry:
// - ConfChangeAddNode, in which case the contained ID will be added into the set.
// - ConfChangeAddRemove, in which case the contained ID will be removed from the set.
func getIDs(snap *raftpb.Snapshot, ents []raftpb.Entry) []uint64 {
	ids := make(map[uint64]bool)
	if snap != nil {
		for _, id := range snap.Metadata.ConfState.Nodes {
			ids[id] = true
		}
	}
	for _, e := range ents {
		if e.Type != raftpb.EntryConfChange {
			continue
		}
		var cc raftpb.ConfChange
		pbutil.MustUnmarshal(&cc, e.Data)
		switch cc.Type {
		case raftpb.ConfChangeAddNode:
			ids[cc.NodeID] = true
		case raftpb.ConfChangeRemoveNode:
			delete(ids, cc.NodeID)
		default:
			log.Panicf("ConfChange Type should be either ConfChangeAddNode or ConfChangeRemoveNode!")
		}
	}
	sids := make(types.Uint64Slice, 0)
	for id := range ids {
		sids = append(sids, id)
	}
	sort.Sort(sids)
	return []uint64(sids)
}

// createConfigChangeEnts creates a series of Raft entries (i.e.
// EntryConfChange) to remove the set of given IDs from the cluster. The ID
// `self` is _not_ removed, even if present in the set.
// If `self` is not inside the given ids, it creates a Raft entry to add a
// default member with the given `self`.
func createConfigChangeEnts(ids []uint64, self uint64, term, index uint64) []raftpb.Entry {
	ents := make([]raftpb.Entry, 0)
	next := index + 1
	found := false
	for _, id := range ids {
		if id == self {
			found = true
			continue
		}
		cc := &raftpb.ConfChange{
			Type:   raftpb.ConfChangeRemoveNode,
			NodeID: id,
		}
		e := raftpb.Entry{
			Type:  raftpb.EntryConfChange,
			Data:  pbutil.MustMarshal(cc),
			Term:  term,
			Index: next,
		}
		ents = append(ents, e)
		next++
	}
	if !found {
		m := Member{
			ID:             types.ID(self),
			RaftAttributes: RaftAttributes{PeerURLs: []string{"http://localhost:7001", "http://localhost:2380"}},
		}
		ctx, err := json.Marshal(m)
		if err != nil {
			log.Panicf("marshal member should never fail: %v", err)
		}
		cc := &raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  self,
			Context: ctx,
		}
		e := raftpb.Entry{
			Type:  raftpb.EntryConfChange,
			Data:  pbutil.MustMarshal(cc),
			Term:  term,
			Index: next,
		}
		ents = append(ents, e)
	}
	return ents
}
