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

package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint32Value(t *testing.T) {
	cases := []struct {
		name        string
		s           string
		expectedVal uint32
		expectError bool
	}{
		{
			name:        "normal uint32 value",
			s:           "200",
			expectedVal: 200,
		},
		{
			name:        "zero value",
			s:           "0",
			expectedVal: 0,
		},
		{
			name:        "negative int value",
			s:           "-200",
			expectError: true,
		},
		{
			name:        "invalid integer value",
			s:           "invalid",
			expectError: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var val uint32Value
			err := val.Set(tc.s)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected failure on parsing uint32 value from %s", tc.s)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error when parsing %s: %v", tc.s, err)
				}
				assert.Equal(t, uint32(val), tc.expectedVal)
			}
		})
	}
}
