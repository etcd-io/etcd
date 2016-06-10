// Copyright 2016 The etcd Authors
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

package auth

import (
	"testing"
)

func isPermsEqual(a, b []*rangePerm) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if len(b) <= i {
			return false
		}

		if a[i].begin != b[i].begin || a[i].end != b[i].end {
			return false
		}
	}

	return true
}

func TestUnifyParams(t *testing.T) {
	tests := []struct {
		params []*rangePerm
		want   []*rangePerm
	}{
		{
			[]*rangePerm{{"a", "b"}},
			[]*rangePerm{{"a", "b"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"b", "c"}},
			[]*rangePerm{{"a", "c"}},
		},
		{
			[]*rangePerm{{"a", "c"}, {"b", "d"}},
			[]*rangePerm{{"a", "d"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"b", "c"}, {"d", "e"}},
			[]*rangePerm{{"a", "c"}, {"d", "e"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"c", "d"}, {"e", "f"}},
			[]*rangePerm{{"a", "b"}, {"c", "d"}, {"e", "f"}},
		},
		{
			[]*rangePerm{{"e", "f"}, {"c", "d"}, {"a", "b"}},
			[]*rangePerm{{"a", "b"}, {"c", "d"}, {"e", "f"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"c", "d"}, {"a", "z"}},
			[]*rangePerm{{"a", "z"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"c", "d"}, {"a", "z"}, {"1", "9"}},
			[]*rangePerm{{"1", "9"}, {"a", "z"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"c", "d"}, {"a", "z"}, {"1", "a"}},
			[]*rangePerm{{"1", "z"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"a", "z"}, {"5", "6"}, {"1", "9"}},
			[]*rangePerm{{"1", "9"}, {"a", "z"}},
		},
		{
			[]*rangePerm{{"a", "b"}, {"b", "c"}, {"c", "d"}, {"d", "f"}, {"1", "9"}},
			[]*rangePerm{{"1", "9"}, {"a", "f"}},
		},
	}

	for i, tt := range tests {
		result := mergeRangePerms(tt.params)
		if !isPermsEqual(result, tt.want) {
			t.Errorf("#%d: result=%q, want=%q", i, result, tt.want)
		}
	}
}
