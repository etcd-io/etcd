package wal

import (
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/coreos/etcd/raft"
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

func TestWriteEntry(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	e := &raft.Entry{1, 1, []byte{1}}
	err = w.writeEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	wb := []byte("\x02\x00\x00\x00\x00\x00\x00\x00!\x00\x00\x00\x00\x00\x00\x00{\"Type\":1,\"Term\":1,\"Data\":\"AQ==\"}")
	if !reflect.DeepEqual(b, wb) {
		t.Errorf("ent = %q, want %q", b, wb)
	}

	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteInfo(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	id := int64(0xBEEF)
	err = w.writeInfo(id)
	if err != nil {
		t.Fatal(err)
	}

	// make sure we can only write info at the head of the wal file
	// still in buffer
	err = w.writeInfo(id)
	if err == nil || err.Error() != "cannot write info at 24, expect 0" {
		t.Errorf("err = %v, want cannot write info at 8, expect 0", err)
	}

	// flush to disk
	w.flush()
	err = w.writeInfo(id)
	if err == nil || err.Error() != "cannot write info at 24, expect 0" {
		t.Errorf("err = %v, want cannot write info at 8, expect 0", err)
	}
	w.Close()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	wb := []byte("\x01\x00\x00\x00\x00\x00\x00\x00\b\x00\x00\x00\x00\x00\x00\x00\xef\xbe\x00\x00\x00\x00\x00\x00")
	if !reflect.DeepEqual(b, wb) {
		t.Errorf("ent = %q, want %q", b, wb)
	}

	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteState(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	st := &raft.State{1, 1, 1}
	err = w.writeState(st)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	wb := []byte("\x03\x00\x00\x00\x00\x00\x00\x00\x18\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00")
	if !reflect.DeepEqual(b, wb) {
		t.Errorf("ent = %q, want %q", b, wb)
	}

	err = os.Remove(p)
	if err != nil {
		t.Fatal(err)
	}
}
