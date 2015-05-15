package backend

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestBackendPut(t *testing.T) {
	backend := New("test", 10*time.Second, 10000)
	defer backend.Close()
	defer os.Remove("test")

	v := []byte("foo")

	batchTx := backend.BatchTx()
	batchTx.Lock()

	batchTx.UnsafeCreateBucket([]byte("test"))

	batchTx.UnsafePut([]byte("test"), []byte("foo"), v)
	gv := batchTx.UnsafeRange([]byte("test"), v, nil, -1)
	if !reflect.DeepEqual(gv[0], v) {
		t.Errorf("v = %s, want %s", string(gv[0]), string(v))
	}

	batchTx.Unlock()
}

func TestBackendForceCommit(t *testing.T) {
	backend := New("test", 10*time.Second, 10000)
	defer backend.Close()
	defer os.Remove("test")

	v := []byte("foo")
	batchTx := backend.BatchTx()

	batchTx.Lock()

	batchTx.UnsafeCreateBucket([]byte("test"))
	batchTx.UnsafePut([]byte("test"), []byte("foo"), v)

	batchTx.Unlock()

	// expect to see nothing that the batch tx created
	tx := backend.ReadTnx()
	gbucket := tx.Bucket([]byte("test"))
	if gbucket != nil {
		t.Errorf("readtx.bu = %p, want nil", gbucket)
	}
	tx.Commit()

	// commit batch tx
	backend.ForceCommit()
	tx = backend.ReadTnx()
	gbucket = tx.Bucket([]byte("test"))
	if gbucket == nil {
		t.Errorf("readtx.bu = nil, want not nil")
	}
}
