// Copyright 2015 The etcd Authors
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
	"fmt"
	"os"
	"time"

	"github.com/coreos/etcd/etcdserver/membership"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/mvcc"
	"github.com/coreos/etcd/mvcc/backend"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
	"github.com/coreos/etcd/snap"
)

// isConnectedToQuorumSince checks whether the local member is connected to the
// quorum of the cluster since the given time.
func isConnectedToQuorumSince(transport rafthttp.Transporter, since time.Time, self types.ID, members []*membership.Member) bool {
	return numConnectedSince(transport, since, self, members) >= (len(members)/2)+1
}

// isConnectedSince checks whether the local member is connected to the
// remote member since the given time.
func isConnectedSince(transport rafthttp.Transporter, since time.Time, remote types.ID) bool {
	t := transport.ActiveSince(remote)
	return !t.IsZero() && t.Before(since)
}

// isConnectedFullySince checks whether the local member is connected to all
// members in the cluster since the given time.
func isConnectedFullySince(transport rafthttp.Transporter, since time.Time, self types.ID, members []*membership.Member) bool {
	return numConnectedSince(transport, since, self, members) == len(members)
}

// numConnectedSince counts how many members are connected to the local member
// since the given time.
func numConnectedSince(transport rafthttp.Transporter, since time.Time, self types.ID, members []*membership.Member) int {
	connectedNum := 0
	for _, m := range members {
		if m.ID == self || isConnectedSince(transport, since, m.ID) {
			connectedNum++
		}
	}
	return connectedNum
}

// longestConnected chooses the member with longest active-since-time.
// It returns false, if nothing is active.
func longestConnected(tp rafthttp.Transporter, membs []types.ID) (types.ID, bool) {
	var longest types.ID
	var oldest time.Time
	for _, id := range membs {
		tm := tp.ActiveSince(id)
		if tm.IsZero() { // inactive
			continue
		}

		if oldest.IsZero() { // first longest candidate
			oldest = tm
			longest = id
		}

		if tm.Before(oldest) {
			oldest = tm
			longest = id
		}
	}
	if uint64(longest) == 0 {
		return longest, false
	}
	return longest, true
}

type notifier struct {
	c   chan struct{}
	err error
}

func newNotifier() *notifier {
	return &notifier{
		c: make(chan struct{}),
	}
}

func (nc *notifier) notify(err error) {
	nc.err = err
	close(nc.c)
}

// checkAndRecoverDB attempts to recover db in the scenario when
// etcd server crashes before updating its in-state db
// and after persisting snapshot to disk from syncing with leader,
// snapshot can be newer than db where
// (snapshot.Metadata.Index > db.consistentIndex ).
//
// when that happen:
// 1. find xxx.snap.db that matches snap index.
// 2. rename xxx.snap.db to db.
// 3. open the new db as the backend.
func checkAndRecoverDB(snapshot *raftpb.Snapshot, oldbe backend.Backend, quotaBackendBytes int64, snapdir string) (be backend.Backend, err error) {
	var cIndex consistentIndex
	kv := mvcc.New(oldbe, &lease.FakeLessor{}, &cIndex)
	defer kv.Close()
	kvindex := kv.ConsistentIndex()
	if snapshot.Metadata.Index <= kvindex {
		return oldbe, nil
	}

	id := snapshot.Metadata.Index
	snapfn, err := snap.DBFilePathFromID(snapdir, id)
	if err != nil {
		return nil, fmt.Errorf("finding %v error: %v", snapdir+fmt.Sprintf("%016x.snap.db", id), err)
	}

	bepath := snapdir + databaseFilename
	if err := os.Rename(snapfn, bepath); err != nil {
		return nil, fmt.Errorf("rename snapshot file error: %v", err)
	}

	oldbe.Close()
	be = openBackend(bepath, quotaBackendBytes)
	return be, nil
}

func openBackend(bepath string, quotaBackendBytes int64) (be backend.Backend) {
	beOpened := make(chan struct{})
	go func() {
		be = newBackend(bepath, quotaBackendBytes)
		beOpened <- struct{}{}
	}()

	select {
	case <-beOpened:
	case <-time.After(time.Second):
		plog.Warningf("another etcd process is running with the same data dir and holding the file lock.")
		plog.Warningf("waiting for it to exit before starting...")
		<-beOpened
	}
	return be
}
