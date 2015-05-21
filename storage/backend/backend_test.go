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
