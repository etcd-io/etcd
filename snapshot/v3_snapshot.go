// Copyright 2018 The etcd Authors
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

package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/etcdserver/membership"
	"github.com/coreos/etcd/internal/lease"
	"github.com/coreos/etcd/internal/mvcc"
	"github.com/coreos/etcd/internal/mvcc/backend"
	"github.com/coreos/etcd/internal/raftsnap"
	"github.com/coreos/etcd/internal/store"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/logger"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"

	bolt "github.com/coreos/bbolt"
)

// Manager defines snapshot methods.
type Manager interface {
	// Save fetches snapshot from remote etcd server and saves data to target path.
	// If the context "ctx" is canceled or timed out, snapshot save stream will error out
	// (e.g. context.Canceled, context.DeadlineExceeded).
	Save(ctx context.Context, dbPath string) error

	// Status returns the snapshot file information.
	Status(dbPath string) (Status, error)

	// Restore restores a new etcd data directory from given snapshot file.
	// It returns an error if specified data directory already exists, to
	// prevent unintended data directory overwrites.
	Restore(dbPath string, cfg RestoreConfig) error
}

// Status is the snapshot file status.
type Status struct {
	Hash      uint32 `json:"hash"`
	Revision  int64  `json:"revision"`
	TotalKey  int    `json:"totalKey"`
	TotalSize int64  `json:"totalSize"`
}

// RestoreConfig configures snapshot restore operation.
type RestoreConfig struct {
	// Name is the human-readable name of this member.
	Name string
	// OutputDataDir is the target data directory to save restored data.
	// OutputDataDir should not conflict with existing etcd data directory.
	// If OutputDataDir already exists, it will return an error to prevent
	// unintended data directory overwrites.
	// Defaults to "[Name].etcd" if not given.
	OutputDataDir string
	// OutputWALDir is the target WAL data directory.
	// Defaults to "[OutputDataDir]/member/wal" if not given.
	OutputWALDir string
	// InitialCluster is the initial cluster configuration for restore bootstrap.
	InitialCluster types.URLsMap
	// InitialClusterToken is the initial cluster token for etcd cluster during restore bootstrap.
	InitialClusterToken string
	// PeerURLs is a list of member's peer URLs to advertise to the rest of the cluster.
	PeerURLs types.URLs
	// SkipHashCheck is "true" to ignore snapshot integrity hash value
	// (required if copied from data directory).
	SkipHashCheck bool
}

// NewV3 returns a new snapshot Manager for v3.x snapshot.
// "*clientv3.Client" is only used for "Save" method.
// Otherwise, pass "nil".
func NewV3(cli *clientv3.Client, lg logger.Logger) Manager {
	if lg == nil {
		lg = logger.NewDiscardLogger()
	}
	return &v3Manager{cli: cli, logger: lg}
}

type v3Manager struct {
	cli *clientv3.Client

	name    string
	dbPath  string
	walDir  string
	snapDir string
	cl      *membership.RaftCluster

	skipHashCheck bool
	logger        logger.Logger
}

func (s *v3Manager) Save(ctx context.Context, dbPath string) error {
	partpath := dbPath + ".part"
	f, err := os.Create(partpath)
	if err != nil {
		os.RemoveAll(partpath)
		return fmt.Errorf("could not open %s (%v)", partpath, err)
	}
	s.logger.Infof("created temporary db file %q", partpath)

	var rd io.ReadCloser
	rd, err = s.cli.Snapshot(ctx)
	if err != nil {
		os.RemoveAll(partpath)
		return err
	}
	s.logger.Infof("copying from snapshot stream")
	if _, err = io.Copy(f, rd); err != nil {
		os.RemoveAll(partpath)
		return err
	}
	if err = fileutil.Fsync(f); err != nil {
		os.RemoveAll(partpath)
		return err
	}
	if err = f.Close(); err != nil {
		os.RemoveAll(partpath)
		return err
	}

	s.logger.Infof("renaming from %q to %q", partpath, dbPath)
	if err = os.Rename(partpath, dbPath); err != nil {
		os.RemoveAll(partpath)
		return fmt.Errorf("could not rename %s to %s (%v)", partpath, dbPath, err)
	}
	return nil
}

func (s *v3Manager) Status(dbPath string) (ds Status, err error) {
	if _, err = os.Stat(dbPath); err != nil {
		return ds, err
	}

	db, err := bolt.Open(dbPath, 0400, &bolt.Options{ReadOnly: true})
	if err != nil {
		return ds, err
	}
	defer db.Close()

	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))

	if err = db.View(func(tx *bolt.Tx) error {
		ds.TotalSize = tx.Size()
		c := tx.Cursor()
		for next, _ := c.First(); next != nil; next, _ = c.Next() {
			b := tx.Bucket(next)
			if b == nil {
				return fmt.Errorf("cannot get hash of bucket %s", string(next))
			}
			h.Write(next)
			iskeyb := (string(next) == "key")
			b.ForEach(func(k, v []byte) error {
				h.Write(k)
				h.Write(v)
				if iskeyb {
					rev := bytesToRev(k)
					ds.Revision = rev.main
				}
				ds.TotalKey++
				return nil
			})
		}
		return nil
	}); err != nil {
		return ds, err
	}

	ds.Hash = h.Sum32()
	return ds, nil
}

func (s *v3Manager) Restore(dbPath string, cfg RestoreConfig) error {
	srv := etcdserver.ServerConfig{
		Name:                cfg.Name,
		InitialClusterToken: cfg.InitialClusterToken,
		InitialPeerURLsMap:  cfg.InitialCluster,
		PeerURLs:            cfg.PeerURLs,
	}
	if err := srv.VerifyBootstrap(); err != nil {
		return err
	}

	var err error
	s.cl, err = membership.NewClusterFromURLsMap(cfg.InitialClusterToken, cfg.InitialCluster)
	if err != nil {
		return err
	}

	dataDir := cfg.OutputDataDir
	if dataDir == "" {
		dataDir = cfg.Name + ".etcd"
	}
	if _, err = os.Stat(dataDir); err == nil {
		return fmt.Errorf("data-dir %q exists", dataDir)
	}
	walDir := cfg.OutputWALDir
	if walDir == "" {
		walDir = filepath.Join(dataDir, "member", "wal")
	} else if _, err = os.Stat(walDir); err == nil {
		return fmt.Errorf("wal-dir %q exists", walDir)
	}
	s.logger.Infof("restoring snapshot file %q to data-dir %q, wal-dir %q", dbPath, dataDir, walDir)

	s.name = cfg.Name
	s.dbPath = dbPath
	s.walDir = walDir
	s.snapDir = filepath.Join(dataDir, "member", "snap")
	s.skipHashCheck = cfg.SkipHashCheck

	s.logger.Infof("writing snapshot directory %q", s.snapDir)
	if err = s.saveDB(); err != nil {
		return err
	}
	s.logger.Infof("writing WAL directory %q and raft snapshot to %q", s.walDir, s.snapDir)
	err = s.saveWALAndSnap()
	if err == nil {
		s.logger.Infof("finished restore %q to data directory %q, wal directory %q", dbPath, dataDir, walDir)
	}
	return err
}

// saveDB copies the database snapshot to the snapshot directory
func (s *v3Manager) saveDB() error {
	f, ferr := os.OpenFile(s.dbPath, os.O_RDONLY, 0600)
	if ferr != nil {
		return ferr
	}
	defer f.Close()

	// get snapshot integrity hash
	if _, err := f.Seek(-sha256.Size, io.SeekEnd); err != nil {
		return err
	}
	sha := make([]byte, sha256.Size)
	if _, err := f.Read(sha); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if err := fileutil.CreateDirAll(s.snapDir); err != nil {
		return err
	}

	dbpath := filepath.Join(s.snapDir, "db")
	db, dberr := os.OpenFile(dbpath, os.O_RDWR|os.O_CREATE, 0600)
	if dberr != nil {
		return dberr
	}
	if _, err := io.Copy(db, f); err != nil {
		return err
	}

	// truncate away integrity hash, if any.
	off, serr := db.Seek(0, io.SeekEnd)
	if serr != nil {
		return serr
	}
	hasHash := (off % 512) == sha256.Size
	if hasHash {
		if err := db.Truncate(off - sha256.Size); err != nil {
			return err
		}
	}

	if !hasHash && !s.skipHashCheck {
		return fmt.Errorf("snapshot missing hash but --skip-hash-check=false")
	}

	if hasHash && !s.skipHashCheck {
		// check for match
		if _, err := db.Seek(0, io.SeekStart); err != nil {
			return err
		}
		h := sha256.New()
		if _, err := io.Copy(h, db); err != nil {
			return err
		}
		dbsha := h.Sum(nil)
		if !reflect.DeepEqual(sha, dbsha) {
			return fmt.Errorf("expected sha256 %v, got %v", sha, dbsha)
		}
	}

	// db hash is OK, can now modify DB so it can be part of a new cluster
	db.Close()

	commit := len(s.cl.Members())

	// update consistentIndex so applies go through on etcdserver despite
	// having a new raft instance
	be := backend.NewDefaultBackend(dbpath)

	// a lessor never timeouts leases
	lessor := lease.NewLessor(be, math.MaxInt64)

	mvs := mvcc.NewStore(be, lessor, (*initIndex)(&commit))
	txn := mvs.Write()
	btx := be.BatchTx()
	del := func(k, v []byte) error {
		txn.DeleteRange(k, nil)
		return nil
	}

	// delete stored members from old cluster since using new members
	btx.UnsafeForEach([]byte("members"), del)

	// todo: add back new members when we start to deprecate old snap file.
	btx.UnsafeForEach([]byte("members_removed"), del)

	// trigger write-out of new consistent index
	txn.End()

	mvs.Commit()
	mvs.Close()
	be.Close()

	return nil
}

// saveWALAndSnap creates a WAL for the initial cluster
func (s *v3Manager) saveWALAndSnap() error {
	if err := fileutil.CreateDirAll(s.walDir); err != nil {
		return err
	}

	// add members again to persist them to the store we create.
	st := store.New(etcdserver.StoreClusterPrefix, etcdserver.StoreKeysPrefix)
	s.cl.SetStore(st)
	for _, m := range s.cl.Members() {
		s.cl.AddMember(m)
	}

	m := s.cl.MemberByName(s.name)
	md := &etcdserverpb.Metadata{NodeID: uint64(m.ID), ClusterID: uint64(s.cl.ID())}
	metadata, merr := md.Marshal()
	if merr != nil {
		return merr
	}
	w, walerr := wal.Create(s.walDir, metadata)
	if walerr != nil {
		return walerr
	}
	defer w.Close()

	peers := make([]raft.Peer, len(s.cl.MemberIDs()))
	for i, id := range s.cl.MemberIDs() {
		ctx, err := json.Marshal((*s.cl).Member(id))
		if err != nil {
			return err
		}
		peers[i] = raft.Peer{ID: uint64(id), Context: ctx}
	}

	ents := make([]raftpb.Entry, len(peers))
	nodeIDs := make([]uint64, len(peers))
	for i, p := range peers {
		nodeIDs[i] = p.ID
		cc := raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  p.ID,
			Context: p.Context,
		}
		d, err := cc.Marshal()
		if err != nil {
			return err
		}
		ents[i] = raftpb.Entry{
			Type:  raftpb.EntryConfChange,
			Term:  1,
			Index: uint64(i + 1),
			Data:  d,
		}
	}

	commit, term := uint64(len(ents)), uint64(1)
	if err := w.Save(raftpb.HardState{
		Term:   term,
		Vote:   peers[0].ID,
		Commit: commit,
	}, ents); err != nil {
		return err
	}

	b, berr := st.Save()
	if berr != nil {
		return berr
	}
	raftSnap := raftpb.Snapshot{
		Data: b,
		Metadata: raftpb.SnapshotMetadata{
			Index: commit,
			Term:  term,
			ConfState: raftpb.ConfState{
				Nodes: nodeIDs,
			},
		},
	}
	sn := raftsnap.New(s.snapDir)
	if err := sn.SaveSnap(raftSnap); err != nil {
		return err
	}

	err := w.SaveSnapshot(walpb.Snapshot{Index: commit, Term: term})
	if err == nil {
		s.logger.Infof("wrote WAL snapshot to %q", s.walDir)
	}
	return err
}
