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

	"github.com/coreos/etcd/auth/authpb"
	"github.com/coreos/etcd/pkg/adt"
)

func TestRangePermission(t *testing.T) {
	tests := []struct {
		perms []adt.Interval
		begin string
		end   string
		want  bool
	}{
		{
			[]adt.Interval{adt.NewStringInterval("a", "c"), adt.NewStringInterval("x", "z")},
			"a", "z",
			false,
		},
		{
			[]adt.Interval{adt.NewStringInterval("a", "f"), adt.NewStringInterval("c", "d"), adt.NewStringInterval("f", "z")},
			"a", "z",
			true,
		},
		{
			[]adt.Interval{adt.NewStringInterval("a", "d"), adt.NewStringInterval("a", "b"), adt.NewStringInterval("c", "f")},
			"a", "f",
			true,
		},
	}

	for i, tt := range tests {
		readPerms := &adt.IntervalTree{}
		for _, p := range tt.perms {
			readPerms.Insert(p, struct{}{})
		}

		result := checkKeyInterval(&unifiedRangePermissions{readPerms: readPerms}, tt.begin, tt.end, authpb.READ)
		if result != tt.want {
			t.Errorf("#%d: result=%t, want=%t", i, result, tt.want)
		}
	}
}
