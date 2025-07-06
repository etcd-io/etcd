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

	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/pkg/v3/adt"

	"go.uber.org/zap"
)

func TestRangePermission(t *testing.T) {
	tests := []struct {
		perms []adt.Interval
		begin []byte
		end   []byte
		want  bool
	}{
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("c")), adt.NewBytesAffineInterval([]byte("x"), []byte("z"))},
			[]byte("a"), []byte("z"),
			false,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("f")), adt.NewBytesAffineInterval([]byte("c"), []byte("d")), adt.NewBytesAffineInterval([]byte("f"), []byte("z"))},
			[]byte("a"), []byte("z"),
			true,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("d")), adt.NewBytesAffineInterval([]byte("a"), []byte("b")), adt.NewBytesAffineInterval([]byte("c"), []byte("f"))},
			[]byte("a"), []byte("f"),
			true,
		},
	}

	for i, tt := range tests {
		readPerms := adt.NewIntervalTree()
		for _, p := range tt.perms {
			readPerms.Insert(p, struct{}{})
		}

		result := checkKeyInterval(zap.NewExample(), &unifiedRangePermissions{readPerms: readPerms}, tt.begin, tt.end, authpb.READ)
		if result != tt.want {
			t.Errorf("#%d: result=%t, want=%t", i, result, tt.want)
		}
	}
}

func TestKeyPermission(t *testing.T) {
	tests := []struct {
		perms []adt.Interval
		key   []byte
		want  bool
	}{
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("c")), adt.NewBytesAffineInterval([]byte("x"), []byte("z"))},
			[]byte("f"),
			false,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("f")), adt.NewBytesAffineInterval([]byte("c"), []byte("d")), adt.NewBytesAffineInterval([]byte("f"), []byte("z"))},
			[]byte("b"),
			true,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("d")), adt.NewBytesAffineInterval([]byte("a"), []byte("b")), adt.NewBytesAffineInterval([]byte("c"), []byte("f"))},
			[]byte("d"),
			true,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("d")), adt.NewBytesAffineInterval([]byte("a"), []byte("b")), adt.NewBytesAffineInterval([]byte("c"), []byte("f"))},
			[]byte("f"),
			false,
		},
	}

	for i, tt := range tests {
		readPerms := adt.NewIntervalTree()
		for _, p := range tt.perms {
			readPerms.Insert(p, struct{}{})
		}

		result := checkKeyPoint(zap.NewExample(), &unifiedRangePermissions{readPerms: readPerms}, tt.key, authpb.READ)
		if result != tt.want {
			t.Errorf("#%d: result=%t, want=%t", i, result, tt.want)
		}
	}
}
