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
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

func TestBatchTxPut(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()

	// create bucket
	tx.UnsafeCreateBucket(schema.Test)

	// put
	v := []byte("bar")
	tx.UnsafePut(schema.Test, []byte("foo"), v)

	tx.Unlock()

	// check put result before and after tx is committed
	for k := 0; k < 2; k++ {
		tx.Lock()
		_, gv := tx.UnsafeRange(schema.Test, []byte("foo"), nil, 0)
		tx.Unlock()
		if !reflect.DeepEqual(gv[0], v) {
			t.Errorf("v = %s, want %s", gv[0], v)
		}
		tx.Commit()
	}
}

func TestBatchTxRange(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	tx.UnsafeCreateBucket(schema.Test)
	// put keys
	allKeys := [][]byte{[]byte("foo"), []byte("foo1"), []byte("foo2")}
	allVals := [][]byte{[]byte("bar"), []byte("bar1"), []byte("bar2")}
	for i := range allKeys {
		tx.UnsafePut(schema.Test, allKeys[i], allVals[i])
	}

	tests := []struct {
		key    []byte
		endKey []byte
		limit  int64

		wkeys [][]byte
		wvals [][]byte
	}{
		// single key
		{
			[]byte("foo"), nil, 0,
			allKeys[:1], allVals[:1],
		},
		// single key, bad
		{
			[]byte("doo"), nil, 0,
			nil, nil,
		},
		// key range
		{
			[]byte("foo"), []byte("foo1"), 0,
			allKeys[:1], allVals[:1],
		},
		// key range, get all keys
		{
			[]byte("foo"), []byte("foo3"), 0,
			allKeys, allVals,
		},
		// key range, bad
		{
			[]byte("goo"), []byte("goo3"), 0,
			nil, nil,
		},
		// key range with effective limit
		{
			[]byte("foo"), []byte("foo3"), 1,
			allKeys[:1], allVals[:1],
		},
		// key range with limit
		{
			[]byte("foo"), []byte("foo3"), 4,
			allKeys, allVals,
		},
	}
	for i, tt := range tests {
		keys, vals := tx.UnsafeRange(schema.Test, tt.key, tt.endKey, tt.limit)
		if !reflect.DeepEqual(keys, tt.wkeys) {
			t.Errorf("#%d: keys = %+v, want %+v", i, keys, tt.wkeys)
		}
		if !reflect.DeepEqual(vals, tt.wvals) {
			t.Errorf("#%d: vals = %+v, want %+v", i, vals, tt.wvals)
		}
	}
}

func TestBatchTxDelete(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()

	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))

	tx.UnsafeDelete(schema.Test, []byte("foo"))

	tx.Unlock()

	// check put result before and after tx is committed
	for k := 0; k < 2; k++ {
		tx.Lock()
		ks, _ := tx.UnsafeRange(schema.Test, []byte("foo"), nil, 0)
		tx.Unlock()
		if len(ks) != 0 {
			t.Errorf("keys on foo = %v, want nil", ks)
		}
		tx.Commit()
	}
}

func TestBatchTxCommit(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()

	tx.Commit()

	// check whether put happens via db view
	backend.DbFromBackendForTest(b).View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(schema.Test.Name())
		if bucket == nil {
			t.Errorf("bucket test does not exit")
			return nil
		}
		v := bucket.Get([]byte("foo"))
		if v == nil {
			t.Errorf("foo key failed to written in backend")
		}
		return nil
	})
}

func TestBatchTxBatchLimitCommit(t *testing.T) {
	// start backend with batch limit 1 so one write can
	// trigger a commit
	b, _ := betesting.NewTmpBackend(t, time.Hour, 1)
	defer betesting.Close(t, b)

	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()

	// batch limit commit should have been triggered
	// check whether put happens via db view
	backend.DbFromBackendForTest(b).View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(schema.Test.Name())
		if bucket == nil {
			t.Errorf("bucket test does not exit")
			return nil
		}
		v := bucket.Get([]byte("foo"))
		if v == nil {
			t.Errorf("foo key failed to written in backend")
		}
		return nil
	})
}

func TestRangeAfterDeleteBucketMatch(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()
	tx.Commit()

	checkForEach(t, b.BatchTx(), b.ReadTx(), [][]byte{[]byte("foo")}, [][]byte{[]byte("bar")})

	tx.Lock()
	tx.UnsafeDeleteBucket(schema.Test)
	tx.Unlock()

	checkForEach(t, b.BatchTx(), b.ReadTx(), nil, nil)
}

func TestRangeAfterDeleteMatch(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()
	tx.Commit()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo"), nil, 0)
	checkForEach(t, b.BatchTx(), b.ReadTx(), [][]byte{[]byte("foo")}, [][]byte{[]byte("bar")})

	tx.Lock()
	tx.UnsafeDelete(schema.Test, []byte("foo"))
	tx.Unlock()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo"), nil, 0)
	checkForEach(t, b.BatchTx(), b.ReadTx(), nil, nil)
}

func TestRangeAfterUnorderedKeyWriteMatch(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo5"), []byte("bar5"))
	tx.UnsafePut(schema.Test, []byte("foo2"), []byte("bar2"))
	tx.UnsafePut(schema.Test, []byte("foo1"), []byte("bar1"))
	tx.UnsafePut(schema.Test, []byte("foo3"), []byte("bar3"))
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.UnsafePut(schema.Test, []byte("foo4"), []byte("bar4"))
	tx.Unlock()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo"), nil, 1)
}

func TestRangeAfterAlternatingBucketWriteMatch(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Key)
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafeSeqPut(schema.Key, []byte("key1"), []byte("val1"))
	tx.Unlock()

	tx.Lock()
	tx.UnsafeSeqPut(schema.Key, []byte("key2"), []byte("val2"))
	tx.Unlock()
	tx.Commit()
	// only in the 2nd commit the schema.Key key is removed from the readBuffer.buckets.
	// This makes sure to test the case when an empty writeBuffer.bucket
	// is used to replace the read buffer bucket.
	tx.Commit()

	tx.Lock()
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()
	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Key, []byte("key"), []byte("key5"), 100)
	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo"), []byte("foo3"), 1)
}

func TestRangeAfterOverwriteMatch(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar2"))
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar0"))
	tx.UnsafePut(schema.Test, []byte("foo1"), []byte("bar10"))
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar1"))
	tx.UnsafePut(schema.Test, []byte("foo1"), []byte("bar11"))
	tx.Unlock()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo"), []byte("foo3"), 1)
	checkForEach(t, b.BatchTx(), b.ReadTx(), [][]byte{[]byte("foo"), []byte("foo1")}, [][]byte{[]byte("bar1"), []byte("bar11")})
}

func TestRangeAfterOverwriteAndDeleteMatch(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar2"))
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar0"))
	tx.UnsafePut(schema.Test, []byte("foo1"), []byte("bar10"))
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar1"))
	tx.UnsafePut(schema.Test, []byte("foo1"), []byte("bar11"))
	tx.Unlock()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo"), nil, 0)
	checkForEach(t, b.BatchTx(), b.ReadTx(), [][]byte{[]byte("foo"), []byte("foo1")}, [][]byte{[]byte("bar1"), []byte("bar11")})

	tx.Lock()
	tx.UnsafePut(schema.Test, []byte("foo"), []byte("bar3"))
	tx.UnsafeDelete(schema.Test, []byte("foo1"))
	tx.Unlock()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo"), nil, 0)
	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), schema.Test, []byte("foo1"), nil, 0)
	checkForEach(t, b.BatchTx(), b.ReadTx(), [][]byte{[]byte("foo")}, [][]byte{[]byte("bar3")})
}

func checkRangeResponseMatch(t *testing.T, tx backend.BatchTx, rtx backend.ReadTx, bucket backend.Bucket, key, endKey []byte, limit int64) {
	tx.Lock()
	ks1, vs1 := tx.UnsafeRange(bucket, key, endKey, limit)
	tx.Unlock()

	rtx.RLock()
	ks2, vs2 := rtx.UnsafeRange(bucket, key, endKey, limit)
	rtx.RUnlock()

	if diff := cmp.Diff(ks1, ks2); diff != "" {
		t.Errorf("keys on read and batch transaction doesn't match, diff: %s", diff)
	}
	if diff := cmp.Diff(vs1, vs2); diff != "" {
		t.Errorf("values on read and batch transaction doesn't match, diff: %s", diff)
	}
}

func checkForEach(t *testing.T, tx backend.BatchTx, rtx backend.ReadTx, expectedKeys, expectedValues [][]byte) {
	tx.Lock()
	checkUnsafeForEach(t, tx, expectedKeys, expectedValues)
	tx.Unlock()

	rtx.RLock()
	checkUnsafeForEach(t, rtx, expectedKeys, expectedValues)
	rtx.RUnlock()
}

func checkUnsafeForEach(t *testing.T, tx backend.UnsafeReader, expectedKeys, expectedValues [][]byte) {
	var ks, vs [][]byte
	tx.UnsafeForEach(schema.Test, func(k, v []byte) error {
		ks = append(ks, k)
		vs = append(vs, v)
		return nil
	})

	if diff := cmp.Diff(ks, expectedKeys); diff != "" {
		t.Errorf("keys on transaction doesn't match expected, diff: %s", diff)
	}
	if diff := cmp.Diff(vs, expectedValues); diff != "" {
		t.Errorf("values on transaction doesn't match expected, diff: %s", diff)
	}
}

// runWriteback is used test the txWriteBuffer.writeback function, which is called inside tx.Unlock().
// The parameters are chosen based on defaultBatchLimit = 10000
func runWriteback(t testing.TB, kss, vss [][]string, isSeq bool) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	tx.UnsafeCreateBucket(schema.Key)
	tx.Unlock()

	for i, ks := range kss {
		vs := vss[i]
		tx.Lock()
		for j := 0; j < len(ks); j++ {
			if isSeq {
				tx.UnsafeSeqPut(schema.Key, []byte(ks[j]), []byte(vs[j]))
			} else {
				tx.UnsafePut(schema.Test, []byte(ks[j]), []byte(vs[j]))
			}
		}
		tx.Unlock()
	}
}

func BenchmarkWritebackSeqBatches1BatchSize10000(b *testing.B) { benchmarkWriteback(b, 1, 10000, true) }

func BenchmarkWritebackSeqBatches10BatchSize1000(b *testing.B) { benchmarkWriteback(b, 10, 1000, true) }

func BenchmarkWritebackSeqBatches100BatchSize100(b *testing.B) { benchmarkWriteback(b, 100, 100, true) }

func BenchmarkWritebackSeqBatches1000BatchSize10(b *testing.B) { benchmarkWriteback(b, 1000, 10, true) }

func BenchmarkWritebackNonSeqBatches1000BatchSize1(b *testing.B) {
	// for non sequential writes, the batch size is usually small, 1 or the order of cluster size.
	benchmarkWriteback(b, 1000, 1, false)
}

func BenchmarkWritebackNonSeqBatches10000BatchSize1(b *testing.B) {
	benchmarkWriteback(b, 10000, 1, false)
}

func BenchmarkWritebackNonSeqBatches100BatchSize10(b *testing.B) {
	benchmarkWriteback(b, 100, 10, false)
}

func BenchmarkWritebackNonSeqBatches1000BatchSize10(b *testing.B) {
	benchmarkWriteback(b, 1000, 10, false)
}

func benchmarkWriteback(b *testing.B, batches, batchSize int, isSeq bool) {
	// kss and vss are key and value arrays to write with size batches*batchSize
	var kss, vss [][]string
	for i := 0; i < batches; i++ {
		var ks, vs []string
		for j := i * batchSize; j < (i+1)*batchSize; j++ {
			k := fmt.Sprintf("key%d", j)
			v := fmt.Sprintf("val%d", j)
			ks = append(ks, k)
			vs = append(vs, v)
		}
		if !isSeq {
			// make sure each batch is shuffled differently but the same for different test runs.
			shuffleList(ks, i*batchSize)
		}
		kss = append(kss, ks)
		vss = append(vss, vs)
	}
	b.ResetTimer()
	for n := 1; n < b.N; n++ {
		runWriteback(b, kss, vss, isSeq)
	}
}

func shuffleList(l []string, seed int) {
	r := rand.New(rand.NewSource(int64(seed)))
	for i := 0; i < len(l); i++ {
		j := r.Intn(i + 1)
		l[i], l[j] = l[j], l[i]
	}
}
