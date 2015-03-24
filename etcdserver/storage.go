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
	"log"
	"os"
	"path"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/migrate"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/version"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
)

type Storage interface {
	// Save function saves ents and state to the underlying stable storage.
	// Save MUST block until st and ents are on stable storage.
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	// SaveSnap function saves snapshot to the underlying stable storage.
	SaveSnap(snap raftpb.Snapshot) error
	// Close closes the Storage and performs finalization.
	Close() error
}

type storage struct {
	*wal.WAL
	*snap.Snapshotter
}

func NewStorage(w *wal.WAL, s *snap.Snapshotter) Storage {
	return &storage{w, s}
}

// SaveSnap saves the snapshot to disk and release the locked
// wal files since they will not be used.
func (st *storage) SaveSnap(snap raftpb.Snapshot) error {
	err := st.Snapshotter.SaveSnap(snap)
	if err != nil {
		return err
	}
	walsnap := walpb.Snapshot{
		Index: snap.Metadata.Index,
		Term:  snap.Metadata.Term,
	}
	err = st.WAL.SaveSnapshot(walsnap)
	if err != nil {
		return err
	}
	err = st.WAL.ReleaseLockTo(snap.Metadata.Index)
	if err != nil {
		return err
	}
	return nil
}

func readWAL(waldir string, snap walpb.Snapshot) (w *wal.WAL, id, cid types.ID, st raftpb.HardState, ents []raftpb.Entry) {
	var err error
	if w, err = wal.Open(waldir, snap); err != nil {
		log.Fatalf("etcdserver: open wal error: %v", err)
	}
	var wmetadata []byte
	if wmetadata, st, ents, err = w.ReadAll(); err != nil {
		log.Fatalf("etcdserver: read wal error: %v", err)
	}
	var metadata pb.Metadata
	pbutil.MustUnmarshal(&metadata, wmetadata)
	id = types.ID(metadata.NodeID)
	cid = types.ID(metadata.ClusterID)
	return
}

// upgradeWAL converts an older version of the etcdServer data to the newest version.
// It must ensure that, after upgrading, the most recent version is present.
func upgradeDataDir(baseDataDir string, name string, ver version.DataDirVersion) error {
	switch ver {
	case version.DataDir0_4:
		log.Print("etcdserver: converting v0.4 log to v2.0")
		err := migrate.Migrate4To2(baseDataDir, name)
		if err != nil {
			log.Fatalf("etcdserver: failed migrating data-dir: %v", err)
			return err
		}
		fallthrough
	case version.DataDir2_0:
		err := makeMemberDir(baseDataDir)
		if err != nil {
			return err
		}
		fallthrough
	case version.DataDir2_0_1:
		fallthrough
	default:
		log.Printf("etcdserver: datadir is valid for the 2.0.1 format")
	}
	return nil
}

func makeMemberDir(dir string) error {
	membdir := path.Join(dir, "member")
	_, err := os.Stat(membdir)
	switch {
	case err == nil:
		return nil
	case !os.IsNotExist(err):
		return err
	}
	if err := os.MkdirAll(membdir, 0700); err != nil {
		return err
	}
	names := []string{"snap", "wal"}
	for _, name := range names {
		if err := os.Rename(path.Join(dir, name), path.Join(membdir, name)); err != nil {
			return err
		}
	}
	return nil
}
