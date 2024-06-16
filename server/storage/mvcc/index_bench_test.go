// Copyright 2018 The etcd Authors
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

package mvcc

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"

	"go.uber.org/zap"
)

func BenchmarkIndexCompactBase(b *testing.B)          { benchmarkIndexCompact(b, 3, 100) }
func BenchmarkIndexCompactLongKey(b *testing.B)       { benchmarkIndexCompact(b, 512, 100) }
func BenchmarkIndexCompactLargeKeySpace(b *testing.B) { benchmarkIndexCompact(b, 3, 100000) }

func BenchmarkIndexKeepBase(b *testing.B)          { benchmarkIndexKeep(b, 3, 100) }
func BenchmarkIndexKeepLongKey(b *testing.B)       { benchmarkIndexKeep(b, 512, 100) }
func BenchmarkIndexKeepLargeKeySpace(b *testing.B) { benchmarkIndexKeep(b, 3, 100000) }

func BenchmarkIndexPutBase(b *testing.B)          { benchmarkIndexPut(b, 3, 100) }
func BenchmarkIndexPutLongKey(b *testing.B)       { benchmarkIndexPut(b, 512, 100) }
func BenchmarkIndexPutLargeKeySpace(b *testing.B) { benchmarkIndexPut(b, 3, 100000) }

func BenchmarkIndexTombstoneBase(b *testing.B)          { benchmarkIndexTombstone(b, 3, 100, 25) }
func BenchmarkIndexTombstoneLongKey(b *testing.B)       { benchmarkIndexTombstone(b, 512, 100, 25) }
func BenchmarkIndexTombstoneLargeKeySpace(b *testing.B) { benchmarkIndexTombstone(b, 3, 100000, 25) }

func BenchmarkIndexGetBase(b *testing.B)          { benchmarkIndexGet(b, 3, 100, 1, 25) }
func BenchmarkIndexGetRepeatedKeys(b *testing.B)  { benchmarkIndexGet(b, 3, 100, 1000, 25) }
func BenchmarkIndexGetAccurate(b *testing.B)      { benchmarkIndexGet(b, 3, 100, 1, 0) }
func BenchmarkIndexGetMisses(b *testing.B)        { benchmarkIndexGet(b, 3, 100, 1, 1000) }
func BenchmarkIndexGetLongKey(b *testing.B)       { benchmarkIndexGet(b, 512, 100, 1, 25) }
func BenchmarkIndexGetLargeKeySpace(b *testing.B) { benchmarkIndexGet(b, 3, 100000, 1, 25) }

func BenchmarkIndexRangeBase(b *testing.B)            { benchmarkIndexRange(b, 3, 100, 1, 0.1) }
func BenchmarkIndexRangeHighSelectivity(b *testing.B) { benchmarkIndexRange(b, 3, 10, 1, 1) }
func BenchmarkIndexRangeLongKey(b *testing.B)         { benchmarkIndexRange(b, 512, 100, 1, 0.1) }
func BenchmarkIndexRangeRepeatedKeys(b *testing.B)    { benchmarkIndexRange(b, 3, 50, 1000, 0.1) }
func BenchmarkIndexRangeLargeKeySpace(b *testing.B)   { benchmarkIndexRange(b, 3, 100000, 1, 0.00005) }

func BenchmarkIndexRevisionsBase(b *testing.B)            { benchmarkIndexRevisions(b, 3, 100, 1, 0, 0.1) }
func BenchmarkIndexRevisionsHighSelectivity(b *testing.B) { benchmarkIndexRevisions(b, 3, 10, 1, 0, 1) }
func BenchmarkIndexRevisionsLongKey(b *testing.B)         { benchmarkIndexRevisions(b, 512, 100, 1, 0, 0.1) }
func BenchmarkIndexRevisionsRepeatedKeys(b *testing.B) {
	benchmarkIndexRevisions(b, 3, 50, 10000, 0, 0.1)
}
func BenchmarkIndexRevisionsLargeKeySpace(b *testing.B) {
	benchmarkIndexRevisions(b, 3, 100000, 1, 0, 0.00005)
}
func BenchmarkIndexRevisionsLimit(b *testing.B) {
	benchmarkIndexRevisions(b, 3, 100000, 1, 5, 0.1)
}

func BenchmarkIndexCountRevisionsBase(b *testing.B) { benchmarkIndexCountRevisions(b, 3, 100, 1, 0.1) }
func BenchmarkIndexCountRevisionsHighSelectivity(b *testing.B) {
	benchmarkIndexCountRevisions(b, 3, 10, 1, 1)
}
func BenchmarkIndexCountRevisionsLongKey(b *testing.B) {
	benchmarkIndexCountRevisions(b, 512, 100, 1, 0.1)
}
func BenchmarkIndexCountRevisionsRepeatedKeys(b *testing.B) {
	benchmarkIndexCountRevisions(b, 3, 50, 1000, 0.1)
}
func BenchmarkIndexCountRevisionsLargeKeySpace(b *testing.B) {
	benchmarkIndexCountRevisions(b, 3, 100000, 1, 0.00005)
}

func benchmarkIndexCompact(b *testing.B, keyLength, keySpace int) {
	log := zap.NewNop()
	kvindex := newTreeIndex(log)

	keys := createBytesSlice(keyLength, keySpace)
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		kvindex.Put(key, Revision{Main: int64(i + 1), Sub: int64(i)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kvindex.Compact(int64(i + 1))
	}
}

func benchmarkIndexKeep(b *testing.B, keyLength, keySpace int) {
	log := zap.NewNop()
	kvindex := newTreeIndex(log)

	keys := createBytesSlice(keyLength, keySpace)
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		kvindex.Put(key, Revision{Main: int64(i + 1), Sub: int64(i)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kvindex.Keep(int64(i + 1))
	}
}

func benchmarkIndexPut(b *testing.B, keyLength, keySpace int) {
	log := zap.NewNop()
	kvindex := newTreeIndex(log)

	keys := createBytesSlice(keyLength, keySpace)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kvindex.Put(keys[i%len(keys)], Revision{Main: int64(i + 1), Sub: int64(i)})
	}
}

func benchmarkIndexTombstone(b *testing.B, keyLength, keySpace, percentMisses int) {
	log := zap.NewNop()
	indexes := []index{}
	keys := createBytesSlice(keyLength, keySpace+keySpace*percentMisses/100)
	dups := map[string]struct{}{}
	j := 0
	for i := 0; i < len(keys); i++ {
		_, dup := dups[string(keys[i])]
		if dup {
			continue
		}
		keys[j] = keys[i]
		dups[string(keys[i])] = struct{}{}
		j++
	}
	for i := 0; i < b.N/len(keys)+1; i++ {
		index := newTreeIndex(log)
		for j := 0; j < keySpace; j++ {
			key := keys[j]
			rev := int64(j + 1)
			index.Put(key, Revision{Main: rev})
		}
		indexes = append(indexes, index)
	}

	b.ReportAllocs()
	b.ResetTimer()
	deleteCount := 0
	for i := 0; i < b.N; i++ {
		index := indexes[i/len(keys)]
		key := keys[i%len(keys)]
		rev := int64(i%len(keys) + 2)
		err := index.Tombstone(key, Revision{Main: rev})
		if !errors.Is(err, ErrRevisionNotFound) {
			deleteCount += 1
		}
	}
	b.ReportMetric(float64(deleteCount)/float64(b.N), "delete/op")
}

var (
	// global vars to ensure benchmarked function are not optimized.
	rev, created Revision
	ver          int64
	err          error
)

func benchmarkIndexGet(b *testing.B, keyLength, keySpace, keyRepeats, percentMisses int) {
	log := zap.NewNop()
	kvindex := newTreeIndex(log)

	keys := createBytesSlice(keyLength, keySpace)
	revisions := len(keys) * keyRepeats
	for i := 0; i < revisions; i++ {
		key := keys[i%len(keys)]
		kvindex.Put(key, Revision{Main: int64(i + 1), Sub: int64(i)})
	}
	keys = append(keys, createBytesSlice(keyLength, keySpace*percentMisses/100)...)

	type keyRev struct {
		key []byte
		rev int64
	}
	keyRevs := make([]keyRev, 0, b.N)
	for i := 0; i < b.N; i++ {
		key := keys[rand.Intn(len(keys))]
		rev := int64(rand.Intn(revisions) + 1)
		keyRevs = append(keyRevs, keyRev{
			key: key,
			rev: rev,
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	keyCount := 0
	for i := 0; i < b.N; i++ {
		kr := keyRevs[i]
		rev, created, ver, err = kvindex.Get(kr.key, kr.rev)
		if !errors.Is(err, ErrRevisionNotFound) {
			keyCount += 1
		}
	}
	b.ReportMetric(float64(keyCount)/float64(b.N), "keys/op")
}

var (
	// global vars to ensure benchmarked function are not optimized.
	keys  [][]byte
	revs  []Revision
	total int
)

func benchmarkIndexRevisions(b *testing.B, keyLength, keySpace, keyRepeats, limit int, selectivity float64) {
	log := zap.NewNop()
	kvindex := newTreeIndex(log)

	keys := createBytesSlice(keyLength, keySpace)
	revisions := len(keys) * keyRepeats
	for i := 0; i < revisions; i++ {
		key := keys[i%len(keys)]
		kvindex.Put(key, Revision{Main: int64(i + 1), Sub: int64(i)})
	}

	type keyRev struct {
		key, end []byte
		rev      int64
	}
	keyRevs := make([]keyRev, 0, b.N)
	for i := 0; i < b.N; i++ {
		keyFloat := rand.Float64() * (1 - selectivity)
		endFloat := keyFloat + selectivity
		key := FloatToBytes(keyFloat, keyLength)
		end := FloatToBytes(endFloat, keyLength)
		rev := int64(rand.Intn(revisions) + 1)
		keyRevs = append(keyRevs, keyRev{
			key: key,
			end: end,
			rev: rev,
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	revCount := 0
	totalCount := 0
	for i := 0; i < b.N; i++ {
		kr := keyRevs[i]
		revs, total = kvindex.Revisions(kr.key, kr.end, kr.rev, limit)
		revCount += len(revs)
		totalCount += total
	}
	b.ReportMetric(float64(revCount)/float64(b.N), "revs/op")
	b.ReportMetric(float64(totalCount)/float64(b.N), "total/op")
}

func benchmarkIndexRange(b *testing.B, keyLength, keySpace, keyRepeats int, selectivity float64) {
	log := zap.NewNop()
	kvindex := newTreeIndex(log)

	keys := createBytesSlice(keyLength, keySpace)
	revisions := len(keys) * keyRepeats
	for i := 0; i < revisions; i++ {
		key := keys[i%len(keys)]
		kvindex.Put(key, Revision{Main: int64(i + 1), Sub: int64(i)})
	}

	type keyRev struct {
		key, end []byte
		rev      int64
	}
	keyRevs := make([]keyRev, 0, b.N)
	for i := 0; i < b.N; i++ {
		keyFloat := rand.Float64() * (1 - selectivity)
		endFloat := keyFloat + selectivity
		key := FloatToBytes(keyFloat, keyLength)
		end := FloatToBytes(endFloat, keyLength)
		rev := int64(rand.Intn(revisions) + 1)
		keyRevs = append(keyRevs, keyRev{
			key: key,
			end: end,
			rev: rev,
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	keyCount := 0
	for i := 0; i < b.N; i++ {
		kr := keyRevs[i]
		keys, revs = kvindex.Range(kr.key, kr.end, kr.rev)
		keyCount += len(keys)
	}
	b.ReportMetric(float64(keyCount)/float64(b.N), "keys/op")
}

func benchmarkIndexCountRevisions(b *testing.B, keyLength, keySpace, keyRepeats int, selectivity float64) {
	log := zap.NewNop()
	kvindex := newTreeIndex(log)

	keys := createBytesSlice(keyLength, keySpace)
	revisions := len(keys) * keyRepeats
	for i := 0; i < revisions; i++ {
		key := keys[i%len(keys)]
		kvindex.Put(key, Revision{Main: int64(i + 1), Sub: int64(i)})
	}

	type keyRev struct {
		key, end []byte
		rev      int64
	}
	keyRevs := make([]keyRev, 0, b.N)
	for i := 0; i < b.N; i++ {
		keyFloat := rand.Float64() * (1 - selectivity)
		endFloat := keyFloat + selectivity
		key := FloatToBytes(keyFloat, keyLength)
		end := FloatToBytes(endFloat, keyLength)
		rev := int64(rand.Intn(revisions) + 1)
		keyRevs = append(keyRevs, keyRev{
			key: key,
			end: end,
			rev: rev,
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	totalCount := 0
	for i := 0; i < b.N; i++ {
		kr := keyRevs[i]
		totalCount += kvindex.CountRevisions(kr.key, kr.end, kr.rev)
	}
	b.ReportMetric(float64(totalCount)/float64(b.N), "total/op")
}

func TestBytesToFloat(t *testing.T) {
	assert.InDelta(t, 0.0, bytesToFloat([]byte("\x00")), 0.01)
	assert.InDelta(t, 0.25, bytesToFloat([]byte("\x40")), 0.0000001)
	assert.InDelta(t, 0.25, bytesToFloat([]byte("\x40\x00")), 0.0000001)
	assert.InDelta(t, 0.5, bytesToFloat([]byte("\x7f")), 0.01)
	assert.InDelta(t, 0.5, bytesToFloat([]byte("\x7f\xff")), 0.0001)
	assert.InDelta(t, 0.5, bytesToFloat([]byte("\x7f\xff\xff")), 0.0000001)
	assert.InDelta(t, 0.75, bytesToFloat([]byte("\xc0")), 0.0000001)
	assert.InDelta(t, 0.75, bytesToFloat([]byte("\xc0\x00")), 0.0000001)
	assert.InDelta(t, 1.0, bytesToFloat([]byte("\xff")), 0.01)
	assert.InDelta(t, 1.0, bytesToFloat([]byte("\xff\xff")), 0.0001)
	assert.InDelta(t, 1.0, bytesToFloat([]byte("\xff\xff\xff")), 0.00001)
}

func bytesToFloat(bytes []byte) float64 {
	if len(bytes) == 0 {
		panic("empty bytes")
	}
	var result float64
	var factor = 1.0
	for _, b := range bytes {
		factor /= 256
		result += float64(b) * factor
	}
	return result
}

func TestFloatToBytes(t *testing.T) {
	assert.Equal(t, []byte("\x00"), FloatToBytes(0, 1))
	assert.Equal(t, []byte("\x3f"), FloatToBytes(0.25, 1))
	assert.Equal(t, []byte("\x3f\xff"), FloatToBytes(0.25, 2))
	assert.Equal(t, []byte("\x7f"), FloatToBytes(0.5, 1))
	assert.Equal(t, []byte("\x7f\xff"), FloatToBytes(0.5, 2))
	assert.Equal(t, []byte("\x7f\xff\xff"), FloatToBytes(0.5, 3))
	assert.Equal(t, []byte("\xbf"), FloatToBytes(0.75, 1))
	assert.Equal(t, []byte("\xbf\xff"), FloatToBytes(0.75, 2))
	assert.Equal(t, []byte("\xff"), FloatToBytes(1.0, 1))
	assert.Equal(t, []byte("\xff\xff"), FloatToBytes(1.0, 2))
	assert.Equal(t, []byte("\xff\xff\xff"), FloatToBytes(1.0, 3))
}
func FloatToBytes(f float64, length int) []byte {
	if f < 0 || f > 1 {
		panic(fmt.Sprintf("expected float between (0, 1), got: %f", f))
	}
	result := make([]byte, length)
	result[0] = uint8(f * 255)
	if length > 1 {
		f -= float64(result[0]) / 256
		f *= 256
		result[1] = uint8(f * 255)
	}
	if length > 2 {
		f -= float64(result[1]) / 256
		f *= 256
		result[2] = uint8(f * 255)
	}
	return result
}
