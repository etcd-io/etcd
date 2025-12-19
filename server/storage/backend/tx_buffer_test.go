// Copyright 2023 The etcd Authors
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

package backend

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_bucketBuffer_CopyUsed_After_Add(t *testing.T) {
	bb := &bucketBuffer{buf: make([]kv, 10), used: 0}
	for i := 0; i < 20; i++ {
		k := fmt.Sprintf("key%d", i)
		v := fmt.Sprintf("val%d", i)
		bb.add([]byte(k), []byte(v))
		bbCopy := bb.CopyUsed()
		assert.Equal(t, bb.used, bbCopy.used)
		assert.Len(t, bbCopy.buf, bbCopy.used)
		assert.GreaterOrEqual(t, len(bb.buf), len(bbCopy.buf))
	}
}

func Test_bucketBuffer_CopyUsed(t *testing.T) {
	tests := []struct {
		name       string
		bufLen     int
		used       int
		wantPanic  bool
		wantUsed   int
		wantBufLen int
	}{
		{
			name:       "used is 0",
			bufLen:     10,
			used:       0,
			wantPanic:  false,
			wantUsed:   0,
			wantBufLen: 0,
		},
		{
			name:       "used is greater than 0 and less than len(buf)",
			bufLen:     10,
			used:       5,
			wantPanic:  false,
			wantUsed:   5,
			wantBufLen: 5,
		},
		{
			name:       "used is equal to len(buf)",
			bufLen:     10,
			used:       10,
			wantPanic:  false,
			wantUsed:   10,
			wantBufLen: 10,
		},
		{
			name:      "used is greater than len(buf)",
			bufLen:    10,
			used:      11,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb := &bucketBuffer{buf: make([]kv, tt.bufLen), used: tt.used}
			if tt.wantPanic {
				assert.Panicsf(t, func() {
					bb.CopyUsed()
				}, "expected panic when used (%d) and the length of buf (%d)", tt.used, tt.bufLen)
			} else {
				bbCopy := bb.CopyUsed()
				assert.Equal(t, tt.wantUsed, bbCopy.used)
				assert.Len(t, bbCopy.buf, tt.wantBufLen)
			}
		})
	}
}

func TestDedupe(t *testing.T) {
	tests := []struct {
		name                                   string
		keys, vals, expectedKeys, expectedVals []string
	}{
		{
			name:         "empty",
			keys:         []string{},
			vals:         []string{},
			expectedKeys: []string{},
			expectedVals: []string{},
		},
		{
			name:         "single kv",
			keys:         []string{"key1"},
			vals:         []string{"val1"},
			expectedKeys: []string{"key1"},
			expectedVals: []string{"val1"},
		},
		{
			name:         "duplicate key",
			keys:         []string{"key1", "key1"},
			vals:         []string{"val1", "val2"},
			expectedKeys: []string{"key1"},
			expectedVals: []string{"val2"},
		},
		{
			name:         "unordered keys",
			keys:         []string{"key3", "key1", "key4", "key2", "key1", "key4"},
			vals:         []string{"val1", "val5", "val3", "val4", "val2", "val6"},
			expectedKeys: []string{"key1", "key2", "key3", "key4"},
			expectedVals: []string{"val2", "val4", "val1", "val6"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb := &bucketBuffer{buf: make([]kv, 10), used: 0}
			for i := 0; i < len(tt.keys); i++ {
				bb.add([]byte(tt.keys[i]), []byte(tt.vals[i]))
			}
			bb.dedupe()
			assert.Len(t, tt.expectedKeys, bb.used)
			for i := 0; i < bb.used; i++ {
				assert.Equal(t, bb.buf[i].key, []byte(tt.expectedKeys[i]))
				assert.Equal(t, bb.buf[i].val, []byte(tt.expectedVals[i]))
			}
		})
	}
}

func Test_bucketBuffer_Range(t *testing.T) {
	tests := []struct {
		name                       string
		bb                         func() *bucketBuffer
		key, endKey                []byte
		limit                      int64
		expectedKeys, expectedVals [][]byte
		expectPanicFromRange       bool
		expectPanicFromAdd         bool
	}{
		{
			name:                 "nil bucketBuffer",
			bb:                   func() *bucketBuffer { return nil },
			expectPanicFromRange: true,
		},
		{
			name: "unproperly initialized bucket",
			bb: func() *bucketBuffer {
				b := &bucketBuffer{}
				b.used = 20
				return b
			},
			expectPanicFromAdd: true,
		},
		{
			name:         "blank bucketBuffer return nil keys and values",
			bb:           newBucketBuffer,
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name: "the key does not exist in the bucket",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key1"), []byte("val1"))
				b.add([]byte("key2"), []byte("val2"))
				return b
			},
			key:          []byte("non-existent"),
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name: "key exists right at bucket's used slot",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key1"), []byte("val1"))
				b.add([]byte("key2"), []byte("val2"))
				b.add([]byte("key3"), []byte("val3"))

				b.used = 1
				return b
			},
			key:          []byte("key2"),
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name: "key exists beyond bucket's used slot",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key1"), []byte("val1"))
				b.add([]byte("key2"), []byte("val2"))
				b.add([]byte("key3"), []byte("val3"))

				b.used = 1
				return b
			},
			key:          []byte("key3"),
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name: "single key lookup (no endKey)",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key1"), []byte("val1"))
				b.add([]byte("key2"), []byte("val2"))
				b.add([]byte("key3"), []byte("val3"))
				return b
			},
			key:          []byte("key1"),
			endKey:       []byte(""),
			expectedKeys: [][]byte{[]byte("key1")},
			expectedVals: [][]byte{[]byte("val1")},
		},
		{
			name: "endKey is lower than key",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				return b
			},
			key:          []byte("key4"),
			endKey:       []byte("key3"),
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name: "endKey is lower than key",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				return b
			},
			key:          []byte("key4"),
			endKey:       []byte("key3"),
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name: "endKey is the same as key",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				return b
			},
			key:          []byte("key5"),
			endKey:       []byte("key5"),
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name: "key and endKey are in the range of the bucket",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				b.add([]byte("key7"), []byte("val7"))
				b.add([]byte("key8"), []byte("val8"))
				b.add([]byte("key9"), []byte("val9"))
				return b
			},
			key:    []byte("key5"),
			endKey: []byte("key8"),
			limit:  6,
			expectedKeys: [][]byte{
				[]byte("key5"),
				[]byte("key6"),
				[]byte("key7"), // exclusive of endKey
			},
			expectedVals: [][]byte{
				[]byte("val5"),
				[]byte("val6"),
				[]byte("val7"),
			},
		},
		{
			name: "key and endKey are consecutive",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				b.add([]byte("key7"), []byte("val7"))
				b.add([]byte("key8"), []byte("val8"))
				b.add([]byte("key9"), []byte("val9"))
				return b
			},
			key:    []byte("key5"),
			endKey: []byte("key6"),
			limit:  6,
			expectedKeys: [][]byte{
				[]byte("key5"),
			},
			expectedVals: [][]byte{
				[]byte("val5"),
			},
		},
		{
			name: "endKey is beyond the range of buffer",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				return b
			},
			key:    []byte("key5"),
			endKey: []byte("key9"),
			limit:  3,
			expectedKeys: [][]byte{
				[]byte("key5"),
				[]byte("key6"),
			},
			expectedVals: [][]byte{
				[]byte("val5"),
				[]byte("val6"),
			},
		},
		{
			name: "range is controlled by the limit",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				b.add([]byte("key7"), []byte("val7"))
				return b
			},
			key:    []byte("key5"),
			endKey: []byte("key9"),
			limit:  2,
			expectedKeys: [][]byte{
				[]byte("key5"),
				[]byte("key6"),
			},
			expectedVals: [][]byte{
				[]byte("val5"),
				[]byte("val6"),
			},
		},
		{
			name: "corrupted buffer with the wrong calculated used slot",
			bb: func() *bucketBuffer {
				b := newBucketBuffer()
				b.add([]byte("key4"), []byte("val4"))
				b.add([]byte("key5"), []byte("val5"))
				b.add([]byte("key6"), []byte("val6"))
				b.add([]byte("key7"), []byte("val7"))
				// corrupted buffer
				b.used = 20
				return b
			},
			key:    []byte("key7"),
			endKey: []byte("key9"),
			limit:  5,
			// nil as sort.Search will continue until idx == b.used due to pre-allocated blank values by newBucketBuffer()
			expectedKeys: nil,
			expectedVals: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb := tt.bb()
			if tt.expectPanicFromRange {
				assert.Panics(t, func() {
					bb.Range(tt.key, tt.endKey, tt.limit)
				})
				return
			}
			if tt.expectPanicFromAdd {
				assert.Panics(t, func() {
					bb.add([]byte("key1"), []byte("val1"))
				})
				return
			}
			actualKeys, actualValues := bb.Range(tt.key, tt.endKey, tt.limit)
			assert.Equal(t, tt.expectedKeys, actualKeys)
			assert.Equal(t, tt.expectedVals, actualValues)
		})
	}
}
