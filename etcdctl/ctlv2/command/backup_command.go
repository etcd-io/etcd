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
	"encoding/binary"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/etcdserver/membership"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/idutil"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"

	bolt "github.com/coreos/bbolt"
	"github.com/urfave/cli"
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

// handleBackup handles a request that intends to do a backup.
func handleBackup(c *cli.Context) error {
	var srcWAL string
	var destWAL string

	withV3 := c.Bool("with-v3")
	srcSnap := filepath.Join(c.String("data-dir"), "member", "snap")
	destSnap := filepath.Join(c.String("backup-dir"), "member", "snap")

	if c.String("wal-dir") != "" {
		srcWAL = c.String("wal-dir")
	} else {
		srcWAL = filepath.Join(c.String("data-dir"), "member", "wal")
	}

	if c.String("backup-wal-dir") != "" {
		destWAL = c.String("backup-wal-dir")
	} else {
		destWAL = filepath.Join(c.String("backup-dir"), "member", "wal")
	}

	if err := fileutil.CreateDirAll(destSnap); err != nil {
		log.Fatalf("failed creating backup snapshot dir %v: %v", destSnap, err)
	}

	walsnap := saveSnap(destSnap, srcSnap)
	metadata, state, ents := loadWAL(srcWAL, walsnap, withV3)
	saveDB(filepath.Join(destSnap, "db"), filepath.Join(srcSnap, "db"), state.Commit, withV3)

	idgen := idutil.NewGenerator(0, time.Now())
	metadata.NodeID = idgen.Next()
	metadata.ClusterID = idgen.Next()

	neww, err := wal.Create(destWAL, pbutil.MustMarshal(&metadata))
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

func saveSnap(destSnap, srcSnap string) (walsnap walpb.Snapshot) {
	ss := snap.New(srcSnap)
	snapshot, err := ss.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		log.Fatal(err)
	}
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
		newss := snap.New(destSnap)
		if err = newss.SaveSnap(*snapshot); err != nil {
			log.Fatal(err)
		}
	}
	return walsnap
}

func loadWAL(srcWAL string, walsnap walpb.Snapshot, v3 bool) (etcdserverpb.Metadata, raftpb.HardState, []raftpb.Entry) {
	w, err := wal.OpenForRead(srcWAL, walsnap)
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
func saveDB(destDB, srcDB string, idx uint64, v3 bool) {
	// open src db to safely copy db state
	if v3 {
		var src *bolt.DB
		ch := make(chan *bolt.DB, 1)
		go func() {
			src, err := bolt.Open(srcDB, 0444, &bolt.Options{ReadOnly: true})
			if err != nil {
				log.Fatal(err)
			}
			ch <- src
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

	db, err := bolt.Open(destDB, 0644, &bolt.Options{})
	if err != nil {
		log.Fatal(err)
	}
	tx, err := db.Begin(true)
	if err != nil {
		log.Fatal(err)
	}

	// remove membership information; should be clobbered by --force-new-cluster
	for _, bucket := range []string{"members", "members_removed", "cluster"} {
		tx.DeleteBucket([]byte(bucket))
	}

	// update consistent index to match hard state
	if !v3 {
		idxBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(idxBytes, idx)
		b, err := tx.CreateBucketIfNotExists([]byte("meta"))
		if err != nil {
			log.Fatal(err)
		}
		b.Put([]byte("consistent_index"), idxBytes)
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
}
