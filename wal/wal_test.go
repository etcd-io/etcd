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
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/coreos/etcd/raft"
)

var (
	infoData  = []byte("\b\xef\xfd\x02")
	infoBlock = append([]byte("\x01\x00\x00\x00\x00\x00\x00\x00\x04\x00\x00\x00\x00\x00\x00\x00"), infoData...)

	stateData  = []byte("\b\x01\x10\x01\x18\x01")
	stateBlock = append([]byte("\x03\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x00\x00\x00\x00"), stateData...)

	entryData  = []byte("\b\x01\x10\x01\x18\x01\x22\x01\x01")
	entryBlock = append([]byte("\x02\x00\x00\x00\x00\x00\x00\x00\t\x00\x00\x00\x00\x00\x00\x00"), entryData...)
)

func TestNew(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "waltest")
	if err != nil {
		t.Fatal(err)
	}
	p := f.Name()
	_, err = New(p)
	if err == nil || err != os.ErrExist {
		t.Errorf("err = %v, want %v", err, os.ErrExist)
	}
	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
	w, err := New(p)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	w.Close()
	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSaveEntry(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	e := &raft.Entry{Type: 1, Index: 1, Term: 1, Data: []byte{1}}
	err = w.SaveEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, entryBlock) {
		t.Errorf("ent = %q, want %q", b, entryBlock)
	}

	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSaveInfo(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
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
	if err == nil || err.Error() != "cannot write info at 20, expect 0" {
		t.Errorf("err = %v, want cannot write info at 20, expect 0", err)
	}

	// sync to disk
	w.Sync()
	err = w.SaveInfo(i)
	if err == nil || err.Error() != "cannot write info at 20, expect 0" {
		t.Errorf("err = %v, want cannot write info at 20, expect 0", err)
	}
	w.Close()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, infoBlock) {
		t.Errorf("ent = %q, want %q", b, infoBlock)
	}

	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSaveState(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	st := &raft.State{Term: 1, Vote: 1, Commit: 1}
	err = w.SaveState(st)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, stateBlock) {
		t.Errorf("ent = %q, want %q", b, stateBlock)
	}

	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
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

func TestLoadNode(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
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

	w, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	n, err := w.LoadNode()
	if err != nil {
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

	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
}
