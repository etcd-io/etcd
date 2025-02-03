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
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				assert.Errorf(t, err, "Expected failure on parsing uint32 value from %s", tc.s)
			} else {
				require.NoErrorf(t, err, "Unexpected error when parsing %s: %v", tc.s, err)
				assert.Equal(t, tc.expectedVal, uint32(val))
			}
		})
	}
}

func TestUint32FromFlag(t *testing.T) {
	const flagName = "max-concurrent-streams"

	cases := []struct {
		name        string
		defaultVal  uint32
		arguments   []string
		expectedVal uint32
	}{
		{
			name:        "only default value",
			defaultVal:  15,
			arguments:   []string{},
			expectedVal: 15,
		},
		{
			name:        "argument has different value from the default one",
			defaultVal:  16,
			arguments:   []string{"--max-concurrent-streams", "200"},
			expectedVal: 200,
		},
		{
			name:        "argument has the same value from the default one",
			defaultVal:  105,
			arguments:   []string{"--max-concurrent-streams", "105"},
			expectedVal: 105,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("etcd", flag.ContinueOnError)
			fs.Var(NewUint32Value(tc.defaultVal), flagName, "Maximum concurrent streams that each client can open at a time.")
			require.NoError(t, fs.Parse(tc.arguments))
			actualMaxStream := Uint32FromFlag(fs, flagName)
			assert.Equal(t, tc.expectedVal, actualMaxStream)
		})
	}
}
