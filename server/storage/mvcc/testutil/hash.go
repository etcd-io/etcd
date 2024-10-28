// Copyright 2022 The etcd Authors
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

package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

const (
	// CompactionCycle is high prime used to test hash calculation between compactions.
	CompactionCycle = 71
)

func TestCompactionHash(ctx context.Context, t *testing.T, h CompactionHashTestCase, compactionBatchLimit int) {
	var totalRevisions int64 = 1210
	assert.Less(t, int64(compactionBatchLimit), totalRevisions)
	assert.Less(t, int64(CompactionCycle*10), totalRevisions)
	var rev int64
	for ; rev < totalRevisions; rev += CompactionCycle {
		testCompactionHash(ctx, t, h, rev, rev+CompactionCycle)
	}
	testCompactionHash(ctx, t, h, rev, rev+totalRevisions)
}

type CompactionHashTestCase interface {
	Put(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	HashByRev(ctx context.Context, rev int64) (KeyValueHash, error)
	Defrag(ctx context.Context) error
	Compact(ctx context.Context, rev int64) error
}

type KeyValueHash struct {
	Hash            uint32
	CompactRevision int64
	Revision        int64
}

func testCompactionHash(ctx context.Context, t *testing.T, h CompactionHashTestCase, start, stop int64) {
	for i := start; i <= stop; i++ {
		if i%67 == 0 {
			err := h.Delete(ctx, PickKey(i+83))
			require.NoErrorf(t, err, "error on delete")
		} else {
			err := h.Put(ctx, PickKey(i), fmt.Sprint(i))
			require.NoErrorf(t, err, "error on put")
		}
	}
	hash1, err := h.HashByRev(ctx, stop)
	require.NoErrorf(t, err, "error on hash (rev %v)", stop)

	err = h.Compact(ctx, stop)
	require.NoErrorf(t, err, "error on compact (rev %v)", stop)

	err = h.Defrag(ctx)
	require.NoErrorf(t, err, "error on defrag")

	hash2, err := h.HashByRev(ctx, stop)
	require.NoErrorf(t, err, "error on hash (rev %v)", stop)
	assert.Equalf(t, hash1, hash2, "hashes do not match on rev %v", stop)
}

func PickKey(i int64) string {
	if i%(CompactionCycle*2) == 30 {
		return "zenek"
	}
	if i%CompactionCycle == 30 {
		return "xavery"
	}
	// Use low prime number to ensure repeats without alignment
	switch i % 7 {
	case 0:
		return "alice"
	case 1:
		return "bob"
	case 2:
		return "celine"
	case 3:
		return "dominik"
	case 4:
		return "eve"
	case 5:
		return "frederica"
	case 6:
		return "gorge"
	default:
		panic("Can't count")
	}
}

func CorruptBBolt(fpath string) error {
	db, derr := bbolt.Open(fpath, os.ModePerm, &bbolt.Options{})
	if derr != nil {
		return derr
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("key"))
		if b == nil {
			return errors.New("got nil bucket for 'key'")
		}
		var vals [][]byte
		var keys [][]byte
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			keys = append(keys, k)
			var kv mvccpb.KeyValue
			if uerr := kv.Unmarshal(v); uerr != nil {
				return uerr
			}
			kv.Key[0]++
			kv.Value[0]++
			v2, v2err := kv.Marshal()
			if v2err != nil {
				return v2err
			}
			vals = append(vals, v2)
		}
		for i := range keys {
			if perr := b.Put(keys[i], vals[i]); perr != nil {
				return perr
			}
		}
		return nil
	})
}
