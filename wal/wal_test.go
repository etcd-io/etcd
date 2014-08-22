/*
Copyright 2014 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package wal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/coreos/etcd/raft"
)

var (
	infoData   = []byte("\b\xef\xfd\x02")
	infoRecord = append([]byte("\n\x00\x00\x00\x00\x00\x00\x00\b\x01\x10\x00\x1a\x04"), infoData...)

	stateData   = []byte("\b\x01\x10\x01\x18\x01")
	stateRecord = append([]byte("\f\x00\x00\x00\x00\x00\x00\x00\b\x03\x10\x00\x1a\x06"), stateData...)

	entryData   = []byte("\b\x01\x10\x01\x18\x01\x22\x01\x01")
	entryRecord = append([]byte("\x0f\x00\x00\x00\x00\x00\x00\x00\b\x02\x10\x00\x1a\t"), entryData...)

	firstWalName = "0000000000000000-0000000000000000.wal"
)

func TestNew(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := path.Base(w.f.Name()); g != firstWalName {
		t.Errorf("name = %+v, want %+v", g, firstWalName)
	}
	w.Close()
}

func TestNewForInitedDir(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	os.Create(path.Join(p, firstWalName))
	if _, err = Create(p); err == nil || err != os.ErrExist {
		t.Errorf("err = %v, want %v", err, os.ErrExist)
	}
}

func TestAppend(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	os.Create(path.Join(p, firstWalName))
	w, err := Open(p)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := path.Base(w.f.Name()); g != firstWalName {
		t.Errorf("name = %+v, want %+v", g, firstWalName)
	}
	w.Close()

	wname := fmt.Sprintf("%016x-%016x.wal", 2, 10)
	os.Create(path.Join(p, wname))
	w, err = Open(p)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if g := path.Base(w.f.Name()); g != wname {
		t.Errorf("name = %+v, want %+v", g, wname)
	}
	w.Close()
}

func TestAppendForUninitedDir(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	if _, err = Open(p); err != ErrNotFound {
		t.Errorf("err = %v, want %v", err, ErrNotFound)
	}
}

func TestCut(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Cut(0); err != nil {
		t.Fatal(err)
	}
	wname := fmt.Sprintf("%016x-%016x.wal", 1, 0)
	if g := path.Base(w.f.Name()); g != wname {
		t.Errorf("name = %s, want %s", g, wname)
	}

	e := &raft.Entry{Type: 1, Index: 1, Term: 1, Data: []byte{1}}
	if err := w.SaveEntry(e); err != nil {
		t.Fatal(err)
	}
	if err := w.Cut(1); err != nil {
		t.Fatal(err)
	}
	wname = fmt.Sprintf("%016x-%016x.wal", 2, 1)
	if g := path.Base(w.f.Name()); g != wname {
		t.Errorf("name = %s, want %s", g, wname)
	}
}

func TestSaveEntry(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p)
	if err != nil {
		t.Fatal(err)
	}
	e := &raft.Entry{Type: 1, Index: 1, Term: 1, Data: []byte{1}}
	err = w.SaveEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	b, err := ioutil.ReadFile(path.Join(p, firstWalName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, entryRecord) {
		t.Errorf("ent = %q, want %q", b, entryRecord)
	}
}

func TestSaveInfo(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p)
	if err != nil {
		t.Fatal(err)
	}
	i := &raft.Info{Id: int64(0xBEEF)}
	err = w.SaveInfo(i)
	if err != nil {
		t.Fatal(err)
	}

	// make sure we can only write info at the head of the wal file
	// still in buffer
	err = w.SaveInfo(i)
	if err == nil || err.Error() != "cannot write info at 18, expect 0" {
		t.Errorf("err = %v, want cannot write info at 18, expect 0", err)
	}

	// sync to disk
	w.Sync()
	err = w.SaveInfo(i)
	if err == nil || err.Error() != "cannot write info at 18, expect 0" {
		t.Errorf("err = %v, want cannot write info at 18, expect 0", err)
	}
	w.Close()

	b, err := ioutil.ReadFile(path.Join(p, firstWalName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, infoRecord) {
		t.Errorf("ent = %q, want %q", b, infoRecord)
	}
}

func TestSaveState(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p)
	if err != nil {
		t.Fatal(err)
	}
	st := &raft.State{Term: 1, Vote: 1, Commit: 1}
	err = w.SaveState(st)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	b, err := ioutil.ReadFile(path.Join(p, firstWalName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, stateRecord) {
		t.Errorf("ent = %q, want %q", b, stateRecord)
	}
}

func TestLoadInfo(t *testing.T) {
	i, err := loadInfo(infoData)
	if err != nil {
		t.Fatal(err)
	}
	if i.Id != 0xBEEF {
		t.Errorf("id = %x, want 0xBEEF", i.Id)
	}
}

func TestLoadEntry(t *testing.T) {
	e, err := loadEntry(entryData)
	if err != nil {
		t.Fatal(err)
	}
	we := raft.Entry{Type: 1, Index: 1, Term: 1, Data: []byte{1}}
	if !reflect.DeepEqual(e, we) {
		t.Errorf("ent = %v, want %v", e, we)
	}
}

func TestLoadState(t *testing.T) {
	s, err := loadState(stateData)
	if err != nil {
		t.Fatal(err)
	}
	ws := raft.State{Term: 1, Vote: 1, Commit: 1}
	if !reflect.DeepEqual(s, ws) {
		t.Errorf("state = %v, want %v", s, ws)
	}
}

func TestNodeLoad(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p)
	if err != nil {
		t.Fatal(err)
	}
	i := &raft.Info{Id: int64(0xBEEF)}
	if err = w.SaveInfo(i); err != nil {
		t.Fatal(err)
	}
	ents := []raft.Entry{{Type: 1, Index: 1, Term: 1, Data: []byte{1}}, {Type: 2, Index: 2, Term: 2, Data: []byte{2}}}
	for _, e := range ents {
		if err = w.SaveEntry(&e); err != nil {
			t.Fatal(err)
		}
	}
	sts := []raft.State{{Term: 1, Vote: 1, Commit: 1}, {Term: 2, Vote: 2, Commit: 2}}
	for _, s := range sts {
		if err = w.SaveState(&s); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	n := newNode(0)
	if err := n.load(path.Join(p, firstWalName)); err != nil {
		t.Fatal(err)
	}
	if n.Id != i.Id {
		t.Errorf("id = %d, want %d", n.Id, i.Id)
	}
	if !reflect.DeepEqual(n.Ents, ents) {
		t.Errorf("ents = %+v, want %+v", n.Ents, ents)
	}
	// only the latest state is recorded
	s := sts[len(sts)-1]
	if !reflect.DeepEqual(n.State, s) {
		t.Errorf("state = %+v, want %+v", n.State, s)
	}
}

func TestSearchIndex(t *testing.T) {
	tests := []struct {
		names []string
		index int64
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
		idx, ok := searchIndex(tt.names, tt.index)
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
		wseq, windex int64
		wok          bool
	}{
		{"0000000000000000-0000000000000000.wal", 0, 0, true},
		{"0000000000000000.wal", 0, 0, false},
		{"0000000000000000-0000000000000000.snap", 0, 0, false},
	}
	for i, tt := range tests {
		s, index, err := parseWalName(tt.str)
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

func TestRead(t *testing.T) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p)
	if err != nil {
		t.Fatal(err)
	}
	info := &raft.Info{Id: int64(0xBEEF)}
	if err = w.SaveInfo(info); err != nil {
		t.Fatal(err)
	}
	if err = w.Cut(0); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 10; i++ {
		e := raft.Entry{Index: int64(i)}
		if err = w.SaveEntry(&e); err != nil {
			t.Fatal(err)
		}
		if err = w.Cut(e.Index); err != nil {
			t.Fatal(err)
		}
		if err = w.SaveInfo(info); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	if err := os.Remove(path.Join(p, "0000000000000004-0000000000000003.wal")); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 15; i++ {
		n, err := Read(p, int64(i))
		if i <= 3 || i >= 10 {
			if err != ErrNotFound {
				t.Errorf("#%d: err = %v, want %v", i, err, ErrNotFound)
			}
			continue
		}
		if err != nil {
			t.Errorf("#%d: err = %v, want nil", i, err)
			continue
		}
		if n.Id != info.Id {
			t.Errorf("#%d: id = %d, want %d", n.Id, info.Id)
		}
		for j, e := range n.Ents {
			if e.Index != int64(j+i+1) {
				t.Errorf("#%d: ents[%d].Index = %+v, want %+v", i, j, e.Index, j+i+1)
			}
		}
	}
}
