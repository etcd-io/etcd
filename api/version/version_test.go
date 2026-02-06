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

package version

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
)

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		name                   string
		ver1                   semver.Version
		ver2                   semver.Version
		expectedCompareResult  int
		expectedLessThanResult bool
		expectedEqualResult    bool
	}{
		{
			name:                   "ver1 should be great than ver2",
			ver1:                   V3_5,
			ver2:                   V3_4,
			expectedCompareResult:  1,
			expectedLessThanResult: false,
			expectedEqualResult:    false,
		},
		{
			name:                   "ver1(4.0) should be great than ver2",
			ver1:                   V4_0,
			ver2:                   V3_7,
			expectedCompareResult:  1,
			expectedLessThanResult: false,
			expectedEqualResult:    false,
		},
		{
			name:                   "ver1 should be less than ver2",
			ver1:                   V3_5,
			ver2:                   V3_6,
			expectedCompareResult:  -1,
			expectedLessThanResult: true,
			expectedEqualResult:    false,
		},
		{
			name:                   "ver1 should be less than ver2 (4.0)",
			ver1:                   V3_5,
			ver2:                   V4_0,
			expectedCompareResult:  -1,
			expectedLessThanResult: true,
			expectedEqualResult:    false,
		},
		{
			name:                   "ver1 should be equal to ver2",
			ver1:                   V3_5,
			ver2:                   V3_5,
			expectedCompareResult:  0,
			expectedLessThanResult: false,
			expectedEqualResult:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compareResult := Compare(tc.ver1, tc.ver2)
			lessThanResult := LessThan(tc.ver1, tc.ver2)
			equalResult := Equal(tc.ver1, tc.ver2)

			assert.Equal(t, tc.expectedCompareResult, compareResult)
			assert.Equal(t, tc.expectedLessThanResult, lessThanResult)
			assert.Equal(t, tc.expectedEqualResult, equalResult)
		})
	}
}
