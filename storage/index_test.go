// Copyright 2015 CoreOS, Inc.
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

package storage

import (
	"reflect"
	"testing"
)

func TestIndexGet(t *testing.T) {
	index := newTreeIndex()
	index.Put([]byte("foo"), revision{main: 2})
	index.Put([]byte("foo"), revision{main: 4})
	index.Tombstone([]byte("foo"), revision{main: 6})

	tests := []struct {
		rev int64

		wrev     revision
		wcreated revision
		wver     int64
		werr     error
	}{
		{0, revision{}, revision{}, 0, ErrRevisionNotFound},
		{1, revision{}, revision{}, 0, ErrRevisionNotFound},
		{2, revision{main: 2}, revision{main: 2}, 1, nil},
		{3, revision{main: 2}, revision{main: 2}, 1, nil},
		{4, revision{main: 4}, revision{main: 2}, 2, nil},
		{5, revision{main: 4}, revision{main: 2}, 2, nil},
		{6, revision{}, revision{}, 0, ErrRevisionNotFound},
	}
	for i, tt := range tests {
		rev, created, ver, err := index.Get([]byte("foo"), tt.rev)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if rev != tt.wrev {
			t.Errorf("#%d: rev = %+v, want %+v", i, rev, tt.wrev)
		}
		if created != tt.wcreated {
			t.Errorf("#%d: created = %+v, want %+v", i, created, tt.wcreated)
		}
		if ver != tt.wver {
			t.Errorf("#%d: ver = %d, want %d", i, ver, tt.wver)
		}
	}
}

func TestIndexRange(t *testing.T) {
	allKeys := [][]byte{[]byte("foo"), []byte("foo1"), []byte("foo2")}
	allRevs := []revision{{main: 1}, {main: 2}, {main: 3}}

	index := newTreeIndex()
	for i := range allKeys {
		index.Put(allKeys[i], allRevs[i])
	}

	atRev := int64(3)
	tests := []struct {
		key, end []byte
		wkeys    [][]byte
		wrevs    []revision
	}{
		// single key that not found
		{
			[]byte("bar"), nil, nil, nil,
		},
		// single key that found
		{
			[]byte("foo"), nil, allKeys[:1], allRevs[:1],
		},
		// range keys, return first member
		{
			[]byte("foo"), []byte("foo1"), allKeys[:1], allRevs[:1],
		},
		// range keys, return first two members
		{
			[]byte("foo"), []byte("foo2"), allKeys[:2], allRevs[:2],
		},
		// range keys, return all members
		{
			[]byte("foo"), []byte("fop"), allKeys, allRevs,
		},
		// range keys, return last two members
		{
			[]byte("foo1"), []byte("fop"), allKeys[1:], allRevs[1:],
		},
		// range keys, return last member
		{
			[]byte("foo2"), []byte("fop"), allKeys[2:], allRevs[2:],
		},
		// range keys, return nothing
		{
			[]byte("foo3"), []byte("fop"), nil, nil,
		},
	}
	for i, tt := range tests {
		keys, revs := index.Range(tt.key, tt.end, atRev)
		if !reflect.DeepEqual(keys, tt.wkeys) {
			t.Errorf("#%d: keys = %+v, want %+v", i, keys, tt.wkeys)
		}
		if !reflect.DeepEqual(revs, tt.wrevs) {
			t.Errorf("#%d: revs = %+v, want %+v", i, revs, tt.wrevs)
		}
	}
}

func TestIndexTombstone(t *testing.T) {
	index := newTreeIndex()
	index.Put([]byte("foo"), revision{main: 1})

	err := index.Tombstone([]byte("foo"), revision{main: 2})
	if err != nil {
		t.Errorf("tombstone error = %v, want nil", err)
	}

	_, _, _, err = index.Get([]byte("foo"), 2)
	if err != ErrRevisionNotFound {
		t.Errorf("get error = %v, want nil", err)
	}
	err = index.Tombstone([]byte("foo"), revision{main: 3})
	if err != ErrRevisionNotFound {
		t.Errorf("tombstone error = %v, want %v", err, ErrRevisionNotFound)
	}
}

func TestIndexRangeEvents(t *testing.T) {
	allKeys := [][]byte{[]byte("foo"), []byte("foo1"), []byte("foo2"), []byte("foo2"), []byte("foo1"), []byte("foo")}
	allRevs := []revision{{main: 1}, {main: 2}, {main: 3}, {main: 4}, {main: 5}, {main: 6}}

	index := newTreeIndex()
	for i := range allKeys {
		index.Put(allKeys[i], allRevs[i])
	}

	atRev := int64(1)
	tests := []struct {
		key, end []byte
		wrevs    []revision
	}{
		// single key that not found
		{
			[]byte("bar"), nil, nil,
		},
		// single key that found
		{
			[]byte("foo"), nil, []revision{{main: 1}, {main: 6}},
		},
		// range keys, return first member
		{
			[]byte("foo"), []byte("foo1"), []revision{{main: 1}, {main: 6}},
		},
		// range keys, return first two members
		{
			[]byte("foo"), []byte("foo2"), []revision{{main: 1}, {main: 2}, {main: 5}, {main: 6}},
		},
		// range keys, return all members
		{
			[]byte("foo"), []byte("fop"), allRevs,
		},
		// range keys, return last two members
		{
			[]byte("foo1"), []byte("fop"), []revision{{main: 2}, {main: 3}, {main: 4}, {main: 5}},
		},
		// range keys, return last member
		{
			[]byte("foo2"), []byte("fop"), []revision{{main: 3}, {main: 4}},
		},
		// range keys, return nothing
		{
			[]byte("foo3"), []byte("fop"), nil,
		},
	}
	for i, tt := range tests {
		revs := index.RangeEvents(tt.key, tt.end, atRev)
		if !reflect.DeepEqual(revs, tt.wrevs) {
			t.Errorf("#%d: revs = %+v, want %+v", i, revs, tt.wrevs)
		}
	}
}

func TestIndexCompact(t *testing.T) {
	maxRev := int64(20)
	tests := []struct {
		key     []byte
		remove  bool
		rev     revision
		created revision
		ver     int64
	}{
		{[]byte("foo"), false, revision{main: 1}, revision{main: 1}, 1},
		{[]byte("foo1"), false, revision{main: 2}, revision{main: 2}, 1},
		{[]byte("foo2"), false, revision{main: 3}, revision{main: 3}, 1},
		{[]byte("foo2"), false, revision{main: 4}, revision{main: 3}, 2},
		{[]byte("foo"), false, revision{main: 5}, revision{main: 1}, 2},
		{[]byte("foo1"), false, revision{main: 6}, revision{main: 2}, 2},
		{[]byte("foo1"), true, revision{main: 7}, revision{}, 0},
		{[]byte("foo2"), true, revision{main: 8}, revision{}, 0},
		{[]byte("foo"), true, revision{main: 9}, revision{}, 0},
		{[]byte("foo"), false, revision{10, 0}, revision{10, 0}, 1},
		{[]byte("foo1"), false, revision{10, 1}, revision{10, 1}, 1},
	}

	// Continuous Compact
	index := newTreeIndex()
	for _, tt := range tests {
		if tt.remove {
			index.Tombstone(tt.key, tt.rev)
		} else {
			index.Put(tt.key, tt.rev)
		}
	}
	for i := int64(1); i < maxRev; i++ {
		am := index.Compact(i)

		windex := newTreeIndex()
		for _, tt := range tests {
			if _, ok := am[tt.rev]; ok || tt.rev.GreaterThan(revision{main: i}) {
				if tt.remove {
					windex.Tombstone(tt.key, tt.rev)
				} else {
					windex.Restore(tt.key, tt.created, tt.rev, tt.ver)
				}
			}
		}
		if !index.Equal(windex) {
			t.Errorf("#%d: not equal index", i)
		}
	}

	// Once Compact
	for i := int64(1); i < maxRev; i++ {
		index := newTreeIndex()
		for _, tt := range tests {
			if tt.remove {
				index.Tombstone(tt.key, tt.rev)
			} else {
				index.Put(tt.key, tt.rev)
			}
		}
		am := index.Compact(i)

		windex := newTreeIndex()
		for _, tt := range tests {
			if _, ok := am[tt.rev]; ok || tt.rev.GreaterThan(revision{main: i}) {
				if tt.remove {
					windex.Tombstone(tt.key, tt.rev)
				} else {
					windex.Restore(tt.key, tt.created, tt.rev, tt.ver)
				}
			}
		}
		if !index.Equal(windex) {
			t.Errorf("#%d: not equal index", i)
		}
	}
}

func TestIndexRestore(t *testing.T) {
	key := []byte("foo")

	tests := []struct {
		created  revision
		modified revision
		ver      int64
	}{
		{revision{1, 0}, revision{1, 0}, 1},
		{revision{1, 0}, revision{1, 1}, 2},
		{revision{1, 0}, revision{2, 0}, 3},
	}

	// Continuous Restore
	index := newTreeIndex()
	for i, tt := range tests {
		index.Restore(key, tt.created, tt.modified, tt.ver)

		modified, created, ver, err := index.Get(key, tt.modified.main)
		if modified != tt.modified {
			t.Errorf("#%d: modified = %v, want %v", i, modified, tt.modified)
		}
		if created != tt.created {
			t.Errorf("#%d: created = %v, want %v", i, created, tt.created)
		}
		if ver != tt.ver {
			t.Errorf("#%d: ver = %d, want %d", i, ver, tt.ver)
		}
		if err != nil {
			t.Errorf("#%d: err = %v, want nil", i, err)
		}
	}

	// Once Restore
	for i, tt := range tests {
		index := newTreeIndex()
		index.Restore(key, tt.created, tt.modified, tt.ver)

		modified, created, ver, err := index.Get(key, tt.modified.main)
		if modified != tt.modified {
			t.Errorf("#%d: modified = %v, want %v", i, modified, tt.modified)
		}
		if created != tt.created {
			t.Errorf("#%d: created = %v, want %v", i, created, tt.created)
		}
		if ver != tt.ver {
			t.Errorf("#%d: ver = %d, want %d", i, ver, tt.ver)
		}
		if err != nil {
			t.Errorf("#%d: err = %v, want nil", i, err)
		}
	}
}
