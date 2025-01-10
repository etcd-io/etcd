// Copyright 2021 The etcd Authors
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

package etcdutl

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3/raftpb"
)

func GetLogger() *zap.Logger {
	config := logutil.DefaultZapLoggerConfig
	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	lg, err := config.Build()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}
	return lg
}

func getLatestWALSnap(lg *zap.Logger, dataDir string) (walpb.Snapshot, error) {
	snapshot, err := getLatestV2Snapshot(lg, dataDir)
	if err != nil {
		return walpb.Snapshot{}, err
	}

	var walsnap walpb.Snapshot
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	return walsnap, nil
}

func getLatestV2Snapshot(lg *zap.Logger, dataDir string) (*raftpb.Snapshot, error) {
	walPath := datadir.ToWALDir(dataDir)
	walSnaps, err := wal.ValidSnapshotEntries(lg, walPath)
	if err != nil {
		return nil, err
	}

	ss := snap.New(lg, datadir.ToSnapDir(dataDir))
	snapshot, err := ss.LoadNewestAvailable(walSnaps)
	if err != nil && !errors.Is(err, snap.ErrNoSnapshot) {
		return nil, err
	}

	return snapshot, nil
}

func createV2SnapshotFromV3Store(dataDir string, be backend.Backend) error {
	var (
		lg = GetLogger()

		snapDir = datadir.ToSnapDir(dataDir)
		walDir  = datadir.ToWALDir(dataDir)
	)

	ci, term := schema.ReadConsistentIndex(be.ReadTx())

	cl := membership.NewCluster(lg)
	cl.SetBackend(schema.NewMembershipBackend(lg, be))
	cl.UnsafeLoad()

	latestWALSnap, err := getLatestWALSnap(lg, dataDir)
	if err != nil {
		return err
	}

	// Each time before creating the v2 snapshot, etcdserve always flush
	// the backend storage (bbolt db), so the consistent index should never
	// less than the Index or term of the latest snapshot.
	if ci < latestWALSnap.Index || term < latestWALSnap.Term {
		// This should never happen
		return fmt.Errorf("consistent_index [Index: %d, Term: %d] is less than the latest snapshot [Index: %d, Term: %d]", ci, term, latestWALSnap.Index, latestWALSnap.Term)
	}

	voters, learners := getVotersAndLearners(cl)
	confState := raftpb.ConfState{
		Voters:   voters,
		Learners: learners,
	}

	// create the v2 snaspshot file
	raftSnap := raftpb.Snapshot{
		Data: etcdserver.GetMembershipInfoInV2Format(lg, cl),
		Metadata: raftpb.SnapshotMetadata{
			Index:     ci,
			Term:      term,
			ConfState: confState,
		},
	}
	sn := snap.New(lg, snapDir)
	if err = sn.SaveSnap(raftSnap); err != nil {
		return err
	}

	// save WAL snapshot record
	w, err := wal.Open(lg, walDir, latestWALSnap)
	if err != nil {
		return err
	}
	defer w.Close()
	// We must read all records to locate the tail of the last valid WAL file.
	_, st, _, err := w.ReadAll()
	if err != nil {
		return err
	}

	if err := w.SaveSnapshot(walpb.Snapshot{Index: ci, Term: term, ConfState: &confState}); err != nil {
		return err
	}

	if term >= st.Term && ci > st.Commit {
		if err := w.Save(raftpb.HardState{Term: term, Commit: ci, Vote: st.Vote}, nil); err != nil {
			return err
		}
	}
	return w.Sync()
}

func getVotersAndLearners(cl *membership.RaftCluster) ([]uint64, []uint64) {
	var (
		voters   []uint64
		learners []uint64
	)
	for _, m := range cl.Members() {
		if m.IsLearner {
			learners = append(learners, uint64(m.ID))
			continue
		}

		voters = append(voters, uint64(m.ID))
	}

	return voters, learners
}
