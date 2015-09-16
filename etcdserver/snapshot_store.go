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
	dstorage "github.com/coreos/etcd/storage"
)

type snapshot struct {
	r  raftpb.Snapshot
	kv dstorage.Snapshot
}

func (s *snapshot) raft() raftpb.Snapshot { return s.r }

func (s *snapshot) size() int64 { return s.kv.Size() }

func (s *snapshot) writeTo(w io.Writer) (n int64, err error) { return s.kv.WriteTo(w) }

func (s *snapshot) close() error { return s.kv.Close() }

type snapshotStore struct {
	// dir to save snapshot data
	dir string
	kv  dstorage.KV

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
func (ss *snapshotStore) getSnap() (*snapshot, error) {
	if ss.snap != nil {
		return nil, raft.ErrSnapshotTemporarilyUnavailable
	}

	// ask to generate v2 snapshot
	ss.reqsnapc <- struct{}{}
	// generate KV snapshot
	kvsnap := ss.kv.Snapshot()
	raftsnap := <-ss.raftsnapc
	ss.snap = &snapshot{
		r:  raftsnap,
		kv: kvsnap,
	}
	return ss.snap, nil
}

// saveSnap saves snapshot into disk.
//
// If snapshot has existed in disk, it keeps the original snapshot and returns error.
// The function guarantees that it always saves either complete snapshot or no snapshot,
// even if the call is aborted because program is hard killed.
func (ss *snapshotStore) saveSnap(s *snapshot) error {
	f, err := ioutil.TempFile(ss.dir, "tmp")
	if err != nil {
		return err
	}
	_, err = s.writeTo(f)
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	fn := path.Join(ss.dir, fmt.Sprintf("%016x.db", s.raft().Metadata.Index))
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
