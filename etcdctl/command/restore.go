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

package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
)

func NewRestoreCommand() cli.Command {
	return cli.Command{
		Name:  "restore",
		Usage: "restore a backup etcd directory",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "backup-dir", Value: "", Usage: "Path to the etcd backup dir"},
			cli.StringFlag{Name: "restore-dir", Value: "", Usage: "Path to the restore dir"},
			cli.StringFlag{Name: "cluster-configuration", Value: "", Usage: "restored configuration"},
			cli.StringFlag{Name: "name", Value: "", Usage: "member name to restore"},
		},
		Action: handleRestore,
	}
}

// handleBackup handles a request that intends to do a backup.
func handleRestore(c *cli.Context) {
	srcSnap := path.Join(c.String("backup-dir"), "snap")
	destSnap := path.Join(c.String("restore-dir"), "snap")
	srcWAL := path.Join(c.String("backup-dir"), "wal")
	destWAL := path.Join(c.String("restore-dir"), "wal")

	if err := os.MkdirAll(destSnap, 0700); err != nil {
		log.Fatalf("failed creating backup snapshot dir %v: %v", destSnap, err)
	}
	ss := snap.New(srcSnap)
	snapshot, err := ss.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		log.Fatal(err)
	}
	var walsnap walpb.Snapshot
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}

	w, err := wal.OpenNotInUse(srcWAL, walsnap)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()
	wmetadata, state, ents, err := w.ReadAll()
	switch err {
	case nil:
	case wal.ErrSnapshotNotFound:
		fmt.Printf("Failed to find the match snapshot record %+v in wal %v.", walsnap, srcWAL)
		fmt.Printf("etcdctl will add it back. Start auto fixing...")
	default:
		log.Fatal(err)
	}
	var metadata etcdserverpb.Metadata
	pbutil.MustUnmarshal(&metadata, wmetadata)
	if metadata.NodeID != 0 {
		log.Fatalf("The given backup datadir is not a backup datadir: found member %s", types.ID(metadata.NodeID))
	}
	rand.Seed(int64(metadata.ClusterID))

	id, confs, confState, err := restoreConf(c.String("cluster-configuration"), c.String("name"), ents[len(ents)-1].Term, ents[len(ents)-1].Index)
	if err != nil {
		log.Fatal(err)
	}

	snapshot.Metadata.ConfState = *confState
	newss := snap.New(destSnap)
	if err := newss.SaveSnap(*snapshot); err != nil {
		log.Fatal(err)
	}

	metadata.NodeID = id
	neww, err := wal.Create(destWAL, pbutil.MustMarshal(&metadata))
	if err != nil {
		log.Fatal(err)
	}
	defer neww.Close()

	ents = append(ents, confs...)
	state.Commit = ents[len(ents)-1].Index
	if err := neww.Save(state, ents); err != nil {
		log.Fatal(err)
	}
	if err := neww.SaveSnapshot(walsnap); err != nil {
		log.Fatal(err)
	}
}

func restoreConf(cluster string, name string, term, index uint64) (uint64, []raftpb.Entry, *raftpb.ConfState, error) {
	c, err := etcdserver.NewClusterFromString("", cluster)
	if err != nil {
		return 0, nil, nil, err
	}

	var memberID uint64
	memberIDs := make([]uint64, 0)
	ents := make([]raftpb.Entry, 0)
	next := index

	for _, member := range c.Members() {
		id := uint64(rand.Int63())
		m := etcdserver.Member{
			ID:             types.ID(id),
			RaftAttributes: etcdserver.RaftAttributes{PeerURLs: member.PeerURLs},
		}
		ctx, err := json.Marshal(m)
		if err != nil {
			log.Panicf("marshal member should never fail: %v", err)
		}
		cc := &raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  id,
			Context: ctx,
		}
		e := raftpb.Entry{
			Type:  raftpb.EntryConfChange,
			Data:  pbutil.MustMarshal(cc),
			Term:  term,
			Index: next,
		}
		next++
		ents = append(ents, e)
		memberIDs = append(memberIDs, id)
		if member.Name == name {
			memberID = id
		}
	}

	if memberID == 0 {
		return 0, nil, nil, errors.New("cannot find name in cluster configuration")
	}

	return memberID, ents, &raftpb.ConfState{Nodes: memberIDs}, nil
}
