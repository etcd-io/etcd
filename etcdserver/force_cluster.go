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
	"log"

	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal"
)

func restartAsStandaloneNode(cfg *ServerConfig, index uint64, snapshot *raftpb.Snapshot) (id types.ID, n raft.Node, w *wal.WAL) {
	var err error
	if w, err = wal.OpenAtIndex(cfg.WALDir(), index); err != nil {
		log.Fatalf("etcdserver: open wal error: %v", err)
	}
	id, cid, st, ents, err := readWAL(w, index)
	if err != nil {
		log.Fatalf("etcdserver: read wal error: %v", err)
	}
	cfg.Cluster.SetID(cid)

	// discard the previously uncommitted entries
	if len(ents) != 0 {
		ents = ents[:st.Commit+1]
	}

	// force append the configuration change entries
	toAppEnts := createConfigChangeEnts(getIDset(snapshot, ents), uint64(id), st.Term, st.Commit)
	ents = append(ents, toAppEnts...)

	// force commit newly appended entries
	for _, e := range toAppEnts {
		err := w.SaveEntry(&e)
		if err != nil {
			log.Fatalf("etcdserver: %v", err)
		}
	}
	if len(ents) != 0 {
		st.Commit = ents[len(ents)-1].Index
	}

	log.Printf("etcdserver: forcing restart of member %s in cluster %s at commit index %d", id, cfg.Cluster.ID(), st.Commit)
	n = raft.RestartNode(uint64(id), 10, 1, snapshot, st, ents)
	return
}

func getIDset(snap *raftpb.Snapshot, ents []raftpb.Entry) map[uint64]bool {
	ids := make(map[uint64]bool)
	if snap != nil {
		for _, id := range snap.Nodes {
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
	return ids
}

func createConfigChangeEnts(ids map[uint64]bool, self uint64, term, index uint64) []raftpb.Entry {
	ents := make([]raftpb.Entry, 0)
	next := index + 1
	for id := range ids {
		if id == self {
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
	return ents
}
