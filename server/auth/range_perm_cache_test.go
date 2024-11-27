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

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/pkg/v3/adt"
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
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("d")), adt.NewBytesAffineInterval([]byte("a"), []byte("b")), adt.NewBytesAffineInterval([]byte("c"), []byte("f"))},
			[]byte("a"),
			[]byte{},
			false,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte{})},
			[]byte("a"),
			[]byte{},
			true,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte{0x00}, []byte{})},
			[]byte("a"),
			[]byte{},
			true,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte{0x00}, []byte{})},
			[]byte{0x00},
			[]byte{},
			true,
		},
	}

	for i, tt := range tests {
		readPerms := adt.NewIntervalTree()
		for _, p := range tt.perms {
			readPerms.Insert(p, struct{}{})
		}

		result := checkKeyInterval(zaptest.NewLogger(t), &unifiedRangePermissions{readPerms: readPerms}, tt.begin, tt.end, authpb.READ)
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
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("d")), adt.NewBytesAffineInterval([]byte("a"), []byte("b")), adt.NewBytesAffineInterval([]byte("c"), []byte{})},
			[]byte("f"),
			true,
		},
		{
			[]adt.Interval{adt.NewBytesAffineInterval([]byte("a"), []byte("d")), adt.NewBytesAffineInterval([]byte("a"), []byte("b")), adt.NewBytesAffineInterval([]byte{0x00}, []byte{})},
			[]byte("f"),
			true,
		},
	}

	for i, tt := range tests {
		readPerms := adt.NewIntervalTree()
		for _, p := range tt.perms {
			readPerms.Insert(p, struct{}{})
		}

		result := checkKeyPoint(zaptest.NewLogger(t), &unifiedRangePermissions{readPerms: readPerms}, tt.key, authpb.READ)
		if result != tt.want {
			t.Errorf("#%d: result=%t, want=%t", i, result, tt.want)
		}
	}
}

func TestRangeCheck(t *testing.T) {
	tests := []struct {
		name     string
		key      []byte
		rangeEnd []byte
		want     bool
	}{
		{
			name:     "valid single key",
			key:      []byte("a"),
			rangeEnd: []byte(""),
			want:     true,
		},
		{
			name:     "valid single key",
			key:      []byte("a"),
			rangeEnd: nil,
			want:     true,
		},
		{
			name:     "valid key range, key < rangeEnd",
			key:      []byte("a"),
			rangeEnd: []byte("b"),
			want:     true,
		},
		{
			name:     "invalid empty key range, key == rangeEnd",
			key:      []byte("a"),
			rangeEnd: []byte("a"),
			want:     false,
		},
		{
			name:     "invalid empty key range, key > rangeEnd",
			key:      []byte("b"),
			rangeEnd: []byte("a"),
			want:     false,
		},
		{
			name:     "invalid key, key must not be \"\"",
			key:      []byte(""),
			rangeEnd: []byte("a"),
			want:     false,
		},
		{
			name:     "invalid key range, key must not be \"\"",
			key:      []byte(""),
			rangeEnd: []byte(""),
			want:     false,
		},
		{
			name:     "invalid key range, key must not be \"\"",
			key:      []byte(""),
			rangeEnd: []byte("\x00"),
			want:     false,
		},
		{
			name:     "valid single key (not useful in practice)",
			key:      []byte("\x00"),
			rangeEnd: []byte(""),
			want:     true,
		},
		{
			name:     "valid key range, larger or equals to \"a\"",
			key:      []byte("a"),
			rangeEnd: []byte("\x00"),
			want:     true,
		},
		{
			name:     "valid key range, which includes all keys",
			key:      []byte("\x00"),
			rangeEnd: []byte("\x00"),
			want:     true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPermissionRange(tt.key, tt.rangeEnd)
			if result != tt.want {
				t.Errorf("#%d: result=%t, want=%t", i, result, tt.want)
			}
		})
	}
}
