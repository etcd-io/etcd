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
	infoData  = []byte("\xef\xbe\x00\x00\x00\x00\x00\x00")
	infoBlock = append([]byte("\x01\x00\x00\x00\x00\x00\x00\x00\b\x00\x00\x00\x00\x00\x00\x00"), infoData...)

	stateData  = []byte("\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00")
	stateBlock = append([]byte("\x03\x00\x00\x00\x00\x00\x00\x00\x18\x00\x00\x00\x00\x00\x00\x00"), stateData...)

	entryJsonData = []byte("{\"Type\":1,\"Term\":1,\"Data\":\"AQ==\"}")
	entryBlock    = append([]byte("\x02\x00\x00\x00\x00\x00\x00\x00\x21\x00\x00\x00\x00\x00\x00\x00"), entryJsonData...)
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
	e := &raft.Entry{1, 1, []byte{1}}
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
	id := int64(0xBEEF)
	err = w.SaveInfo(id)
	if err != nil {
		t.Fatal(err)
	}

	// make sure we can only write info at the head of the wal file
	// still in buffer
	err = w.SaveInfo(id)
	if err == nil || err.Error() != "cannot write info at 24, expect 0" {
		t.Errorf("err = %v, want cannot write info at 8, expect 0", err)
	}

	// flush to disk
	w.Flush()
	err = w.SaveInfo(id)
	if err == nil || err.Error() != "cannot write info at 24, expect 0" {
		t.Errorf("err = %v, want cannot write info at 8, expect 0", err)
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
	st := &raft.State{1, 1, 1}
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
	id, err := loadInfo(infoData)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0xBEEF {
		t.Errorf("id = %x, want 0xBEEF", id)
	}
}

func TestLoadEntry(t *testing.T) {
	e, err := loadEntry(entryJsonData)
	if err != nil {
		t.Fatal(err)
	}
	we := raft.Entry{1, 1, []byte{1}}
	if !reflect.DeepEqual(e, we) {
		t.Errorf("ent = %v, want %v", e, we)
	}
}

func TestLoadState(t *testing.T) {
	s, err := loadState(stateData)
	if err != nil {
		t.Fatal(err)
	}
	ws := raft.State{1, 1, 1}
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
	id := int64(0xBEEF)
	if err = w.SaveInfo(id); err != nil {
		t.Fatal(err)
	}
	ents := []raft.Entry{{1, 1, []byte{1}}, {2, 2, []byte{2}}}
	for _, e := range ents {
		if err = w.SaveEntry(&e); err != nil {
			t.Fatal(err)
		}
	}
	sts := []raft.State{{1, 1, 1}, {2, 2, 2}}
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
	if n.Id != id {
		t.Errorf("id = %d, want %d", n.Id, id)
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
