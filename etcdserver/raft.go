/*
   Copyright 2015 CoreOS, Inc.

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
	"log"
	"os"
	"sort"
	"sync/atomic"
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

type RaftTimer interface {
	Index() uint64
	Term() uint64
}

type apply struct {
	entries   []raftpb.Entry
	snapshot  raftpb.Snapshot
	confState *raftpb.ConfState
	done      chan uint64
}

type raftNode struct {
	raft.Node
	applyc chan apply

	// TODO: remove the etcdserver related logic from raftNode
	// TODO: add a state machine interface to apply the commit entries
	// and do snapshot/recover
	s *EtcdServer

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

func (r *raftNode) run() {
	var syncC <-chan time.Time

	// load initial state from raft storage
	snap, err := r.raftStorage.Snapshot()
	if err != nil {
		log.Panicf("etcdserver: get snapshot from raft storage error: %v", err)
	}
	// snapi indicates the index of the last submitted snapshot request
	snapi := snap.Metadata.Index
	confState := snap.Metadata.ConfState

	defer r.stop()
	for {
		select {
		case <-r.ticker:
			r.Tick()
		case rd := <-r.Ready():
			if rd.SoftState != nil {
				atomic.StoreUint64(&r.lead, rd.SoftState.Lead)
				if rd.RaftState == raft.StateLeader {
					syncC = r.s.SyncTicker
					// TODO: remove the nil checking
					// current test utility does not provide the stats
					if r.s.stats != nil {
						r.s.stats.BecomeLeader()
					}
				} else {
					syncC = nil
				}
			}

			apply := apply{
				entries:   rd.CommittedEntries,
				snapshot:  rd.Snapshot,
				confState: &confState,
				done:      make(chan uint64),
			}
			r.applyc <- apply

			// apply snapshot to storage if it is more updated than current snapi
			if !raft.IsEmptySnap(rd.Snapshot) && rd.Snapshot.Metadata.Index > snapi {
				if err := r.storage.SaveSnap(rd.Snapshot); err != nil {
					log.Fatalf("etcdraft: save snapshot error: %v", err)
				}
				r.raftStorage.ApplySnapshot(rd.Snapshot)
				snapi = rd.Snapshot.Metadata.Index
				log.Printf("etcdraft: saved incoming snapshot at index %d", snapi)
			}

			if err := r.storage.Save(rd.HardState, rd.Entries); err != nil {
				log.Fatalf("etcdraft: save state and entries error: %v", err)
			}
			r.raftStorage.Append(rd.Entries)

			r.s.send(rd.Messages)

			appliedi := <-apply.done
			r.Advance()

			if appliedi-snapi > r.snapCount {
				log.Printf("etcdserver: start to snapshot (applied: %d, lastsnap: %d)", appliedi, snapi)
				r.s.snapshot(appliedi, &confState)
				snapi = appliedi
			}
		case <-syncC:
			r.s.sync(defaultSyncTimeout)
		case <-r.s.done:
			return
		}
	}
}

func (r *raftNode) apply() chan apply {
	return r.applyc
}

func (r *raftNode) stop() {
	r.Stop()
	r.transport.Stop()
	if err := r.storage.Close(); err != nil {
		log.Panicf("etcdraft: close storage error: %v", err)
	}
}

// Implement the RaftTimer interface
func (r *raftNode) Index() uint64 { return atomic.LoadUint64(&r.index) }

func (r *raftNode) Term() uint64 { return atomic.LoadUint64(&r.term) }

// Only for testing purpose
// TODO: add Raft server interface to expose raft related info:
// Index, Term, Lead, Committed, Applied, LastIndex, etc.
func (r *raftNode) Lead() uint64 { return atomic.LoadUint64(&r.lead) }

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
	n := raft.RestartNode(uint64(id), cfg.ElectionTicks, 1, s)
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
	n := raft.RestartNode(uint64(id), cfg.ElectionTicks, 1, s)
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
