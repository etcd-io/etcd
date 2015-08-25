package storage

import (
	"crypto/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/storage/storagepb"
)

// TODO: improve to a unit test
func TestRangeLimitWhenKeyDeleted(t *testing.T) {
	s := newStore(tmpPath)
	defer os.Remove(tmpPath)

	s.Put([]byte("foo"), []byte("bar"))
	s.Put([]byte("foo1"), []byte("bar1"))
	s.Put([]byte("foo2"), []byte("bar2"))
	s.DeleteRange([]byte("foo1"), nil)
	kvs := []storagepb.KeyValue{
		{Key: []byte("foo"), Value: []byte("bar"), CreateIndex: 1, ModIndex: 1, Version: 1},
		{Key: []byte("foo2"), Value: []byte("bar2"), CreateIndex: 3, ModIndex: 3, Version: 1},
	}

	tests := []struct {
		limit int64
		wkvs  []storagepb.KeyValue
	}{
		// no limit
		{0, kvs},
		{1, kvs[:1]},
		{2, kvs},
		{3, kvs},
	}
	for i, tt := range tests {
		kvs, _, err := s.Range([]byte("foo"), []byte("foo3"), tt.limit, 0)
		if err != nil {
			t.Fatalf("#%d: range error (%v)", i, err)
		}
		if !reflect.DeepEqual(kvs, tt.wkvs) {
			t.Errorf("#%d: kvs = %+v, want %+v", i, kvs, tt.wkvs)
		}
	}
}

func TestRestoreContinueUnfinishedCompaction(t *testing.T) {
	s0 := newStore(tmpPath)
	defer os.Remove(tmpPath)

	s0.Put([]byte("foo"), []byte("bar"))
	s0.Put([]byte("foo"), []byte("bar1"))
	s0.Put([]byte("foo"), []byte("bar2"))

	// write scheduled compaction, but not do compaction
	rbytes := newRevBytes()
	revToBytes(revision{main: 2}, rbytes)
	tx := s0.b.BatchTx()
	tx.Lock()
	tx.UnsafePut(metaBucketName, scheduledCompactKeyName, rbytes)
	tx.Unlock()

	s0.Close()

	s1 := newStore(tmpPath)
	s1.Restore()

	// wait for scheduled compaction to be finished
	time.Sleep(100 * time.Millisecond)

	if _, _, err := s1.Range([]byte("foo"), nil, 0, 2); err != ErrCompacted {
		t.Errorf("range on compacted rev error = %v, want %v", err, ErrCompacted)
	}
	// check the key in backend is deleted
	revbytes := newRevBytes()
	// TODO: compact should delete main=2 key too
	revToBytes(revision{main: 1}, revbytes)
	tx = s1.b.BatchTx()
	tx.Lock()
	ks, _ := tx.UnsafeRange(keyBucketName, revbytes, nil, 0)
	if len(ks) != 0 {
		t.Errorf("key for rev %+v still exists, want deleted", bytesToRev(revbytes))
	}
	tx.Unlock()
}

func BenchmarkStorePut(b *testing.B) {
	s := newStore(tmpPath)
	defer os.Remove(tmpPath)

	// prepare keys
	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = make([]byte, 64)
		rand.Read(keys[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[i], []byte("foo"))
	}
}
