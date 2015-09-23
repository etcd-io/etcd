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
	"github.com/coreos/etcd/raft/raftpb"
)

type snapshot struct {
	// storing the raft snapshot. the data in raftsnap is v2 store.
	raftsnap raftpb.Snapshot
	// rc is a reader that provides the data of KV snapshot.
	rc io.ReadCloser
	// dataSize specifies the size of snapshot data.
	dataSize int64
}

type snapshotHub struct {
	// dir to save snapshot data
	dir string

	// send empty to reqsnapc to notify the channel receiver to send back latest
	// snapshot to snapc
	reqsnapc chan struct{}
	// a chan to receive the requested snapshot
	// hub will receive from the chan immediately after it sends empty to reqsnapc
	snapc chan snapshot
	// host received snapshots
	snaps []snapshot
}

func newSnapshotHub(dir string) *snapshotHub {
	return &snapshotHub{
		dir:      dir,
		reqsnapc: make(chan struct{}),
		snapc:    make(chan snapshot),
	}
}

// CreateSnapshot creates snapshot with latest index. The returned snapshot
// is the mark of snapshot, and caller could call SnapshotData to get its data.
// TODO: the function should be unblock and finish in 1ms. It may use ~20ms
// in the peak for 2 million keys.
func (sh *snapshotHub) CreateSnapshot() raftpb.Snapshot {
	sh.reqsnapc <- struct{}{}
	snap := <-sh.snapc
	sh.snaps = append(sh.snaps, snap)
	return snap.raftsnap
}

// SnapshotData returns snapshot data that matches the given raft snapshot.
// The given raft snapshot must be from returned snapshot. If not, it panics.
// SnapshotData should be called once and only once for each Snapshot call, because
// SnapshotData can only be read once and occupies resources if not used.
func (sh *snapshotHub) SnapshotData(raftsnap raftpb.Snapshot) (io.ReadCloser, int64) {
	for i, s := range sh.snaps {
		if s.raftsnap.Metadata.Index == raftsnap.Metadata.Index {
			sh.snaps = append(sh.snaps[:i], sh.snaps[i+1:]...)
			return s.rc, s.dataSize
		}
	}
	plog.Panicf("cannot get snapshot of %+v", raftsnap)
	return nil, 0
}

// SaveSnapshotData saves snapshot data into disk. If snapshot data of the same
// snapshot mark has existed, it returns error and keeps the original snapshot
// data. The function guarantees that if the call is aborted at any time,
// it leaves either complete snapshot data or nothing.
func (sh *snapshotHub) SaveSnapshotData(raftsnap raftpb.Snapshot, r io.Reader) error {
	f, err := ioutil.TempFile(sh.dir, "tmp")
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	f.Close()
	if err != nil {
		return err
	}
	fn := sh.SavedSnapshotFilename(raftsnap)
	if fileutil.Exist(fn) {
		return fmt.Errorf("snapshot to save has existed")
	}
	return os.Rename(f.Name(), fn)
}

// SavedSnapshotFilename returns the filename of saved snapshot.
func (sh *snapshotHub) SavedSnapshotFilename(raftsnap raftpb.Snapshot) string {
	return path.Join(sh.dir, fmt.Sprintf("%016x.db", raftsnap.Metadata.Index))
}
