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
	"io"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/jonboulle/clockwork"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
	dstorage "github.com/coreos/etcd/storage"
)

// clearUnusedSnapshotInterval specifies the time interval to wait
// before clearing unused snapshot.
// The newly created snapshot should be retrieved within one heartbeat
// interval because raft state machine retries to send snapshot
// to slow follower when receiving MsgHeartbeatResp from the follower.
// Set it as 5s to match the upper limit of heartbeat interval.
const clearUnusedSnapshotInterval = 5 * time.Second

type snapshot struct {
	r raftpb.Snapshot

	io.ReadCloser // used to read out v3 snapshot

	done chan struct{}
}

func newSnapshot(r raftpb.Snapshot, kv dstorage.Snapshot) *snapshot {
	done := make(chan struct{})
	pr, pw := io.Pipe()
	go func() {
		_, err := kv.WriteTo(pw)
		pw.CloseWithError(err)
		kv.Close()
		close(done)
	}()
	return &snapshot{
		r:          r,
		ReadCloser: pr,
		done:       done,
	}
}

func (s *snapshot) raft() raftpb.Snapshot { return s.r }

func (s *snapshot) isClosed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// TODO: remove snapshotStore. getSnap part could be put into memoryStorage.
type snapshotStore struct {
	kv dstorage.KV
	tr rafthttp.Transporter

	// send empty to reqsnapc to notify the channel receiver to send back latest
	// snapshot to snapc
	reqsnapc chan struct{}
	// a chan to receive the requested raft snapshot
	// snapshotStore will receive from the chan immediately after it sends empty to reqsnapc
	raftsnapc chan raftpb.Snapshot

	mu sync.Mutex // protect belowing vars
	// snap is nil iff there is no snapshot stored
	snap       *snapshot
	inUse      bool
	createOnce sync.Once // ensure at most one snapshot is created when no snapshot stored

	clock clockwork.Clock
}

func newSnapshotStore(kv dstorage.KV) *snapshotStore {
	return &snapshotStore{
		kv:        kv,
		reqsnapc:  make(chan struct{}),
		raftsnapc: make(chan raftpb.Snapshot),
		clock:     clockwork.NewRealClock(),
	}
}

// getSnap returns a snapshot.
// If there is no available snapshot, ErrSnapshotTemporarilyUnavaliable will be returned.
//
// If the snapshot stored is in use, it returns ErrSnapshotTemporarilyUnavailable.
// If there is no snapshot stored, it creates new snapshot
// asynchronously and returns ErrSnapshotTemporarilyUnavailable, so
// caller could get snapshot later when the snapshot is created.
// Otherwise, it returns the snapshot stored.
//
// The created snapshot is cleared from the snapshot store if it is
// either unused after clearUnusedSnapshotInterval, or explicitly cleared
// through clearUsedSnap after using.
// closeSnapBefore is used to close outdated snapshot,
// so the snapshot will be cleared faster when in use.
//
// snapshot store stores at most one snapshot at a time.
// If raft state machine wants to send two snapshot messages to two followers,
// the second snapshot message will keep getting snapshot and succeed only after
// the first message is sent. This increases the time used to send messages,
// but it is acceptable because this should happen seldomly.
func (ss *snapshotStore) getSnap() (*snapshot, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.inUse {
		return nil, raft.ErrSnapshotTemporarilyUnavailable
	}

	if ss.snap == nil {
		// create snapshot asynchronously
		ss.createOnce.Do(func() { go ss.createSnap() })
		return nil, raft.ErrSnapshotTemporarilyUnavailable
	}

	ss.inUse = true
	// give transporter the generated snapshot that is ready to send out
	ss.tr.SnapshotReady(ss.snap, ss.snap.raft().Metadata.Index)
	return ss.snap, nil
}

// clearUsedSnap clears the snapshot from the snapshot store after it
// is used.
// After clear, snapshotStore could create new snapshot when getSnap.
func (ss *snapshotStore) clearUsedSnap() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if !ss.inUse {
		plog.Panicf("unexpected clearUsedSnap when snapshot is not in use")
	}
	ss.clear()
}

// closeSnapBefore closes the stored snapshot if its index is not greater
// than the given compact index.
// If it closes the snapshot, it returns true.
func (ss *snapshotStore) closeSnapBefore(index uint64) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.snap != nil && ss.snap.raft().Metadata.Index <= index {
		if err := ss.snap.Close(); err != nil {
			plog.Errorf("snapshot close error (%v)", err)
		}
		return true
	}
	return false
}

// createSnap creates a new snapshot and stores it into the snapshot store.
// It also sets a timer to clear the snapshot if it is not in use after
// some time interval.
// It should only be called in snapshotStore functions.
func (ss *snapshotStore) createSnap() {
	// ask to generate v2 snapshot
	ss.reqsnapc <- struct{}{}
	// generate KV snapshot
	kvsnap := ss.kv.Snapshot()
	raftsnap := <-ss.raftsnapc
	snap := newSnapshot(raftsnap, kvsnap)

	ss.mu.Lock()
	ss.snap = snap
	ss.mu.Unlock()

	go func() {
		<-ss.clock.After(clearUnusedSnapshotInterval)
		ss.mu.Lock()
		defer ss.mu.Unlock()
		if snap == ss.snap && !ss.inUse {
			ss.clear()
		}
	}()
}

// clear clears snapshot related variables in snapshotStore. It closes
// the snapshot stored and sets the variables to initial values.
// It should only be called in snapshotStore functions.
func (ss *snapshotStore) clear() {
	if err := ss.snap.Close(); err != nil {
		plog.Errorf("snapshot close error (%v)", err)
	}
	ss.snap = nil
	ss.inUse = false
	ss.createOnce = sync.Once{}
}
