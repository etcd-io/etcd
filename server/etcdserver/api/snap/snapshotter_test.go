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

package snap

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3/raftpb"
)

var testSnap = &raftpb.Snapshot{
	Data: []byte("some snapshot"),
	Metadata: raftpb.SnapshotMetadata{
		ConfState: raftpb.ConfState{
			Voters: []uint64{1, 2, 3},
		},
		Index: 1,
		Term:  1,
	},
}

func TestSaveAndLoad(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	require.NoError(t, err)

	g, err := ss.Load()
	require.NoErrorf(t, err, "err = %v, want nil", err)
	assert.Truef(t, reflect.DeepEqual(g, testSnap), "snap = %#v, want %#v", g, testSnap)
}

func TestBadCRC(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	require.NoError(t, err)
	defer func() { crcTable = crc32.MakeTable(crc32.Castagnoli) }()
	// switch to use another crc table
	// fake a crc mismatch
	crcTable = crc32.MakeTable(crc32.Koopman)

	_, err = Read(zaptest.NewLogger(t), filepath.Join(dir, fmt.Sprintf("%016x-%016x.snap", 1, 1)))
	assert.ErrorIsf(t, err, ErrCRCMismatch, "err = %v, want %v", err, ErrCRCMismatch)
}

func TestFailback(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	large := fmt.Sprintf("%016x-%016x-%016x.snap", 0xFFFF, 0xFFFF, 0xFFFF)
	err = os.WriteFile(filepath.Join(dir, large), []byte("bad data"), 0o666)
	require.NoError(t, err)

	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	require.NoError(t, err)

	g, err := ss.Load()
	require.NoErrorf(t, err, "err = %v, want nil", err)
	assert.Truef(t, reflect.DeepEqual(g, testSnap), "snap = %#v, want %#v", g, testSnap)
	f, err := os.Open(filepath.Join(dir, large) + ".broken")
	require.NoErrorf(t, err, "broken snapshot does not exist")
	f.Close()
}

func TestSnapNames(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	for i := 1; i <= 5; i++ {
		var f *os.File
		f, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.snap", i)))
		require.NoError(t, err)
		f.Close()
	}
	ss := New(zaptest.NewLogger(t), dir)
	names, err := ss.snapNames()
	require.NoErrorf(t, err, "err = %v, want nil", err)
	assert.Lenf(t, names, 5, "len = %d, want 10", len(names))
	w := []string{"5.snap", "4.snap", "3.snap", "2.snap", "1.snap"}
	assert.Truef(t, reflect.DeepEqual(names, w), "names = %v, want %v", names, w)
}

func TestLoadNewestSnap(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	require.NoError(t, err)

	newSnap := *testSnap
	newSnap.Metadata.Index = 5
	err = ss.save(&newSnap)
	require.NoError(t, err)

	cases := []struct {
		name              string
		availableWALSnaps []walpb.Snapshot
		expected          *raftpb.Snapshot
	}{
		{
			name:     "load-newest",
			expected: &newSnap,
		},
		{
			name:              "loadnewestavailable-newest",
			availableWALSnaps: []walpb.Snapshot{{Index: 0, Term: 0}, {Index: 1, Term: 1}, {Index: 5, Term: 1}},
			expected:          &newSnap,
		},
		{
			name:              "loadnewestavailable-newest-unsorted",
			availableWALSnaps: []walpb.Snapshot{{Index: 5, Term: 1}, {Index: 1, Term: 1}, {Index: 0, Term: 0}},
			expected:          &newSnap,
		},
		{
			name:              "loadnewestavailable-previous",
			availableWALSnaps: []walpb.Snapshot{{Index: 0, Term: 0}, {Index: 1, Term: 1}},
			expected:          testSnap,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			var g *raftpb.Snapshot
			if tc.availableWALSnaps != nil {
				g, err = ss.LoadNewestAvailable(tc.availableWALSnaps)
			} else {
				g, err = ss.Load()
			}
			require.NoErrorf(t, err, "err = %v, want nil", err)
			assert.Truef(t, reflect.DeepEqual(g, tc.expected), "snap = %#v, want %#v", g, tc.expected)
		})
	}
}

func TestNoSnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	_, err = ss.Load()
	assert.ErrorIsf(t, err, ErrNoSnapshot, "err = %v, want %v", err, ErrNoSnapshot)
}

func TestEmptySnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	err = os.WriteFile(filepath.Join(dir, "1.snap"), []byte(""), 0x700)
	require.NoError(t, err)

	_, err = Read(zaptest.NewLogger(t), filepath.Join(dir, "1.snap"))
	assert.ErrorIsf(t, err, ErrEmptySnapshot, "err = %v, want %v", err, ErrEmptySnapshot)
}

// TestAllSnapshotBroken ensures snapshotter returns
// ErrNoSnapshot if all the snapshots are broken.
func TestAllSnapshotBroken(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	err = os.WriteFile(filepath.Join(dir, "1.snap"), []byte("bad"), 0x700)
	require.NoError(t, err)

	ss := New(zaptest.NewLogger(t), dir)
	_, err = ss.Load()
	assert.ErrorIsf(t, err, ErrNoSnapshot, "err = %v, want %v", err, ErrNoSnapshot)
}

func TestReleaseSnapDBs(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	snapIndices := []uint64{100, 200, 300, 400}
	for _, index := range snapIndices {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		require.NoError(t, os.WriteFile(filename, []byte("snap file\n"), 0o644))
	}

	ss := New(zaptest.NewLogger(t), dir)

	err = ss.ReleaseSnapDBs(raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 300}})
	require.NoError(t, err)

	deleted := []uint64{100, 200}
	for _, index := range deleted {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		assert.Falsef(t, fileutil.Exist(filename), "expected %s (index: %d)  to be deleted, but it still exists", filename, index)
	}

	retained := []uint64{300, 400}
	for _, index := range retained {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		assert.Truef(t, fileutil.Exist(filename), "expected %s (index: %d) to be retained, but it no longer exists", filename, index)
	}
}
