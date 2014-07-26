package wal

import (
	"bytes"
	"io"
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

func TestParseInfo(t *testing.T) {
	data := []byte("\xef\xbe\x00\x00\x00\x00\x00\x00")
	id, err := parseInfo(data)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0xBEEF {
		t.Errorf("id = %x, want 0xBEEF", id)
	}
}

func TestParseEntry(t *testing.T) {
	data := []byte("{\"Type\":1,\"Term\":1,\"Data\":\"AQ==\"}")
	e, err := parseEntry(data)
	if err != nil {
		t.Fatal(err)
	}
	we := raft.Entry{1, 1, []byte{1}}
	if !reflect.DeepEqual(e, we) {
		t.Errorf("ent = %v, want %v", e, we)
	}
}

func TestParseState(t *testing.T) {
	data := []byte("\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00")
	s, err := parseState(data)
	if err != nil {
		t.Fatal(err)
	}
	ws := raft.State{1, 1, 1}
	if !reflect.DeepEqual(s, ws) {
		t.Errorf("state = %v, want %v", s, ws)
	}
}

func TestReadBlock(t *testing.T) {
	tests := []struct {
		data []byte
		wb   *block
		we   error
	}{
		{
			[]byte("\x01\x00\x00\x00\x00\x00\x00\x00\b\x00\x00\x00\x00\x00\x00\x00\xef\xbe\x00\x00\x00\x00\x00\x00"),
			&block{1, 8, []byte("\xef\xbe\x00\x00\x00\x00\x00\x00")},
			nil,
		},
		{
			[]byte(""),
			nil,
			io.EOF,
		},
		{
			[]byte("\x01\x00\x00\x00"),
			nil,
			io.ErrUnexpectedEOF,
		},
		{
			[]byte("\x01\x00\x00\x00\x00\x00\x00\x00"),
			nil,
			io.ErrUnexpectedEOF,
		},
		{
			[]byte("\x01\x00\x00\x00\x00\x00\x00\x00\b\x00\x00\x00\x00\x00\x00\x00"),
			nil,
			io.ErrUnexpectedEOF,
		},
	}

	for i, tt := range tests {
		buf := bytes.NewBuffer(tt.data)
		b, e := readBlock(buf)
		if !reflect.DeepEqual(b, tt.wb) {
			t.Errorf("#%d: block = %v, want %v", i, b, tt.wb)
		}
		if !reflect.DeepEqual(e, tt.we) {
			t.Errorf("#%d: err = %v, want %v", i, e, tt.we)
		}
	}
}

func TestReadNode(t *testing.T) {
	p := path.Join(os.TempDir(), "waltest")
	w, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	id := int64(0xBEEF)
	if err = w.writeInfo(id); err != nil {
		t.Fatal(err)
	}
	ents := []raft.Entry{{1, 1, []byte{1}}, {2, 2, []byte{2}}}
	for _, e := range ents {
		if err = w.writeEntry(&e); err != nil {
			t.Fatal(err)
		}
	}
	sts := []raft.State{{1, 1, 1}, {2, 2, 2}}
	for _, s := range sts {
		if err = w.writeState(&s); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	w, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	n, err := w.ReadNode()
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
