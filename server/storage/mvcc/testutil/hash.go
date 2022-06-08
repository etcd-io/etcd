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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
			assert.NoError(t, err, "error on delete")
		} else {
			err := h.Put(ctx, PickKey(i), fmt.Sprint(i))
			assert.NoError(t, err, "error on put")
		}
	}
	hash1, err := h.HashByRev(ctx, stop)
	assert.NoError(t, err, "error on hash (rev %v)", stop)

	err = h.Compact(ctx, stop)
	assert.NoError(t, err, "error on compact (rev %v)", stop)

	err = h.Defrag(ctx)
	assert.NoError(t, err, "error on defrag")

	hash2, err := h.HashByRev(ctx, stop)
	assert.NoError(t, err, "error on hash (rev %v)", stop)
	assert.Equal(t, hash1, hash2, "hashes do not match on rev %v", stop)
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
