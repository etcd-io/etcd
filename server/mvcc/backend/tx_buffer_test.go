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
	"testing"

	"github.com/stretchr/testify/assert"
)

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
