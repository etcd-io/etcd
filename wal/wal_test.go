package wal

import (
	"io/ioutil"
	"os"
	"testing"
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
