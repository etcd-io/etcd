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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"

	"go.uber.org/zap"
)

type corruptFunc func(string, int64) error

// TestRepairTruncate ensures a truncated file can be repaired
func TestRepairTruncate(t *testing.T) {
	corruptf := func(p string, offset int64) error {
		f, err := openLast(zap.NewExample(), p)
		if err != nil {
			return err
		}
		defer f.Close()
		return f.Truncate(offset - 4)
	}

	testRepair(t, makeEnts(10), corruptf, 9)
}

func testRepair(t *testing.T, ents [][]raftpb.Entry, corrupt corruptFunc, expectedEnts int) {
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

	for _, es := range ents {
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
	}

	offset, err := w.tail().Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	err = corrupt(p, offset)
	if err != nil {
		t.Fatal(err)
	}

	// verify we broke the wal
	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = w.ReadAll()
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("err = %v, want error %v", err, io.ErrUnexpectedEOF)
	}
	w.Close()

	// repair the wal
	if ok := Repair(zap.NewExample(), p); !ok {
		t.Fatalf("'Repair' returned '%v', want 'true'", ok)
	}

	// read it back
	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, walEnts, err := w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(walEnts) != expectedEnts {
		t.Fatalf("len(ents) = %d, want %d", len(walEnts), expectedEnts)
	}

	// write some more entries to repaired log
	for i := 1; i <= 10; i++ {
		es := []raftpb.Entry{{Index: uint64(expectedEnts + i)}}
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	// read back entries following repair, ensure it's all there
	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, walEnts, err = w.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(walEnts) != expectedEnts+10 {
		t.Fatalf("len(ents) = %d, want %d", len(walEnts), expectedEnts+10)
	}
}

func makeEnts(ents int) (ret [][]raftpb.Entry) {
	for i := 1; i <= ents; i++ {
		ret = append(ret, []raftpb.Entry{{Index: uint64(i)}})
	}
	return ret
}

// TestRepairWriteTearLast repairs the WAL in case the last record is a torn write
// that straddled two sectors.
func TestRepairWriteTearLast(t *testing.T) {
	corruptf := func(p string, offset int64) error {
		f, err := openLast(zap.NewExample(), p)
		if err != nil {
			return err
		}
		defer f.Close()
		// 512 bytes perfectly aligns the last record, so use 1024
		if offset < 1024 {
			return fmt.Errorf("got offset %d, expected >1024", offset)
		}
		if terr := f.Truncate(1024); terr != nil {
			return terr
		}
		return f.Truncate(offset)
	}
	testRepair(t, makeEnts(50), corruptf, 40)
}

// TestRepairWriteTearMiddle repairs the WAL when there is write tearing
// in the middle of a record.
func TestRepairWriteTearMiddle(t *testing.T) {
	corruptf := func(p string, offset int64) error {
		f, err := openLast(zap.NewExample(), p)
		if err != nil {
			return err
		}
		defer f.Close()
		// corrupt middle of 2nd record
		_, werr := f.WriteAt(make([]byte, 512), 4096+512)
		return werr
	}
	ents := makeEnts(5)
	// 4096 bytes of data so a middle sector is easy to corrupt
	dat := make([]byte, 4096)
	for i := range dat {
		dat[i] = byte(i)
	}
	for i := range ents {
		ents[i][0].Data = dat
	}
	testRepair(t, ents, corruptf, 1)
}

func TestRepairFailDeleteDir(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(zap.NewExample(), p, nil)
	if err != nil {
		t.Fatal(err)
	}

	oldSegmentSizeBytes := SegmentSizeBytes
	SegmentSizeBytes = 64
	defer func() {
		SegmentSizeBytes = oldSegmentSizeBytes
	}()
	for _, es := range makeEnts(50) {
		if err = w.Save(raftpb.HardState{}, es); err != nil {
			t.Fatal(err)
		}
	}

	_, serr := w.tail().Seek(0, io.SeekCurrent)
	if serr != nil {
		t.Fatal(serr)
	}
	w.Close()

	f, err := openLast(zap.NewExample(), p)
	if err != nil {
		t.Fatal(err)
	}
	if terr := f.Truncate(20); terr != nil {
		t.Fatal(err)
	}
	f.Close()

	w, err = Open(zap.NewExample(), p, walpb.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = w.ReadAll()
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("err = %v, want error %v", err, io.ErrUnexpectedEOF)
	}
	w.Close()

	os.RemoveAll(p)
	if Repair(zap.NewExample(), p) {
		t.Fatal("expect 'Repair' fail on unexpected directory deletion")
	}
}
