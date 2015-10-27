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
	"reflect"
	"sync"
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/jonboulle/clockwork"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	dstorage "github.com/coreos/etcd/storage"
	"github.com/coreos/etcd/storage/storagepb"
)

func TestSnapshotStoreCreateSnap(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{Index: 1},
	}
	ss := newSnapshotStore("", &nopKV{})
	fakeClock := clockwork.NewFakeClock()
	ss.clock = fakeClock
	go func() {
		<-ss.reqsnapc
		ss.raftsnapc <- snap
	}()

	// create snapshot
	ss.createSnap()
	if !reflect.DeepEqual(ss.snap.raft(), snap) {
		t.Errorf("raftsnap = %+v, want %+v", ss.snap.raft(), snap)
	}

	// unused snapshot is cleared after clearUnusedSnapshotInterval
	fakeClock.BlockUntil(1)
	fakeClock.Advance(clearUnusedSnapshotInterval)
	testutil.WaitSchedule()
	ss.mu.Lock()
	if ss.snap != nil {
		t.Errorf("snap = %+v, want %+v", ss.snap, nil)
	}
	ss.mu.Unlock()
}

func TestSnapshotStoreGetSnap(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{Index: 1},
	}
	ss := newSnapshotStore("", &nopKV{})
	fakeClock := clockwork.NewFakeClock()
	ss.clock = fakeClock
	ss.tr = &nopTransporter{}
	go func() {
		<-ss.reqsnapc
		ss.raftsnapc <- snap
	}()

	// get snap when no snapshot stored
	_, err := ss.getSnap()
	if err != raft.ErrSnapshotTemporarilyUnavailable {
		t.Fatalf("getSnap error = %v, want %v", err, raft.ErrSnapshotTemporarilyUnavailable)
	}

	// wait for asynchronous snapshot creation to finish
	testutil.WaitSchedule()
	// get the created snapshot
	s, err := ss.getSnap()
	if err != nil {
		t.Fatalf("getSnap error = %v, want nil", err)
	}
	if !reflect.DeepEqual(s.raft(), snap) {
		t.Errorf("raftsnap = %+v, want %+v", s.raft(), snap)
	}
	if !ss.inUse {
		t.Errorf("inUse = %v, want true", ss.inUse)
	}

	// get snap when snapshot stored has been in use
	_, err = ss.getSnap()
	if err != raft.ErrSnapshotTemporarilyUnavailable {
		t.Fatalf("getSnap error = %v, want %v", err, raft.ErrSnapshotTemporarilyUnavailable)
	}

	// clean up
	fakeClock.Advance(clearUnusedSnapshotInterval)
}

func TestSnapshotStoreClearUsedSnap(t *testing.T) {
	s := &fakeSnapshot{}
	var once sync.Once
	once.Do(func() {})
	ss := &snapshotStore{
		snap:       newSnapshot(raftpb.Snapshot{}, s),
		inUse:      true,
		createOnce: once,
	}

	ss.clearUsedSnap()
	// wait for underlying KV snapshot closed
	testutil.WaitSchedule()
	s.mu.Lock()
	if !s.closed {
		t.Errorf("snapshot closed = %v, want true", s.closed)
	}
	s.mu.Unlock()
	if ss.snap != nil {
		t.Errorf("snapshot = %v, want nil", ss.snap)
	}
	if ss.inUse {
		t.Errorf("isUse = %v, want false", ss.inUse)
	}
	// test createOnce is reset
	if ss.createOnce == once {
		t.Errorf("createOnce fails to reset")
	}
}

func TestSnapshotStoreCloseSnapBefore(t *testing.T) {
	snapIndex := uint64(5)

	tests := []struct {
		index uint64
		wok   bool
	}{
		{snapIndex - 2, false},
		{snapIndex - 1, false},
		{snapIndex, true},
	}
	for i, tt := range tests {
		rs := raftpb.Snapshot{
			Metadata: raftpb.SnapshotMetadata{Index: 5},
		}
		s := &fakeSnapshot{}
		ss := &snapshotStore{
			snap: newSnapshot(rs, s),
		}

		ok := ss.closeSnapBefore(tt.index)
		if ok != tt.wok {
			t.Errorf("#%d: closeSnapBefore = %v, want %v", i, ok, tt.wok)
		}
		if ok {
			// wait for underlying KV snapshot closed
			testutil.WaitSchedule()
			s.mu.Lock()
			if !s.closed {
				t.Errorf("#%d: snapshot closed = %v, want true", i, s.closed)
			}
			s.mu.Unlock()
		}
	}
}

type nopKV struct{}

func (kv *nopKV) Rev() int64 { return 0 }
func (kv *nopKV) Range(key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error) {
	return nil, 0, nil
}
func (kv *nopKV) Put(key, value []byte) (rev int64)          { return 0 }
func (kv *nopKV) DeleteRange(key, end []byte) (n, rev int64) { return 0, 0 }
func (kv *nopKV) TxnBegin() int64                            { return 0 }
func (kv *nopKV) TxnEnd(txnID int64) error                   { return nil }
func (kv *nopKV) TxnRange(txnID int64, key, end []byte, limit, rangeRev int64) (kvs []storagepb.KeyValue, rev int64, err error) {
	return nil, 0, nil
}
func (kv *nopKV) TxnPut(txnID int64, key, value []byte) (rev int64, err error) { return 0, nil }
func (kv *nopKV) TxnDeleteRange(txnID int64, key, end []byte) (n, rev int64, err error) {
	return 0, 0, nil
}
func (kv *nopKV) Compact(rev int64) error     { return nil }
func (kv *nopKV) Hash() (uint32, error)       { return 0, nil }
func (kv *nopKV) Snapshot() dstorage.Snapshot { return &fakeSnapshot{} }
func (kv *nopKV) Commit()                     {}
func (kv *nopKV) Restore() error              { return nil }
func (kv *nopKV) Close() error                { return nil }

type fakeSnapshot struct {
	mu     sync.Mutex
	closed bool
}

func (s *fakeSnapshot) Size() int64                        { return 0 }
func (s *fakeSnapshot) WriteTo(w io.Writer) (int64, error) { return 0, nil }
func (s *fakeSnapshot) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}
