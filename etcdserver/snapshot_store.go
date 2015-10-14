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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
	dstorage "github.com/coreos/etcd/storage"
)

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

// TODO: remove snapshotStore. getSnap part could be put into memoryStorage,
// while SaveFrom could be put into another struct, or even put into dstorage package.
type snapshotStore struct {
	// dir to save snapshot data
	dir string
	kv  dstorage.KV
	tr  rafthttp.Transporter

	// send empty to reqsnapc to notify the channel receiver to send back latest
	// snapshot to snapc
	reqsnapc chan struct{}
	// a chan to receive the requested raft snapshot
	// snapshotStore will receive from the chan immediately after it sends empty to reqsnapc
	raftsnapc chan raftpb.Snapshot

	snap *snapshot
}

func newSnapshotStore(dir string, kv dstorage.KV) *snapshotStore {
	return &snapshotStore{
		dir:       dir,
		kv:        kv,
		reqsnapc:  make(chan struct{}),
		raftsnapc: make(chan raftpb.Snapshot),
	}
}

// getSnap returns a snapshot.
// If there is no available snapshot, ErrSnapshotTemporarilyUnavaliable will be returned.
//
// Internally it creates new snapshot and returns the snapshot. Unless the
// returned snapshot is closed, it rejects creating new one and returns
// ErrSnapshotTemporarilyUnavailable.
// If raft state machine wants to send two snapshot messages to two followers,
// the second snapshot message will keep getting snapshot and succeed only after
// the first message is sent. This increases the time used to send messages,
// but it is acceptable because this should happen seldomly.
func (ss *snapshotStore) getSnap() (*snapshot, error) {
	// If snapshotStore has some snapshot that has not been closed, it cannot
	// request new snapshot. So it returns ErrSnapshotTemporarilyUnavailable.
	if ss.snap != nil && !ss.snap.isClosed() {
		return nil, raft.ErrSnapshotTemporarilyUnavailable
	}

	// ask to generate v2 snapshot
	ss.reqsnapc <- struct{}{}
	// generate KV snapshot
	kvsnap := ss.kv.Snapshot()
	raftsnap := <-ss.raftsnapc
	ss.snap = newSnapshot(raftsnap, kvsnap)
	// give transporter the generated snapshot that is ready to send out
	ss.tr.SnapshotReady(ss.snap, raftsnap.Metadata.Index)
	return ss.snap, nil
}

// SaveFrom saves snapshot at the given index from the given reader.
// If the snapshot with the given index has been saved successfully, it keeps
// the original saved snapshot and returns error.
// The function guarantees that SaveFrom always saves either complete
// snapshot or no snapshot, even if the call is aborted because program
// is hard killed.
func (ss *snapshotStore) SaveFrom(r io.Reader, index uint64) error {
	f, err := ioutil.TempFile(ss.dir, "tmp")
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	fn := path.Join(ss.dir, fmt.Sprintf("%016x.db", index))
	if fileutil.Exist(fn) {
		os.Remove(f.Name())
		return fmt.Errorf("snapshot to save has existed")
	}
	err = os.Rename(f.Name(), fn)
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

// getSnapFilePath returns the file path for the snapshot with given index.
// If the snapshot does not exist, it returns error.
func (ss *snapshotStore) getSnapFilePath(index uint64) (string, error) {
	fns, err := fileutil.ReadDir(ss.dir)
	if err != nil {
		return "", err
	}
	wfn := fmt.Sprintf("%016x.db", index)
	for _, fn := range fns {
		if fn == wfn {
			return path.Join(ss.dir, fn), nil
		}
	}
	return "", fmt.Errorf("snapshot file doesn't exist")
}
