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
		assert.Equal(t, bbCopy.used, len(bbCopy.buf))
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
				assert.Equal(t, tt.wantBufLen, len(bbCopy.buf))
			}
		})
	}
}
