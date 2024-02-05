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
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	betesting "go.etcd.io/etcd/server/v3/mvcc/backend/testing"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
)

func TestBatchTxPut(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()

	// create bucket
	tx.UnsafeCreateBucket(buckets.Test)

	// put
	v := []byte("bar")
	tx.UnsafePut(buckets.Test, []byte("foo"), v)

	tx.Unlock()

	// check put result before and after tx is committed
	for k := 0; k < 2; k++ {
		tx.Lock()
		_, gv := tx.UnsafeRange(buckets.Test, []byte("foo"), nil, 0)
		tx.Unlock()
		if !reflect.DeepEqual(gv[0], v) {
			t.Errorf("v = %s, want %s", string(gv[0]), string(v))
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

	tx.UnsafeCreateBucket(buckets.Test)
	// put keys
	allKeys := [][]byte{[]byte("foo"), []byte("foo1"), []byte("foo2")}
	allVals := [][]byte{[]byte("bar"), []byte("bar1"), []byte("bar2")}
	for i := range allKeys {
		tx.UnsafePut(buckets.Test, allKeys[i], allVals[i])
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
		keys, vals := tx.UnsafeRange(buckets.Test, tt.key, tt.endKey, tt.limit)
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

	tx.UnsafeCreateBucket(buckets.Test)
	tx.UnsafePut(buckets.Test, []byte("foo"), []byte("bar"))

	tx.UnsafeDelete(buckets.Test, []byte("foo"))

	tx.Unlock()

	// check put result before and after tx is committed
	for k := 0; k < 2; k++ {
		tx.Lock()
		ks, _ := tx.UnsafeRange(buckets.Test, []byte("foo"), nil, 0)
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
	tx.UnsafeCreateBucket(buckets.Test)
	tx.UnsafePut(buckets.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()

	tx.Commit()

	// check whether put happens via db view
	backend.DbFromBackendForTest(b).View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(buckets.Test.Name())
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
	tx.UnsafeCreateBucket(buckets.Test)
	tx.UnsafePut(buckets.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()

	// batch limit commit should have been triggered
	// check whether put happens via db view
	backend.DbFromBackendForTest(b).View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(buckets.Test.Name())
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
	tx.UnsafeCreateBucket(buckets.Test)
	tx.UnsafePut(buckets.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()
	tx.Commit()

	checkForEach(t, b.BatchTx(), b.ReadTx(), [][]byte{[]byte("foo")}, [][]byte{[]byte("bar")})

	tx.Lock()
	tx.UnsafeDeleteBucket(buckets.Test)
	tx.Unlock()

	checkForEach(t, b.BatchTx(), b.ReadTx(), nil, nil)
}

func TestRangeAfterDeleteMatch(t *testing.T) {
	b, _ := betesting.NewTmpBackend(t, time.Hour, 10000)
	defer betesting.Close(t, b)

	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(buckets.Test)
	tx.UnsafePut(buckets.Test, []byte("foo"), []byte("bar"))
	tx.Unlock()
	tx.Commit()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), []byte("foo"), nil, 0)
	checkForEach(t, b.BatchTx(), b.ReadTx(), [][]byte{[]byte("foo")}, [][]byte{[]byte("bar")})

	tx.Lock()
	tx.UnsafeDelete(buckets.Test, []byte("foo"))
	tx.Unlock()

	checkRangeResponseMatch(t, b.BatchTx(), b.ReadTx(), []byte("foo"), nil, 0)
	checkForEach(t, b.BatchTx(), b.ReadTx(), nil, nil)
}

func checkRangeResponseMatch(t *testing.T, tx backend.BatchTx, rtx backend.ReadTx, key, endKey []byte, limit int64) {
	tx.Lock()
	ks1, vs1 := tx.UnsafeRange(buckets.Test, key, endKey, limit)
	tx.Unlock()

	rtx.RLock()
	ks2, vs2 := rtx.UnsafeRange(buckets.Test, key, endKey, limit)
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

func checkUnsafeForEach(t *testing.T, tx backend.ReadTx, expectedKeys, expectedValues [][]byte) {
	var ks, vs [][]byte
	tx.UnsafeForEach(buckets.Test, func(k, v []byte) error {
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
