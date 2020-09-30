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

package wal

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"go.etcd.io/etcd/v3/pkg/fileutil"
	"go.etcd.io/etcd/v3/pkg/pbutil"
	"go.etcd.io/etcd/v3/raft/raftpb"
	"go.etcd.io/etcd/v3/wal/walpb"

	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(zap.NewExample(), p, []byte("somedata"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := filepath.Base(w.tail().Name()); g != walName(0, 0) {
		t.Errorf("name = %+v, want %+v", g, walName(0, 0))
	}
	defer w.Close()

	// file is preallocated to segment size; only read data written by wal
	off, err := w.tail().Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatal(err)
	}
	gd := make([]byte, off)
	f, err := os.Open(filepath.Join(p, filepath.Base(w.tail().Name())))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = io.ReadFull(f, gd); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	var wb bytes.Buffer
	e := newEncoder(&wb, 0, 0)
	err = e.encode(&walpb.Record{Type: crcType, Crc: 0})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	err = e.encode(&walpb.Record{Type: metadataType, Data: []byte("somedata")})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	r := &walpb.Record{
		Type: snapshotType,
		Data: pbutil.MustMarshal(&walpb.Snapshot{}),
	}
	if err = e.encode(r); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	e.flush()
	if !bytes.Equal(gd, wb.Bytes()) {
		t.Errorf("data = %v, want %v", gd, wb.Bytes())
	}
}

func TestCreateFailFromPollutedDir(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	ioutil.WriteFile(filepath.Join(p, "test.wal"), []byte("data"), os.ModeTemporary)

	_, err = Create(zap.NewExample(), p, []byte("data"))
	if err != os.ErrExist {
		t.Fatalf("expected %v, got %v", os.ErrExist, err)
	}
}

func TestWalCleanup(t *testing.T) {
	testRoot, err := ioutil.TempDir(os.TempDir(), "waltestroot")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ioutil.TempDir(testRoot, "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testRoot)

	logger := zap.NewExample()
	w, err := Create(logger, p, []byte(""))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	w.cleanupWAL(logger)
	fnames, err := fileutil.ReadDir(testRoot)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(fnames) != 1 {
		t.Fatalf("expected 1 file under %v, got %v", testRoot, len(fnames))
	}
	pattern := fmt.Sprintf(`%s.broken\.[\d]{8}\.[\d]{6}\.[\d]{1,6}?`, filepath.Base(p))
	match, _ := regexp.MatchString(pattern, fnames[0])
	if !match {
		t.Errorf("match = false, expected true for %v with pattern %v", fnames[0], pattern)
	}
}

func TestCreateFailFromNoSpaceLeft(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	oldSegmentSizeBytes := SegmentSizeBytes
	defer func() {
		SegmentSizeBytes = oldSegmentSizeBytes
	}()
	SegmentSizeBytes = math.MaxInt64

	_, err = Create(zap.NewExample(), p, []byte("data"))
	if err == nil { // no space left on device
		t.Fatalf("expected error 'no space left on device', got nil")
	}
}

func TestNewForInitedDir(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	os.Create(filepath.Join(p, walName(0, 0)))
	if _, err = Create(zap.NewExample(), p, nil); err == nil || err != os.ErrExist {
		t.Errorf("err = %v, want %v", err, os.ErrExist)
	}
}

func TestOpenAtIndex(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	f, err := os.Create(filepath.Join(dir, walName(0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err := Open(zap.NewExample(), dir, walpb.Snapshot{})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := filepath.Base(w.tail().Name()); g != walName(0, 0) {
		t.Errorf("name = %+v, want %+v", g, walName(0, 0))
	}
	if w.seq() != 0 {
		t.Errorf("seq = %d, want %d", w.seq(), 0)
	}
	w.Close()

	wname := walName(2, 10)
	f, err = os.Create(filepath.Join(dir, wname))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err = Open(zap.NewExample(), dir, walpb.Snapshot{Index: 5})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := filepath.Base(w.tail().Name()); g != wname {
		t.Errorf("name = %+v, want %+v", g, wname)
	}
	if w.seq() != 2 {
		t.Errorf("seq = %d, want %d", w.seq(), 2)
	}
	w.Close()

	emptydir, err := ioutil.TempDir(os.TempDir(), "waltestempty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emptydir)
	if _, err = Open(zap.NewExample(), emptydir, walpb.Snapshot{}); err != ErrFileNotFound {
		t.Errorf("err = %v, want %v", err, ErrFileNotFound)
	}
}

// TestVerify tests that Verify throws a non-nil error when the WAL is corrupted.
// The test creates a WAL directory and cuts out multiple WAL files. Then
// it corrupts one of the files by completely truncating it.
func TestVerify(t *testing.T) {
	walDir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(walDir)

	// create WAL
	w, err := Create(zap.NewExample(), walDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// make 5 separate files
	for i := 0; i < 5; i++ {
		es := []raftpb.Entry{{Index: uint64(i), Data: []byte(fmt.Sprintf("waldata%d", i+1))}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
		if err = w.cut(); err != nil {
			t.Fatal(err)
		}
	}

	// to verify the WAL is not corrupted at this point
	err = Verify(zap.NewExample(), walDir, walpb.Snapshot{})
	if err != nil {
		t.Errorf("expected a nil error, got %v", err)
	}

	walFiles, err := ioutil.ReadDir(walDir)
	if err != nil {
		t.Fatal(err)
	}

	// corrupt the WAL by truncating one of the WAL files completely
	err = os.Truncate(path.Join(walDir, walFiles[2].Name()), 0)
	if err != nil {
		t.Fatal(err)
	}

	err = Verify(zap.NewExample(), walDir, walpb.Snapshot{})
	if err == nil {
		t.Error("expected a non-nil error, got nil")
	}
}

// TODO: split it into smaller tests for better readability
func TestCut(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(zap.NewExample(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	state := raftpb.HardState{Term: 1}
	if err = w.Save(state, nil); err != nil {
		t.Fatal(err)
	}
	if err = w.cut(); err != nil {
		t.Fatal(err)
	}
	wname := walName(1, 1)
	if g := filepath.Base(w.tail().Name()); g != wname {
		t.Errorf("name = %s, want %s", g, wname)
	}

	es := []raftpb.Entry{{Index: 1, Term: 1, Data: []byte{1}}}
	if err = w.Save(raftpb.HardState{}, es); err != nil {
		t.Fatal(err)
	}
	if err = w.cut(); err != nil {
		t.Fatal(err)
	}
	snap := walpb.Snapshot{Index: 2, Term: 1}
	if err = w.SaveSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	wname = walName(2, 2)
	if g := filepath.Base(w.tail().Name()); g != wname {
		t.Errorf("name = %s, want %s", g, wname)
	}

	// check the state in the last WAL
	// We do check before closing the WAL to ensure that Cut syncs the data
	// into the disk.
	f, err := os.Open(filepath.Join(p, wname))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	nw := &WAL{
		decoder: newDecoder(f),
		start:   snap,
	}
	_, gst, _, err := nw.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gst, state) {
		t.Errorf("state = %+v, want %+v", gst, state)
	}
}

func TestSaveWithCut(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(zap.NewExample(), p, []byte("metadata"))
	if err != nil {
		t.Fatal(err)
	}

	state := raftpb.HardState{Term: 1}
	if err = w.Save(state, nil); err != nil {
		t.Fatal(err)
	}
	bigData := make([]byte, 500)
	strdata := "Hello World!!"
	copy(bigData, strdata)
	// set a lower value for SegmentSizeBytes, else the test takes too long to complete
	restoreLater := SegmentSizeBytes
	const EntrySize int = 500
	SegmentSizeBytes = 2 * 1024
	defer func() { SegmentSizeBytes = restoreLater }()
	index := uint64(0)
	for totalSize := 0; totalSize < int(SegmentSizeBytes); totalSize += EntrySize {
		ents := []raftpb.Entry{{Index: index, Term: 1, Data: bigData}}
		if err = w.Save(state, ents); err != nil {
			t.Fatal(err)
		}
		index++
	}

	w.Close()

	neww, err := Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer neww.Close()
	wname := walName(1, index)
	if g := filepath.Base(neww.tail().Name()); g != wname {
		t.Errorf("name = %s, want %s", g, wname)
	}

	_, newhardstate, entries, err := neww.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(newhardstate, state) {
		t.Errorf("Hard State = %+v, want %+v", newhardstate, state)
	}
	if len(entries) != int(SegmentSizeBytes/int64(EntrySize)) {
		t.Errorf("Number of entries = %d, expected = %d", len(entries), int(SegmentSizeBytes/int64(EntrySize)))
	}
	for _, oneent := range entries {
		if !bytes.Equal(oneent.Data, bigData) {
			t.Errorf("the saved data does not match at Index %d : found: %s , want :%s", oneent.Index, oneent.Data, bigData)
		}
	}
}

func TestRecover(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(zap.NewExample(), p, []byte("metadata"))
	if err != nil {
		t.Fatal(err)
	}
	if err = w.SaveSnapshot(walpb.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	ents := []raftpb.Entry{{Index: 1, Term: 1, Data: []byte{1}}, {Index: 2, Term: 2, Data: []byte{2}}}
	if err = w.Save(raftpb.HardState{}, ents); err != nil {
		t.Fatal(err)
	}
	sts := []raftpb.HardState{{Term: 1, Vote: 1, Commit: 1}, {Term: 2, Vote: 2, Commit: 2}}
	for _, s := range sts {
		if err = w.Save(s, nil); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	if w, err = Open(zap.NewExample(), p, walpb.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	metadata, state, entries, err := w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(metadata, []byte("metadata")) {
		t.Errorf("metadata = %s, want %s", metadata, "metadata")
	}
	if !reflect.DeepEqual(entries, ents) {
		t.Errorf("ents = %+v, want %+v", entries, ents)
	}
	// only the latest state is recorded
	s := sts[len(sts)-1]
	if !reflect.DeepEqual(state, s) {
		t.Errorf("state = %+v, want %+v", state, s)
	}
	w.Close()
}

func TestSearchIndex(t *testing.T) {
	tests := []struct {
		names []string
		index uint64
		widx  int
		wok   bool
	}{
		{
			[]string{
				"0000000000000000-0000000000000000.wal",
				"0000000000000001-0000000000001000.wal",
				"0000000000000002-0000000000002000.wal",
			},
			0x1000, 1, true,
		},
		{
			[]string{
				"0000000000000001-0000000000004000.wal",
				"0000000000000002-0000000000003000.wal",
				"0000000000000003-0000000000005000.wal",
			},
			0x4000, 1, true,
		},
		{
			[]string{
				"0000000000000001-0000000000002000.wal",
				"0000000000000002-0000000000003000.wal",
				"0000000000000003-0000000000005000.wal",
			},
			0x1000, -1, false,
		},
	}
	for i, tt := range tests {
		idx, ok := searchIndex(zap.NewExample(), tt.names, tt.index)
		if idx != tt.widx {
			t.Errorf("#%d: idx = %d, want %d", i, idx, tt.widx)
		}
		if ok != tt.wok {
			t.Errorf("#%d: ok = %v, want %v", i, ok, tt.wok)
		}
	}
}

func TestScanWalName(t *testing.T) {
	tests := []struct {
		str          string
		wseq, windex uint64
		wok          bool
	}{
		{"0000000000000000-0000000000000000.wal", 0, 0, true},
		{"0000000000000000.wal", 0, 0, false},
		{"0000000000000000-0000000000000000.snap", 0, 0, false},
	}
	for i, tt := range tests {
		s, index, err := parseWALName(tt.str)
		if g := err == nil; g != tt.wok {
			t.Errorf("#%d: ok = %v, want %v", i, g, tt.wok)
		}
		if s != tt.wseq {
			t.Errorf("#%d: seq = %d, want %d", i, s, tt.wseq)
		}
		if index != tt.windex {
			t.Errorf("#%d: index = %d, want %d", i, index, tt.windex)
		}
	}
}

func TestRecoverAfterCut(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	md, err := Create(zap.NewExample(), p, []byte("metadata"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err = md.SaveSnapshot(walpb.Snapshot{Index: uint64(i)}); err != nil {
			t.Fatal(err)
		}
		es := []raftpb.Entry{{Index: uint64(i)}}
		if err = md.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
		if err = md.cut(); err != nil {
			t.Fatal(err)
		}
	}
	md.Close()

	if err := os.Remove(filepath.Join(p, walName(4, 4))); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		w, err := Open(zap.NewExample(), p, walpb.Snapshot{Index: uint64(i)})
		if err != nil {
			if i <= 4 {
				if err != ErrFileNotFound {
					t.Errorf("#%d: err = %v, want %v", i, err, ErrFileNotFound)
				}
			} else {
				t.Errorf("#%d: err = %v, want nil", i, err)
			}
			continue
		}
		metadata, _, entries, err := w.ReadAll()
		if err != nil {
			t.Errorf("#%d: err = %v, want nil", i, err)
			continue
		}
		if !bytes.Equal(metadata, []byte("metadata")) {
			t.Errorf("#%d: metadata = %s, want %s", i, metadata, "metadata")
		}
		for j, e := range entries {
			if e.Index != uint64(j+i+1) {
				t.Errorf("#%d: ents[%d].Index = %+v, want %+v", i, j, e.Index, j+i+1)
			}
		}
		w.Close()
	}
}

func TestOpenAtUncommittedIndex(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(zap.NewExample(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.SaveSnapshot(walpb.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	if err = w.Save(raftpb.HardState{}, []raftpb.Entry{{Index: 0}}); err != nil {
		t.Fatal(err)
	}
	w.Close()

	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	// commit up to index 0, try to read index 1
	if _, _, _, err = w.ReadAll(); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	w.Close()
}

// TestOpenForRead tests that OpenForRead can load all files.
// The tests creates WAL directory, and cut out multiple WAL files. Then
// it releases the lock of part of data, and excepts that OpenForRead
// can read out all files even if some are locked for write.
func TestOpenForRead(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	// create WAL
	w, err := Create(zap.NewExample(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	// make 10 separate files
	for i := 0; i < 10; i++ {
		es := []raftpb.Entry{{Index: uint64(i)}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
		if err = w.cut(); err != nil {
			t.Fatal(err)
		}
	}
	// release the lock to 5
	unlockIndex := uint64(5)
	w.ReleaseLockTo(unlockIndex)

	// All are available for read
	w2, err := OpenForRead(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	_, _, ents, err := w2.ReadAll()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := ents[len(ents)-1].Index; g != 9 {
		t.Errorf("last index read = %d, want %d", g, 9)
	}
}

func TestOpenWithMaxIndex(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	// create WAL
	w, err := Create(zap.NewExample(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	es := []raftpb.Entry{{Index: uint64(math.MaxInt64)}}
	if err = w.Save(raftpb.HardState{}, es); err != nil {
		t.Fatal(err)
	}
	w.Close()

	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = w.ReadAll()
	if err == nil || err != ErrSliceOutOfRange {
		t.Fatalf("err = %v, want ErrSliceOutOfRange", err)
	}
}

func TestSaveEmpty(t *testing.T) {
	var buf bytes.Buffer
	var est raftpb.HardState
	w := WAL{
		encoder: newEncoder(&buf, 0, 0),
	}
	if err := w.saveState(&est); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if len(buf.Bytes()) != 0 {
		t.Errorf("buf.Bytes = %d, want 0", len(buf.Bytes()))
	}
}

func TestReleaseLockTo(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	// create WAL
	w, err := Create(zap.NewExample(), p, nil)
	defer func() {
		if err = w.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}

	// release nothing if no files
	err = w.ReleaseLockTo(10)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}

	// make 10 separate files
	for i := 0; i < 10; i++ {
		es := []raftpb.Entry{{Index: uint64(i)}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
		if err = w.cut(); err != nil {
			t.Fatal(err)
		}
	}
	// release the lock to 5
	unlockIndex := uint64(5)
	w.ReleaseLockTo(unlockIndex)

	// expected remaining are 4,5,6,7,8,9,10
	if len(w.locks) != 7 {
		t.Errorf("len(w.locks) = %d, want %d", len(w.locks), 7)
	}
	for i, l := range w.locks {
		var lockIndex uint64
		_, lockIndex, err = parseWALName(filepath.Base(l.Name()))
		if err != nil {
			t.Fatal(err)
		}

		if lockIndex != uint64(i+4) {
			t.Errorf("#%d: lockindex = %d, want %d", i, lockIndex, uint64(i+4))
		}
	}

	// release the lock to 15
	unlockIndex = uint64(15)
	w.ReleaseLockTo(unlockIndex)

	// expected remaining is 10
	if len(w.locks) != 1 {
		t.Errorf("len(w.locks) = %d, want %d", len(w.locks), 1)
	}
	_, lockIndex, err := parseWALName(filepath.Base(w.locks[0].Name()))
	if err != nil {
		t.Fatal(err)
	}

	if lockIndex != uint64(10) {
		t.Errorf("lockindex = %d, want %d", lockIndex, 10)
	}
}

// TestTailWriteNoSlackSpace ensures that tail writes append if there's no preallocated space.
func TestTailWriteNoSlackSpace(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	// create initial WAL
	w, err := Create(zap.NewExample(), p, []byte("metadata"))
	if err != nil {
		t.Fatal(err)
	}
	// write some entries
	for i := 1; i <= 5; i++ {
		es := []raftpb.Entry{{Index: uint64(i), Term: 1, Data: []byte{byte(i)}}}
		if err = w.Save(raftpb.HardState{Term: 1}, es); err != nil {
			t.Fatal(err)
		}
	}
	// get rid of slack space by truncating file
	off, serr := w.tail().Seek(0, io.SeekCurrent)
	if serr != nil {
		t.Fatal(serr)
	}
	if terr := w.tail().Truncate(off); terr != nil {
		t.Fatal(terr)
	}
	w.Close()

	// open, write more
	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, ents, rerr := w.ReadAll()
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(ents) != 5 {
		t.Fatalf("got entries %+v, expected 5 entries", ents)
	}
	// write more entries
	for i := 6; i <= 10; i++ {
		es := []raftpb.Entry{{Index: uint64(i), Term: 1, Data: []byte{byte(i)}}}
		if err = w.Save(raftpb.HardState{Term: 1}, es); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	// confirm all writes
	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, ents, rerr = w.ReadAll()
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(ents) != 10 {
		t.Fatalf("got entries %+v, expected 10 entries", ents)
	}
	w.Close()
}

// TestRestartCreateWal ensures that an interrupted WAL initialization is clobbered on restart
func TestRestartCreateWal(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	// make temporary directory so it looks like initialization is interrupted
	tmpdir := filepath.Clean(p) + ".tmp"
	if err = os.Mkdir(tmpdir, fileutil.PrivateDirMode); err != nil {
		t.Fatal(err)
	}
	if _, err = os.OpenFile(filepath.Join(tmpdir, "test"), os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode); err != nil {
		t.Fatal(err)
	}

	w, werr := Create(zap.NewExample(), p, []byte("abc"))
	if werr != nil {
		t.Fatal(werr)
	}
	w.Close()
	if Exist(tmpdir) {
		t.Fatalf("got %q exists, expected it to not exist", tmpdir)
	}

	if w, err = OpenForRead(zap.NewExample(), p, walpb.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if meta, _, _, rerr := w.ReadAll(); rerr != nil || string(meta) != "abc" {
		t.Fatalf("got error %v and meta %q, expected nil and %q", rerr, meta, "abc")
	}
}

// TestOpenOnTornWrite ensures that entries past the torn write are truncated.
func TestOpenOnTornWrite(t *testing.T) {
	maxEntries := 40
	clobberIdx := 20
	overwriteEntries := 5

	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	w, err := Create(zap.NewExample(), p, nil)
	defer func() {
		if err = w.Close(); err != nil && err != os.ErrInvalid {
			t.Fatal(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}

	// get offset of end of each saved entry
	offsets := make([]int64, maxEntries)
	for i := range offsets {
		es := []raftpb.Entry{{Index: uint64(i)}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
		if offsets[i], err = w.tail().Seek(0, io.SeekCurrent); err != nil {
			t.Fatal(err)
		}
	}

	fn := filepath.Join(p, filepath.Base(w.tail().Name()))
	w.Close()

	// clobber some entry with 0's to simulate a torn write
	f, ferr := os.OpenFile(fn, os.O_WRONLY, fileutil.PrivateFileMode)
	if ferr != nil {
		t.Fatal(ferr)
	}
	defer f.Close()
	_, err = f.Seek(offsets[clobberIdx], io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	zeros := make([]byte, offsets[clobberIdx+1]-offsets[clobberIdx])
	_, err = f.Write(zeros)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	// seek up to clobbered entry
	_, _, _, err = w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	// write a few entries past the clobbered entry
	for i := 0; i < overwriteEntries; i++ {
		// Index is different from old, truncated entries
		es := []raftpb.Entry{{Index: uint64(i + clobberIdx), Data: []byte("new")}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	// read back the entries, confirm number of entries matches expectation
	w, err = OpenForRead(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}

	_, _, ents, rerr := w.ReadAll()
	if rerr != nil {
		// CRC error? the old entries were likely never truncated away
		t.Fatal(rerr)
	}
	wEntries := (clobberIdx - 1) + overwriteEntries
	if len(ents) != wEntries {
		t.Fatalf("expected len(ents) = %d, got %d", wEntries, len(ents))
	}
}

func TestRenameFail(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	oldSegmentSizeBytes := SegmentSizeBytes
	defer func() {
		SegmentSizeBytes = oldSegmentSizeBytes
	}()
	SegmentSizeBytes = math.MaxInt64

	tp, terr := ioutil.TempDir(os.TempDir(), "waltest")
	if terr != nil {
		t.Fatal(terr)
	}
	os.RemoveAll(tp)

	w := &WAL{
		lg:  zap.NewExample(),
		dir: p,
	}
	w2, werr := w.renameWAL(tp)
	if w2 != nil || werr == nil { // os.Rename should fail from 'no such file or directory'
		t.Fatalf("expected error, got %v", werr)
	}
}

// TestReadAllFail ensure ReadAll error if used without opening the WAL
func TestReadAllFail(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// create initial WAL
	f, err := Create(zap.NewExample(), dir, []byte("metadata"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	// try to read without opening the WAL
	_, _, _, err = f.ReadAll()
	if err == nil || err != ErrDecoderNotFound {
		t.Fatalf("err = %v, want ErrDecoderNotFound", err)
	}
}

// TestValidSnapshotEntries ensures ValidSnapshotEntries returns all valid wal snapshot entries, accounting
// for hardstate
func TestValidSnapshotEntries(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	snap0 := walpb.Snapshot{Index: 0, Term: 0}
	snap1 := walpb.Snapshot{Index: 1, Term: 1}
	state1 := raftpb.HardState{Commit: 1, Term: 1}
	snap2 := walpb.Snapshot{Index: 2, Term: 1}
	snap3 := walpb.Snapshot{Index: 3, Term: 2}
	state2 := raftpb.HardState{Commit: 3, Term: 2}
	snap4 := walpb.Snapshot{Index: 4, Term: 2} // will be orphaned since the last committed entry will be snap3
	func() {
		w, err := Create(zap.NewExample(), p, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()

		// snap0 is implicitly created at index 0, term 0
		if err = w.SaveSnapshot(snap1); err != nil {
			t.Fatal(err)
		}
		if err = w.Save(state1, nil); err != nil {
			t.Fatal(err)
		}
		if err = w.SaveSnapshot(snap2); err != nil {
			t.Fatal(err)
		}
		if err = w.SaveSnapshot(snap3); err != nil {
			t.Fatal(err)
		}
		if err = w.Save(state2, nil); err != nil {
			t.Fatal(err)
		}
		if err = w.SaveSnapshot(snap4); err != nil {
			t.Fatal(err)
		}
	}()
	walSnaps, err := ValidSnapshotEntries(zap.NewExample(), p)
	if err != nil {
		t.Fatal(err)
	}
	expected := []walpb.Snapshot{snap0, snap1, snap2, snap3}
	if !reflect.DeepEqual(walSnaps, expected) {
		t.Errorf("expected walSnaps %+v, got %+v", expected, walSnaps)
	}
}

// TestValidSnapshotEntriesAfterPurgeWal ensure that there are many wal files, and after cleaning the first wal file,
// it can work well.
func TestValidSnapshotEntriesAfterPurgeWal(t *testing.T) {
	oldSegmentSizeBytes := SegmentSizeBytes
	SegmentSizeBytes = 64
	defer func() {
		SegmentSizeBytes = oldSegmentSizeBytes
	}()
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)
	snap0 := walpb.Snapshot{Index: 0, Term: 0}
	snap1 := walpb.Snapshot{Index: 1, Term: 1}
	state1 := raftpb.HardState{Commit: 1, Term: 1}
	snap2 := walpb.Snapshot{Index: 2, Term: 1}
	snap3 := walpb.Snapshot{Index: 3, Term: 2}
	state2 := raftpb.HardState{Commit: 3, Term: 2}
	func() {
		w, err := Create(zap.NewExample(), p, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()

		// snap0 is implicitly created at index 0, term 0
		if err = w.SaveSnapshot(snap1); err != nil {
			t.Fatal(err)
		}
		if err = w.Save(state1, nil); err != nil {
			t.Fatal(err)
		}
		if err = w.SaveSnapshot(snap2); err != nil {
			t.Fatal(err)
		}
		if err = w.SaveSnapshot(snap3); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 128; i++ {
			if err = w.Save(state2, nil); err != nil {
				t.Fatal(err)
			}
		}

	}()
	files, _, err := selectWALFiles(nil, p, snap0)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(p + "/" + files[0])
	_, err = ValidSnapshotEntries(zap.NewExample(), p)
	if err != nil {
		t.Fatal(err)
	}
}
