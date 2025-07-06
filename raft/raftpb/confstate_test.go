// Copyright 2019 The etcd Authors
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

package raftpb

import (
	"testing"
)

func TestConfState_Equivalent(t *testing.T) {
	type testCase struct {
		cs, cs2 ConfState
		ok      bool
	}

	testCases := []testCase{
		// Reordered voters and learners.
		{ConfState{
			Voters:         []uint64{1, 2, 3},
			Learners:       []uint64{5, 4, 6},
			VotersOutgoing: []uint64{9, 8, 7},
			LearnersNext:   []uint64{10, 20, 15},
		}, ConfState{
			Voters:         []uint64{1, 2, 3},
			Learners:       []uint64{4, 5, 6},
			VotersOutgoing: []uint64{7, 9, 8},
			LearnersNext:   []uint64{20, 10, 15},
		}, true},
		// Not sensitive to nil vs empty slice.
		{ConfState{Voters: []uint64{}}, ConfState{Voters: []uint64(nil)}, true},
		// Non-equivalent voters.
		{ConfState{Voters: []uint64{1, 2, 3, 4}}, ConfState{Voters: []uint64{2, 1, 3}}, false},
		{ConfState{Voters: []uint64{1, 4, 3}}, ConfState{Voters: []uint64{2, 1, 3}}, false},
		// Non-equivalent learners.
		{ConfState{Voters: []uint64{1, 2, 3, 4}}, ConfState{Voters: []uint64{2, 1, 3}}, false},
		// Sensitive to AutoLeave flag.
		{ConfState{AutoLeave: true}, ConfState{}, false},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			if err := tc.cs.Equivalent(tc.cs2); (err == nil) != tc.ok {
				t.Fatalf("wanted error: %t, got:\n%s", tc.ok, err)
			}
		})
	}
}
