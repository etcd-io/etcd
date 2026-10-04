// Copyright 2026 The etcd Authors
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

package verify

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/raft/v3/raftpb"
)

// writeWALWithoutSurvivingSnapshot creates a WAL directory whose only
// surviving segment(s) hold Entry/HardState records but no Snapshot record.
//
// wal.Create always writes a bootstrap snapshot record into the first
// segment. To simulate a WAL directory that lost the segment holding the
// snapshot record while a later, entry-only segment survives (e.g. an
// incomplete backup/restore, or WAL files pruned beyond etcd's own
// retention), this forces a segment cut and then removes the first segment
// file.
func writeWALWithoutSurvivingSnapshot(t *testing.T, walDir string) {
	t.Helper()
	lg := zaptest.NewLogger(t)

	oldSegmentSizeBytes := wal.SegmentSizeBytes
	wal.SegmentSizeBytes = 1024
	t.Cleanup(func() { wal.SegmentSizeBytes = oldSegmentSizeBytes })

	w, err := wal.Create(lg, walDir, nil)
	require.NoError(t, err)

	term, commit := uint64(1), uint64(1)
	for i := uint64(1); i <= 200; i++ {
		commit = i
		ent := raftpb.Entry{Term: &term, Index: &i, Data: make([]byte, 64)}
		hs := raftpb.HardState{Term: &term, Commit: &commit}
		require.NoError(t, w.Save(&hs, []*raftpb.Entry{&ent}))
	}
	require.NoError(t, w.Close())

	entries, err := os.ReadDir(walDir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	require.Greater(t, len(names), 1, "expected the WAL to have cut into multiple segments")

	// The lowest-named segment is the one holding the bootstrap snapshot
	// record; removing it leaves only entry-only segments behind.
	require.NoError(t, os.Remove(filepath.Join(walDir, names[0])))
}

func TestValidateWALReturnsErrorWithoutPanicWhenNoSnapshotEntriesSurvive(t *testing.T) {
	dataDir := t.TempDir()
	walDir := datadir.ToWALDir(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(walDir), 0o700))

	writeWALWithoutSurvivingSnapshot(t, walDir)

	snaps, err := wal.ValidSnapshotEntries(zaptest.NewLogger(t), walDir)
	require.NoError(t, err)
	require.Empty(t, snaps, "test setup should produce a WAL with no surviving snapshot entries")

	require.NotPanics(t, func() {
		_, _, err = validateWAL(Config{DataDir: dataDir, Logger: zaptest.NewLogger(t)})
	})
	require.Error(t, err)
}

func TestVerifyReturnsErrorWithoutPanicWhenNoSnapshotEntriesSurvive(t *testing.T) {
	dataDir := t.TempDir()
	walDir := datadir.ToWALDir(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(walDir), 0o700))

	writeWALWithoutSurvivingSnapshot(t, walDir)

	dbPath := datadir.ToBackendFileName(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o700))
	require.NoError(t, os.WriteFile(dbPath, []byte{}, 0o600))

	var err error
	require.NotPanics(t, func() {
		err = Verify(Config{DataDir: dataDir, Logger: zaptest.NewLogger(t)})
	})
	require.Error(t, err)
}
