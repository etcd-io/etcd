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
	"bytes"
	"fmt"
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

		if !bytes.Equal(a[i].begin, b[i].begin) || !bytes.Equal(a[i].end, b[i].end) {
			return false
		}
	}

	return true
}

func TestGetMergedPerms(t *testing.T) {
	tests := []struct {
		params []*rangePerm
		want   []*rangePerm
	}{
		{ // subsets converge
			[]*rangePerm{{[]byte{2}, []byte{3}}, {[]byte{2}, []byte{5}}, {[]byte{1}, []byte{4}}},
			[]*rangePerm{{[]byte{1}, []byte{5}}},
		},
		{ // subsets converge
			[]*rangePerm{{[]byte{0}, []byte{3}}, {[]byte{0}, []byte{1}}, {[]byte{2}, []byte{4}}, {[]byte{0}, []byte{2}}},
			[]*rangePerm{{[]byte{0}, []byte{4}}},
		},
		{ // biggest range at the end
			[]*rangePerm{{[]byte{2}, []byte{3}}, {[]byte{0}, []byte{2}}, {[]byte{1}, []byte{4}}, {[]byte{0}, []byte{5}}},
			[]*rangePerm{{[]byte{0}, []byte{5}}},
		},
		{ // biggest range at the beginning
			[]*rangePerm{{[]byte{0}, []byte{5}}, {[]byte{2}, []byte{3}}, {[]byte{0}, []byte{2}}, {[]byte{1}, []byte{4}}},
			[]*rangePerm{{[]byte{0}, []byte{5}}},
		},
		{ // no overlapping ranges
			[]*rangePerm{{[]byte{2}, []byte{3}}, {[]byte{0}, []byte{1}}, {[]byte{4}, []byte{7}}, {[]byte{8}, []byte{15}}},
			[]*rangePerm{{[]byte{0}, []byte{1}}, {[]byte{2}, []byte{3}}, {[]byte{4}, []byte{7}}, {[]byte{8}, []byte{15}}},
		},
		{
			[]*rangePerm{makePerm("00", "03"), makePerm("18", "19"), makePerm("02", "08"), makePerm("10", "12")},
			[]*rangePerm{makePerm("00", "08"), makePerm("10", "12"), makePerm("18", "19")},
		},
		{
			[]*rangePerm{makePerm("a", "b")},
			[]*rangePerm{makePerm("a", "b")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("b", "")},
			[]*rangePerm{makePerm("a", "b"), makePerm("b", "")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("b", "c")},
			[]*rangePerm{makePerm("a", "c")},
		},
		{
			[]*rangePerm{makePerm("a", "c"), makePerm("b", "d")},
			[]*rangePerm{makePerm("a", "d")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("b", "c"), makePerm("d", "e")},
			[]*rangePerm{makePerm("a", "c"), makePerm("d", "e")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("c", "d"), makePerm("e", "f")},
			[]*rangePerm{makePerm("a", "b"), makePerm("c", "d"), makePerm("e", "f")},
		},
		{
			[]*rangePerm{makePerm("e", "f"), makePerm("c", "d"), makePerm("a", "b")},
			[]*rangePerm{makePerm("a", "b"), makePerm("c", "d"), makePerm("e", "f")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("c", "d"), makePerm("a", "z")},
			[]*rangePerm{makePerm("a", "z")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("c", "d"), makePerm("a", "z"), makePerm("1", "9")},
			[]*rangePerm{makePerm("1", "9"), makePerm("a", "z")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("c", "d"), makePerm("a", "z"), makePerm("1", "a")},
			[]*rangePerm{makePerm("1", "z")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("a", "z"), makePerm("5", "6"), makePerm("1", "9")},
			[]*rangePerm{makePerm("1", "9"), makePerm("a", "z")},
		},
		{
			[]*rangePerm{makePerm("a", "b"), makePerm("b", "c"), makePerm("c", "d"), makePerm("b", "f"), makePerm("1", "9")},
			[]*rangePerm{makePerm("1", "9"), makePerm("a", "f")},
		},
		// overlapping
		{
			[]*rangePerm{makePerm("a", "f"), makePerm("b", "g")},
			[]*rangePerm{makePerm("a", "g")},
		},
		// keys
		{
			[]*rangePerm{makePerm("a", ""), makePerm("b", "")},
			[]*rangePerm{makePerm("a", ""), makePerm("b", "")},
		},
		{
			[]*rangePerm{makePerm("a", ""), makePerm("a", "c")},
			[]*rangePerm{makePerm("a", "c")},
		},
		{
			[]*rangePerm{makePerm("a", ""), makePerm("a", "c"), makePerm("b", "")},
			[]*rangePerm{makePerm("a", "c")},
		},
		{
			[]*rangePerm{makePerm("a", ""), makePerm("b", "c"), makePerm("b", ""), makePerm("c", ""), makePerm("d", "")},
			[]*rangePerm{makePerm("a", ""), makePerm("b", "c"), makePerm("c", ""), makePerm("d", "")},
		},
		// duplicate ranges
		{
			[]*rangePerm{makePerm("a", "f"), makePerm("a", "f")},
			[]*rangePerm{makePerm("a", "f")},
		},
		// duplicate keys
		{
			[]*rangePerm{makePerm("a", ""), makePerm("a", ""), makePerm("a", "")},
			[]*rangePerm{makePerm("a", "")},
		},
	}

	for i, tt := range tests {
		result := mergeRangePerms(tt.params)
		if !isPermsEqual(result, tt.want) {
			t.Errorf("#%d: result=%q, want=%q", i, sprintPerms(result), sprintPerms(tt.want))
		}
	}
}

func makePerm(a, b string) *rangePerm {
	return &rangePerm{begin: []byte(a), end: []byte(b)}
}

func sprintPerms(rs []*rangePerm) (s string) {
	for _, rp := range rs {
		s += fmt.Sprintf("[%s %s] ", rp.begin, rp.end)
	}
	return
}
