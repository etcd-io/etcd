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
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3/raftpb"
)

var testSnap = &raftpb.Snapshot{
	Data: []byte("some snapshot"),
	Metadata: &raftpb.SnapshotMetadata{
		ConfState: &raftpb.ConfState{
			Voters: []uint64{1, 2, 3},
		},
		Index: new(uint64(1)),
		Term:  new(uint64(1)),
	},
}

func TestSaveAndLoad(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	g, err := ss.Load()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if !proto.Equal(g, testSnap) {
		t.Errorf("snap = %#v, want %#v", g, testSnap)
	}
}

func TestBadCRC(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { crcTable = crc32.MakeTable(crc32.Castagnoli) }()
	// switch to use another crc table
	// fake a crc mismatch
	crcTable = crc32.MakeTable(crc32.Koopman)

	_, err = Read(zaptest.NewLogger(t), filepath.Join(dir, fmt.Sprintf("%016x-%016x.snap", 1, 1)))
	if err == nil || !errors.Is(err, ErrCRCMismatch) {
		t.Errorf("err = %v, want %v", err, ErrCRCMismatch)
	}
}

func TestFailback(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	large := fmt.Sprintf("%016x-%016x-%016x.snap", 0xFFFF, 0xFFFF, 0xFFFF)
	err = os.WriteFile(filepath.Join(dir, large), []byte("bad data"), 0o666)
	if err != nil {
		t.Fatal(err)
	}

	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	g, err := ss.Load()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if !proto.Equal(g, testSnap) {
		t.Errorf("snap = %#v, want %#v", g, testSnap)
	}
	if f, err := os.Open(filepath.Join(dir, large) + ".broken"); err != nil {
		t.Fatal("broken snapshot does not exist")
	} else {
		f.Close()
	}
}

func TestSnapNames(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	for i := 1; i <= 5; i++ {
		var f *os.File
		if f, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.snap", i))); err != nil {
			t.Fatal(err)
		} else {
			f.Close()
		}
	}
	ss := New(zaptest.NewLogger(t), dir)
	names, err := ss.snapNames()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if len(names) != 5 {
		t.Errorf("len = %d, want 10", len(names))
	}
	w := []string{"5.snap", "4.snap", "3.snap", "2.snap", "1.snap"}
	if !reflect.DeepEqual(names, w) {
		t.Errorf("names = %v, want %v", names, w)
	}
}

func TestLoadNewestSnap(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	newSnap := proto.Clone(testSnap).(*raftpb.Snapshot)
	newSnap.Metadata.Index = new(uint64(5))
	err = ss.save(newSnap)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name              string
		availableWALSnaps []*walpb.Snapshot
		expected          *raftpb.Snapshot
	}{
		{
			name:     "load-newest",
			expected: newSnap,
		},
		{
			name:              "loadnewestavailable-newest",
			availableWALSnaps: []*walpb.Snapshot{{Index: new(uint64(0)), Term: new(uint64(0))}, {Index: new(uint64(1)), Term: new(uint64(1))}, {Index: new(uint64(5)), Term: new(uint64(1))}},
			expected:          newSnap,
		},
		{
			name:              "loadnewestavailable-newest-unsorted",
			availableWALSnaps: []*walpb.Snapshot{{Index: new(uint64(5)), Term: new(uint64(1))}, {Index: new(uint64(1)), Term: new(uint64(1))}, {Index: new(uint64(0)), Term: new(uint64(0))}},
			expected:          newSnap,
		},
		{
			name:              "loadnewestavailable-previous",
			availableWALSnaps: []*walpb.Snapshot{{Index: new(uint64(0)), Term: new(uint64(0))}, {Index: new(uint64(1)), Term: new(uint64(1))}},
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
			if err != nil {
				t.Errorf("err = %v, want nil", err)
			}
			if !proto.Equal(g, tc.expected) {
				t.Errorf("snap = %#v, want %#v", g, tc.expected)
			}
		})
	}
}

func TestNoSnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zaptest.NewLogger(t), dir)
	_, err = ss.Load()
	if !errors.Is(err, ErrNoSnapshot) {
		t.Errorf("err = %v, want %v", err, ErrNoSnapshot)
	}
}

func TestEmptySnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = os.WriteFile(filepath.Join(dir, "1.snap"), []byte(""), 0x700)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Read(zaptest.NewLogger(t), filepath.Join(dir, "1.snap"))
	if !errors.Is(err, ErrEmptySnapshot) {
		t.Errorf("err = %v, want %v", err, ErrEmptySnapshot)
	}
}

// TestAllSnapshotBroken ensures snapshotter returns
// ErrNoSnapshot if all the snapshots are broken.
func TestAllSnapshotBroken(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = os.WriteFile(filepath.Join(dir, "1.snap"), []byte("bad"), 0x700)
	if err != nil {
		t.Fatal(err)
	}

	ss := New(zaptest.NewLogger(t), dir)
	_, err = ss.Load()
	if !errors.Is(err, ErrNoSnapshot) {
		t.Errorf("err = %v, want %v", err, ErrNoSnapshot)
	}
}

func TestReleaseSnapDBs(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	snapIndices := []uint64{100, 200, 300, 400}
	for _, index := range snapIndices {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		if err := os.WriteFile(filename, []byte("snap file\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ss := New(zaptest.NewLogger(t), dir)

	if err := ss.ReleaseSnapDBs(&raftpb.Snapshot{Metadata: &raftpb.SnapshotMetadata{Index: new(uint64(300))}}); err != nil {
		t.Fatal(err)
	}

	deleted := []uint64{100, 200}
	for _, index := range deleted {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		if fileutil.Exist(filename) {
			t.Errorf("expected %s (index: %d)  to be deleted, but it still exists", filename, index)
		}
	}

	retained := []uint64{300, 400}
	for _, index := range retained {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		if !fileutil.Exist(filename) {
			t.Errorf("expected %s (index: %d) to be retained, but it no longer exists", filename, index)
		}
	}
}

// TestReleaseSnapDBsRetainsSnapshotPendingApply reproduces the scenario from
// https://github.com/etcd-io/etcd/issues/18055. A follower saves the database
// file for snapshot A on receipt (rafthttp -> SaveDBFrom) and the snapshot
// waits in the apply queue. Before A is applied, a newer snapshot B arrives
// and the raft loop releases all older database files (ReleaseSnapDBs),
// deleting A's file while its apply is still pending. When the apply loop
// finally opens A's file (OpenSnapshotBackend -> DBFilePath), the file is
// gone and etcdserver panics with "failed to open snapshot backend".
func TestReleaseSnapDBsRetainsSnapshotPendingApply(t *testing.T) {
	dir := t.TempDir()
	ss := New(zaptest.NewLogger(t), dir)

	const (
		indexA uint64 = 100
		indexB uint64 = 200
	)

	// Snapshot A is received and its database file saved.
	if _, err := ss.SaveDBFrom(strings.NewReader("A"), indexA); err != nil {
		t.Fatal(err)
	}

	// Snapshot B arrives before A has been applied; the raft loop releases
	// older snapshot database files.
	if err := ss.ReleaseSnapDBs(&raftpb.Snapshot{Metadata: &raftpb.SnapshotMetadata{Index: new(uint64(indexB))}}); err != nil {
		t.Fatal(err)
	}

	// The apply loop now looks up snapshot A's database file to apply it.
	if _, err := ss.DBFilePath(indexA); err != nil {
		t.Errorf("DBFilePath(%d) = %v, want success: a saved snapshot db must not be released before it has been applied", indexA, err)
	}

	// Once the apply path has consumed the file, the protection is dropped
	// and any leftover file becomes eligible for cleanup again.
	ss.ReleaseDBSnapshot(indexA)
	if err := ss.ReleaseSnapDBs(&raftpb.Snapshot{Metadata: &raftpb.SnapshotMetadata{Index: new(uint64(indexB))}}); err != nil {
		t.Fatal(err)
	}
	if fileutil.Exist(filepath.Join(dir, fmt.Sprintf("%016x.snap.db", indexA))) {
		t.Errorf("expected snapshot db for index %d to be deleted after its apply completed", indexA)
	}
}
