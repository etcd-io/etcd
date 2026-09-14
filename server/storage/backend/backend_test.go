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

package backend_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

func TestBackendClose(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)

	// check close could work
	done := make(chan struct{}, 1)
	go func() {
		err := b.Close()
		if err != nil {
			t.Errorf("close error = %v, want nil", err)
		}
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Errorf("failed to close database in 10s")
	}
}

func TestBackendSnapshot(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()
	b.ForceCommit()

	// write snapshot to a new file
	f, err := os.CreateTemp(t.TempDir(), "etcd_backend_test")
	if err != nil {
		t.Fatal(err)
	}
	snap := b.Snapshot()
	defer func() { assert.NoError(t, snap.Close()) }()
	if _, err := snap.WriteTo(f); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, f.Close())

	// bootstrap new backend from the snapshot
	bcfg := backend.DefaultBackendConfig(zaptest.NewLogger(t))
	bcfg.Path, bcfg.BatchInterval, bcfg.BatchLimit = f.Name(), time.Hour, 10000
	nb := backend.New(bcfg)
	defer betesting.Close(t, nb)

	newTx := nb.BatchTx()
	newTx.Lock()
	ks, _ := newTx.UnsafeRange(schema.Test, []byte("foo"), []byte("goo"), 0)
	if len(ks) != 1 {
		t.Errorf("len(kvs) = %d, want 1", len(ks))
	}
	newTx.Unlock()
}

func TestBackendBatchIntervalCommit(t *testing.T) {
	// start backend with super short batch interval so
	// we do not need to wait long before commit to happen.
	b, _ := betesting.NewTmpBackend(t, time.Nanosecond, 10000)
	defer betesting.Close(t, b)

	pc := backend.CommitsForTest(b)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()

	for i := 0; i < 10; i++ {
		if backend.CommitsForTest(b) >= pc+1 {
			break
		}
		time.Sleep(time.Duration(i*100) * time.Millisecond)
	}

	// check whether put happens via db view
	assert.NoError(t, backend.DbFromBackendForTest(b).View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("test"))
		if bucket == nil {
			t.Errorf("bucket test does not exit")
			return nil
		}
		v := bucket.Get([]byte("foo"))
		if v == nil {
			t.Errorf("foo key failed to written in backend")
		}
		return nil
	}))
}

func TestBackendDefrag(t *testing.T) {
	bcfg := backend.DefaultBackendConfig(zaptest.NewLogger(t))
	// Make sure we change BackendFreelistType
	// The goal is to verify that we restore config option after defrag.
	if bcfg.BackendFreelistType == bolt.FreelistMapType {
		bcfg.BackendFreelistType = bolt.FreelistArrayType
	} else {
		bcfg.BackendFreelistType = bolt.FreelistMapType
	}

	b, _ := betesting.NewTmpBackendFromCfg(t, bcfg)

	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	for i := 0; i < backend.DefragLimitForTest()+100; i++ {
		tx.UnsafePut(schema.Test, []byte(fmt.Sprintf("foo_%d", i)), []byte("bar"))
	}
	tx.Unlock()
	b.ForceCommit()

	// remove some keys to ensure the disk space will be reclaimed after defrag
	tx = b.BatchTx()
	tx.Lock()
	for i := 0; i < 50; i++ {
		tx.UnsafeDelete(schema.Test, []byte(fmt.Sprintf("foo_%d", i)))
	}
	tx.Unlock()
	b.ForceCommit()

	size := b.Size()

	// shrink and check hash
	oh, err := b.Hash(nil)
	if err != nil {
		t.Fatal(err)
	}

	err = b.Defrag()
	if err != nil {
		t.Fatal(err)
	}

	nh, err := b.Hash(nil)
	if err != nil {
		t.Fatal(err)
	}
	if oh != nh {
		t.Errorf("hash = %v, want %v", nh, oh)
	}

	nsize := b.Size()
	if nsize >= size {
		t.Errorf("new size = %v, want < %d", nsize, size)
	}
	db := backend.DbFromBackendForTest(b)
	if db.FreelistType != bcfg.BackendFreelistType {
		t.Errorf("db FreelistType = [%v], want [%v]", db.FreelistType, bcfg.BackendFreelistType)
	}

	// try put more keys after shrink.
	tx = b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("more"), []byte("bar"))
	tx.Unlock()
	b.ForceCommit()
}

// TestBackendDefragNonBlocking verifies that, with NonBlockingDefrag enabled, writers are not blocked while
// Defrag() is running, and that everything written during the run - to both a safe-range bucket
// (schema.Key) and a non-safe-range one (schema.Test) - is present afterward.
func TestBackendDefragNonBlocking(t *testing.T) {
	bcfg := backend.DefaultBackendConfig(zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel)))
	bcfg.NonBlockingDefrag = true
	b, _ := betesting.NewTmpBackendFromCfg(t, bcfg)
	defer betesting.Close(t, b)

	n := backend.DefragLimitForTest() + 100
	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Key)
	tx.UnsafeCreateBucket(schema.Test)
	for i := 0; i < n; i++ {
		tx.UnsafeSeqPut(schema.Key, []byte(fmt.Sprintf("key_%08d", i)), []byte("bar"))
		tx.UnsafePut(schema.Test, []byte(fmt.Sprintf("foo_%d", i)), []byte("bar"))
	}
	tx.Unlock()
	b.ForceCommit()

	// Delete some entries from the non-safe-range bucket, so there's space to reclaim and the
	// catch-up phase's full re-copy of that bucket is exercised against a bucket whose shape
	// changed after the bulk-copy snapshot was taken.
	tx = b.BatchTx()
	tx.Lock()
	for i := 0; i < 50; i++ {
		tx.UnsafeDelete(schema.Test, []byte(fmt.Sprintf("foo_%d", i)))
	}
	tx.Unlock()
	b.ForceCommit()

	defragDone := make(chan error, 1)
	go func() {
		defragDone <- b.Defrag()
	}()

	// Wait for the goroutine above to actually start running Defrag() before racing writes
	// against it below.
	require.Eventuallyf(t, backend.IsDefragActiveForTest, time.Second, time.Millisecond,
		"Defrag() did not start running in time")

	// Issue writes to both buckets while Defrag() is (expected to be) still running its
	// non-blocking bulk-copy phase. A legacy blocking defrag would make every one of these wait
	// for Defrag() to return.
	concurrentWrites := 0
	sawConcurrentProgress := false
loop:
	for i := 0; ; i++ {
		select {
		case err := <-defragDone:
			require.NoError(t, err)
			break loop
		default:
		}
		wtx := b.BatchTx()
		wtx.Lock()
		wtx.UnsafeSeqPut(schema.Key, []byte(fmt.Sprintf("key_%08d", n+i)), []byte("during"))
		wtx.UnsafePut(schema.Test, []byte(fmt.Sprintf("during_%d", i)), []byte("during"))
		wtx.Unlock()
		b.ForceCommit()
		concurrentWrites++
		sawConcurrentProgress = true
		if i > 5000 {
			// Safety valve: don't loop forever if Defrag() unexpectedly never signals.
			require.NoError(t, <-defragDone)
			break loop
		}
	}
	require.Truef(t, sawConcurrentProgress, "expected at least one write to complete while Defrag() was still running (non-blocking defrag should not block writers)")

	tx = b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	keys, _ := tx.UnsafeRange(schema.Key, []byte(fmt.Sprintf("key_%08d", n)), []byte(fmt.Sprintf("key_%08d", n+concurrentWrites)), 0)
	require.Lenf(t, keys, concurrentWrites, "catch-up phase should have copied every key written to the safe-range bucket during the run")
	testKeys, _ := tx.UnsafeRange(schema.Test, []byte("during_"), []byte("during_\xff"), 0)
	require.Lenf(t, testKeys, concurrentWrites, "catch-up phase should have copied every key written to the non-safe-range bucket during the run")
}

// populateForDefragTest writes identical key/value data (schema.Key, the mvcc "key" bucket, is
// the only safe-range bucket) to a backend for the blocking/non-blocking defrag size-comparison
// test below: enough rows to force at least one intermediate defrag commit, then deletes some of
// them so there's free space for defrag to reclaim.
func populateForDefragTest(b backend.Backend) {
	n := backend.DefragLimitForTest() + 100
	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Key)
	for i := 0; i < n; i++ {
		tx.UnsafeSeqPut(schema.Key, []byte(fmt.Sprintf("key_%08d", i)), []byte("bar"))
	}
	tx.Unlock()
	b.ForceCommit()

	tx = b.BatchTx()
	tx.Lock()
	for i := 0; i < 50; i++ {
		tx.UnsafeDelete(schema.Key, []byte(fmt.Sprintf("key_%08d", i)))
	}
	tx.Unlock()
	b.ForceCommit()
}

// TestBackendDefragBlockingAndNonBlockingProduceSameSize verifies that, given identical key/value
// data and no concurrent traffic during the non-blocking run, blocking and non-blocking defrag
// produce a final db of exactly the same size and hash.
func TestBackendDefragBlockingAndNonBlockingProduceSameSize(t *testing.T) {
	blockingCfg := backend.DefaultBackendConfig(zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel)))
	bBlocking, _ := betesting.NewTmpBackendFromCfg(t, blockingCfg)
	defer betesting.Close(t, bBlocking)
	populateForDefragTest(bBlocking)

	nonBlockingCfg := backend.DefaultBackendConfig(zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel)))
	nonBlockingCfg.NonBlockingDefrag = true
	bNonBlocking, _ := betesting.NewTmpBackendFromCfg(t, nonBlockingCfg)
	defer betesting.Close(t, bNonBlocking)
	populateForDefragTest(bNonBlocking)

	requireSameHash := func(msgAndArgs ...any) {
		t.Helper()
		blockingHash, err := bBlocking.Hash(nil)
		require.NoError(t, err)
		nonBlockingHash, err := bNonBlocking.Hash(nil)
		require.NoError(t, err)
		require.Equal(t, blockingHash, nonBlockingHash, msgAndArgs...)
	}

	requireSameHash("pre-defrag hashes should already match: both backends were populated identically")

	require.NoError(t, bBlocking.Defrag())
	require.NoError(t, bNonBlocking.Defrag())

	assert.Equalf(t, bBlocking.Size(), bNonBlocking.Size(), "blocking and non-blocking defrag should produce a db of exactly the same size when there's no traffic during the non-blocking run")
	assert.Equal(t, bBlocking.SizeInUse(), bNonBlocking.SizeInUse())

	requireSameHash()
}

// TestBackendWriteback ensures writes are stored to the read txn on write txn unlock.
func TestBackendWriteback(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Key)
	tx.UnsafePut(schema.Key, []byte("abc"), []byte("bar"))
	tx.UnsafePut(schema.Key, []byte("def"), []byte("baz"))
	tx.UnsafePut(schema.Key, []byte("overwrite"), []byte("1"))
	tx.Unlock()

	// overwrites should be propagated too
	tx.Lock()
	tx.UnsafePut(schema.Key, []byte("overwrite"), []byte("2"))
	tx.Unlock()

	keys := []struct {
		key   []byte
		end   []byte
		limit int64

		wkey [][]byte
		wval [][]byte
	}{
		{
			key: []byte("abc"),
			end: nil,

			wkey: [][]byte{[]byte("abc")},
			wval: [][]byte{[]byte("bar")},
		},
		{
			key: []byte("abc"),
			end: []byte("def"),

			wkey: [][]byte{[]byte("abc")},
			wval: [][]byte{[]byte("bar")},
		},
		{
			key: []byte("abc"),
			end: []byte("deg"),

			wkey: [][]byte{[]byte("abc"), []byte("def")},
			wval: [][]byte{[]byte("bar"), []byte("baz")},
		},
		{
			key:   []byte("abc"),
			end:   []byte("\xff"),
			limit: 1,

			wkey: [][]byte{[]byte("abc")},
			wval: [][]byte{[]byte("bar")},
		},
		{
			key: []byte("abc"),
			end: []byte("\xff"),

			wkey: [][]byte{[]byte("abc"), []byte("def"), []byte("overwrite")},
			wval: [][]byte{[]byte("bar"), []byte("baz"), []byte("2")},
		},
	}
	rtx := b.ReadTx()
	for i, tt := range keys {
		func() {
			rtx.RLock()
			defer rtx.RUnlock()
			k, v := rtx.UnsafeRange(schema.Key, tt.key, tt.end, tt.limit)
			if !reflect.DeepEqual(tt.wkey, k) || !reflect.DeepEqual(tt.wval, v) {
				t.Errorf("#%d: want k=%+v, v=%+v; got k=%+v, v=%+v", i, tt.wkey, tt.wval, k, v)
			}
		}()
	}
}

// TestConcurrentReadTx ensures that current read transaction can see all prior writes stored in read buffer
func TestConcurrentReadTx(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	wtx1 := b.BatchTx()
	wtx1.Lock()
	wtx1.UnsafeCreateBucket(schema.Key)
	wtx1.UnsafePut(schema.Key, []byte("abc"), []byte("ABC"))
	wtx1.UnsafePut(schema.Key, []byte("overwrite"), []byte("1"))
	wtx1.Unlock()

	wtx2 := b.BatchTx()
	wtx2.Lock()
	wtx2.UnsafePut(schema.Key, []byte("def"), []byte("DEF"))
	wtx2.UnsafePut(schema.Key, []byte("overwrite"), []byte("2"))
	wtx2.Unlock()

	rtx := b.ConcurrentReadTx()
	rtx.RLock() // no-op
	k, v := rtx.UnsafeRange(schema.Key, []byte("abc"), []byte("\xff"), 0)
	rtx.RUnlock()
	wKey := [][]byte{[]byte("abc"), []byte("def"), []byte("overwrite")}
	wVal := [][]byte{[]byte("ABC"), []byte("DEF"), []byte("2")}
	if !reflect.DeepEqual(wKey, k) || !reflect.DeepEqual(wVal, v) {
		t.Errorf("want k=%+v, v=%+v; got k=%+v, v=%+v", wKey, wVal, k, v)
	}
}

// TestBackendWritebackForEach checks that partially written / buffered
// data is visited in the same order as fully committed data.
func TestBackendWritebackForEach(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Key)
	for i := 0; i < 5; i++ {
		k := []byte(fmt.Sprintf("%04d", i))
		tx.UnsafePut(schema.Key, k, []byte("bar"))
	}
	tx.Unlock()

	// writeback
	b.ForceCommit()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Key)
	for i := 5; i < 20; i++ {
		k := []byte(fmt.Sprintf("%04d", i))
		tx.UnsafePut(schema.Key, k, []byte("bar"))
	}
	tx.Unlock()

	seq := ""
	getSeq := func(k, v []byte) error {
		seq += string(k)
		return nil
	}
	rtx := b.ReadTx()
	rtx.RLock()
	require.NoError(t, rtx.UnsafeForEach(schema.Key, getSeq))
	rtx.RUnlock()

	partialSeq := seq

	seq = ""
	b.ForceCommit()

	tx.Lock()
	require.NoError(t, tx.UnsafeForEach(schema.Key, getSeq))
	tx.Unlock()

	if seq != partialSeq {
		t.Fatalf("expected %q, got %q", seq, partialSeq)
	}
}
