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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
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
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zap.NewExample(), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	g, err := ss.Load()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(g, testSnap) {
		t.Errorf("snap = %#v, want %#v", g, testSnap)
	}
}

func TestBadCRC(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zap.NewExample(), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { crcTable = crc32.MakeTable(crc32.Castagnoli) }()
	// switch to use another crc table
	// fake a crc mismatch
	crcTable = crc32.MakeTable(crc32.Koopman)

	_, err = Read(zap.NewExample(), filepath.Join(dir, fmt.Sprintf("%016x-%016x.snap", 1, 1)))
	if err == nil || err != ErrCRCMismatch {
		t.Errorf("err = %v, want %v", err, ErrCRCMismatch)
	}
}

func TestFailback(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	large := fmt.Sprintf("%016x-%016x-%016x.snap", 0xFFFF, 0xFFFF, 0xFFFF)
	err = ioutil.WriteFile(filepath.Join(dir, large), []byte("bad data"), 0666)
	if err != nil {
		t.Fatal(err)
	}

	ss := New(zap.NewExample(), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	g, err := ss.Load()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(g, testSnap) {
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
	err := os.Mkdir(dir, 0700)
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
	ss := New(zap.NewExample(), dir)
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
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zap.NewExample(), dir)
	err = ss.save(testSnap)
	if err != nil {
		t.Fatal(err)
	}

	newSnap := *testSnap
	newSnap.Metadata.Index = 5
	err = ss.save(&newSnap)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name              string
		availableWalSnaps []walpb.Snapshot
		expected          *raftpb.Snapshot
	}{
		{
			name:     "load-newest",
			expected: &newSnap,
		},
		{
			name:              "loadnewestavailable-newest",
			availableWalSnaps: []walpb.Snapshot{{Index: 0, Term: 0}, {Index: 1, Term: 1}, {Index: 5, Term: 1}},
			expected:          &newSnap,
		},
		{
			name:              "loadnewestavailable-newest-unsorted",
			availableWalSnaps: []walpb.Snapshot{{Index: 5, Term: 1}, {Index: 1, Term: 1}, {Index: 0, Term: 0}},
			expected:          &newSnap,
		},
		{
			name:              "loadnewestavailable-previous",
			availableWalSnaps: []walpb.Snapshot{{Index: 0, Term: 0}, {Index: 1, Term: 1}},
			expected:          testSnap,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			var g *raftpb.Snapshot
			if tc.availableWalSnaps != nil {
				g, err = ss.LoadNewestAvailable(tc.availableWalSnaps)
			} else {
				g, err = ss.Load()
			}
			if err != nil {
				t.Errorf("err = %v, want nil", err)
			}
			if !reflect.DeepEqual(g, tc.expected) {
				t.Errorf("snap = %#v, want %#v", g, tc.expected)
			}
		})
	}
}

func TestNoSnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ss := New(zap.NewExample(), dir)
	_, err = ss.Load()
	if err != ErrNoSnapshot {
		t.Errorf("err = %v, want %v", err, ErrNoSnapshot)
	}
}

func TestEmptySnapshot(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = ioutil.WriteFile(filepath.Join(dir, "1.snap"), []byte(""), 0x700)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Read(zap.NewExample(), filepath.Join(dir, "1.snap"))
	if err != ErrEmptySnapshot {
		t.Errorf("err = %v, want %v", err, ErrEmptySnapshot)
	}
}

// TestAllSnapshotBroken ensures snapshotter returns
// ErrNoSnapshot if all the snapshots are broken.
func TestAllSnapshotBroken(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = ioutil.WriteFile(filepath.Join(dir, "1.snap"), []byte("bad"), 0x700)
	if err != nil {
		t.Fatal(err)
	}

	ss := New(zap.NewExample(), dir)
	_, err = ss.Load()
	if err != ErrNoSnapshot {
		t.Errorf("err = %v, want %v", err, ErrNoSnapshot)
	}
}

func TestReleaseSnapDBs(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	snapIndices := []uint64{100, 200, 300, 400}
	for _, index := range snapIndices {
		filename := filepath.Join(dir, fmt.Sprintf("%016x.snap.db", index))
		if err := ioutil.WriteFile(filename, []byte("snap file\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ss := New(zap.NewExample(), dir)

	if err := ss.ReleaseSnapDBs(raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 300}}); err != nil {
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
