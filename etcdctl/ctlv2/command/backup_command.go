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

package command

import (
	"log"
	"os"
	"path"
	"regexp"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"

	"github.com/urfave/cli"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

func NewBackupCommand() cli.Command {
	return cli.Command{
		Name:      "backup",
		Usage:     "backup an etcd directory",
		ArgsUsage: " ",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "data-dir", Value: "", Usage: "Path to the etcd data dir"},
			cli.StringFlag{Name: "wal-dir", Value: "", Usage: "Path to the etcd wal dir"},
			cli.StringFlag{Name: "backup-dir", Value: "", Usage: "Path to the backup dir"},
			cli.StringFlag{Name: "backup-wal-dir", Value: "", Usage: "Path to the backup wal dir"},
			cli.BoolFlag{Name: "with-v3", Usage: "Backup v3 backend data"},
		},
		Action: handleBackup,
	}
}

type desiredCluster struct {
	clusterId types.ID
	nodeId    types.ID
	members   []*membership.Member
	confState raftpb.ConfState
}

func newDesiredCluster() desiredCluster {
	idgen := idutil.NewGenerator(0, time.Now())
	nodeID := idgen.Next()
	clusterID := idgen.Next()

	return desiredCluster{
		clusterId: types.ID(clusterID),
		nodeId:    types.ID(nodeID),
		members: []*membership.Member{
			{
				ID: types.ID(nodeID),
				Attributes: membership.Attributes{
					Name: "etcdctl-v2-backup",
				},
				RaftAttributes: membership.RaftAttributes{
					PeerURLs: []string{"http://use-flag--force-new-cluster:2080"},
				}}},
		confState: raftpb.ConfState{Voters: []uint64{nodeID}},
	}
}

// handleBackup handles a request that intends to do a backup.
func handleBackup(c *cli.Context) error {
	var srcWAL string
	var destWAL string

	lg := zap.NewExample()

	withV3 := c.Bool("with-v3")
	srcDir := c.String("data-dir")
	destDir := c.String("backup-dir")

	srcSnap := datadir.ToSnapDir(srcDir)
	destSnap := datadir.ToSnapDir(destDir)

	if c.String("wal-dir") != "" {
		srcWAL = c.String("wal-dir")
	} else {
		srcWAL = datadir.ToWalDir(srcDir)
	}

	if c.String("backup-wal-dir") != "" {
		destWAL = c.String("backup-wal-dir")
	} else {
		destWAL = datadir.ToWalDir(destDir)
	}

	if err := fileutil.CreateDirAll(destSnap); err != nil {
		log.Fatalf("failed creating backup snapshot dir %v: %v", destSnap, err)
	}

	desired := newDesiredCluster()

	walsnap := saveSnap(lg, destSnap, srcSnap, &desired)
	metadata, state, ents := loadWAL(srcWAL, walsnap, withV3)
	destDbPath := datadir.ToBackendFileName(destDir)
	saveDB(lg, destDbPath,  datadir.ToBackendFileName(srcDir), state.Commit, &desired, withV3)

	neww, err := wal.Create(zap.NewExample(), destWAL, pbutil.MustMarshal(&metadata))
	if err != nil {
		log.Fatal(err)
	}
	defer neww.Close()
	if err := neww.Save(state, ents); err != nil {
		log.Fatal(err)
	}
	if err := neww.SaveSnapshot(walsnap); err != nil {
		log.Fatal(err)
	}

	return nil
}

func saveSnap(lg *zap.Logger, destSnap, srcSnap string, desired *desiredCluster) (walsnap walpb.Snapshot) {
	ss := snap.New(lg, srcSnap)
	snapshot, err := ss.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		log.Fatal(err)
	}
	if snapshot != nil {
		walsnap.Index, walsnap.Term, walsnap.ConfState = snapshot.Metadata.Index, snapshot.Metadata.Term, &desired.confState
		newss := snap.New(lg, destSnap)
		snapshot.Metadata.ConfState = desired.confState
		snapshot.Data = mustTranslateV2store(lg, snapshot.Data, desired)
		if err = newss.SaveSnap(*snapshot); err != nil {
			log.Fatal(err)
		}
	}
	return walsnap
}

// mustTranslateV2store processes storeData such that they match 'desiredCluster'.
// In particular the method overrides membership information.
func mustTranslateV2store(lg *zap.Logger, storeData []byte, desired *desiredCluster) []byte {
	st := v2store.New()
	if err := st.Recovery(storeData); err != nil {
		lg.Panic("cannot translate v2store", zap.Error(err))
	}

	raftCluster := membership.NewClusterFromMembers(lg, desired.clusterId, desired.members)
	raftCluster.SetID(desired.nodeId, desired.clusterId)
	raftCluster.SetStore(st)
	raftCluster.PushMembershipToStorage()

	outputData, err := st.Save()
	if err != nil {
		lg.Panic("cannot save v2store", zap.Error(err))
	}
	return outputData
}

func loadWAL(srcWAL string, walsnap walpb.Snapshot, v3 bool) (etcdserverpb.Metadata, raftpb.HardState, []raftpb.Entry) {
	w, err := wal.OpenForRead(zap.NewExample(), srcWAL, walsnap)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()
	wmetadata, state, ents, err := w.ReadAll()
	switch err {
	case nil:
	case wal.ErrSnapshotNotFound:
		log.Printf("Failed to find the match snapshot record %+v in wal %v.", walsnap, srcWAL)
		log.Printf("etcdctl will add it back. Start auto fixing...")
	default:
		log.Fatal(err)
	}

	re := path.Join(membership.StoreMembersPrefix, "[[:xdigit:]]{1,16}", "attributes")
	memberAttrRE := regexp.MustCompile(re)

	removed := uint64(0)
	i := 0
	remove := func() {
		ents = append(ents[:i], ents[i+1:]...)
		removed++
		i--
	}
	for i = 0; i < len(ents); i++ {
		ents[i].Index -= removed
		if ents[i].Type == raftpb.EntryConfChange {
			log.Println("ignoring EntryConfChange raft entry")
			remove()
			continue
		}

		var raftReq etcdserverpb.InternalRaftRequest
		var v2Req *etcdserverpb.Request
		if pbutil.MaybeUnmarshal(&raftReq, ents[i].Data) {
			v2Req = raftReq.V2
		} else {
			v2Req = &etcdserverpb.Request{}
			pbutil.MustUnmarshal(v2Req, ents[i].Data)
		}

		if v2Req != nil && v2Req.Method == "PUT" && memberAttrRE.MatchString(v2Req.Path) {
			log.Println("ignoring member attribute update on", v2Req.Path)
			remove()
			continue
		}

		if v2Req != nil {
			continue
		}

		if raftReq.ClusterMemberAttrSet != nil {
			log.Println("ignoring cluster_member_attr_set")
			remove()
			continue
		}

		if v3 || raftReq.Header == nil {
			continue
		}
		log.Println("ignoring v3 raft entry")
		remove()
	}
	state.Commit -= removed
	var metadata etcdserverpb.Metadata
	pbutil.MustUnmarshal(&metadata, wmetadata)
	return metadata, state, ents
}

// saveDB copies the v3 backend and strips cluster information.
func saveDB(lg *zap.Logger, destDB, srcDB string, idx uint64, desired *desiredCluster, v3 bool) {

	// open src db to safely copy db state
	if v3 {
		var src *bolt.DB
		ch := make(chan *bolt.DB, 1)
		go func() {
			db, err := bolt.Open(srcDB, 0444, &bolt.Options{ReadOnly: true})
			if err != nil {
				log.Fatal(err)
			}
			ch <- db
		}()
		select {
		case src = <-ch:
		case <-time.After(time.Second):
			log.Println("waiting to acquire lock on", srcDB)
			src = <-ch
		}
		defer src.Close()

		tx, err := src.Begin(false)
		if err != nil {
			log.Fatal(err)
		}

		// copy srcDB to destDB
		dest, err := os.Create(destDB)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := tx.WriteTo(dest); err != nil {
			log.Fatal(err)
		}
		dest.Close()
		if err := tx.Rollback(); err != nil {
			log.Fatal(err)
		}
	}

	be := backend.NewDefaultBackend(destDB)
	defer be.Close()

	if err := membership.TrimClusterFromBackend(be); err != nil {
		log.Fatal(err)
	}

	raftCluster := membership.NewClusterFromMembers(lg, desired.clusterId, desired.members)
	raftCluster.SetID(desired.nodeId, desired.clusterId)
	raftCluster.SetBackend(be)
	raftCluster.PushMembershipToStorage()

	if !v3 {
		tx := be.BatchTx()
		tx.Lock()
		defer tx.Unlock()
		tx.UnsafeCreateBucket([]byte("meta"))
		ci := cindex.NewConsistentIndex(tx)
		ci.SetConsistentIndex(idx)
		ci.UnsafeSave(tx)
	}

}
