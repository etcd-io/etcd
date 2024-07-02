// Copyright 2015 The etcd Authors
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

package mvcc

import (
	"reflect"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestIndexGet(t *testing.T) {
	ti := newTreeIndex(zaptest.NewLogger(t))
	ti.Put([]byte("foo"), Revision{Main: 2})
	ti.Put([]byte("foo"), Revision{Main: 3})
	ti.Tombstone([]byte("foo"), Revision{Main: 4})

	tests := []struct {
		rev int64

		wrev     Revision
		wcreated Revision
		wver     int64
		werr     error
	}{
		{0, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{1, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{2, Revision{Main: 2}, Revision{Main: 2}, 1, nil},
		{3, Revision{Main: 3}, Revision{Main: 2}, 2, nil},
		{4, Revision{}, Revision{}, 0, ErrRevisionNotFound},
	}
	for i, tt := range tests {
		rev, created, ver, err := ti.Get([]byte("foo"), tt.rev)
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
	allRevs := []Revision{Revision{Main: 2}, Revision{Main: 3}, Revision{Main: 4}}

	ti := newTreeIndex(zaptest.NewLogger(t))
	for i := range allKeys {
		ti.Put(allKeys[i], allRevs[i])
	}

	atRev := int64(4)
	tests := []struct {
		key, end []byte
		wkeys    [][]byte
		wrevs    []Revision
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
		keys, revs := ti.Range(tt.key, tt.end, atRev)
		if !reflect.DeepEqual(keys, tt.wkeys) {
			t.Errorf("#%d: keys = %s, want %s", i, keys, tt.wkeys)
		}
		if !reflect.DeepEqual(revs, tt.wrevs) {
			t.Errorf("#%d: revs = %+v, want %+v", i, revs, tt.wrevs)
		}
	}
}

func TestIndexTombstone(t *testing.T) {
	ti := newTreeIndex(zaptest.NewLogger(t))
	ti.Put([]byte("foo"), Revision{Main: 2})

	err := ti.Tombstone([]byte("foo"), Revision{Main: 3})
	if err != nil {
		t.Errorf("tombstone error = %v, want nil", err)
	}

	_, _, _, err = ti.Get([]byte("foo"), 3)
	if err != ErrRevisionNotFound {
		t.Errorf("get error = %v, want ErrRevisionNotFound", err)
	}
	err = ti.Tombstone([]byte("foo"), Revision{Main: 4})
	if err != ErrRevisionNotFound {
		t.Errorf("tombstone error = %v, want %v", err, ErrRevisionNotFound)
	}
}

func TestIndexRevision(t *testing.T) {
	allKeys := [][]byte{[]byte("foo"), []byte("foo1"), []byte("foo2"), []byte("foo2"), []byte("foo1"), []byte("foo")}
	allRevs := []Revision{Revision{Main: 1}, Revision{Main: 2}, Revision{Main: 3}, Revision{Main: 4}, Revision{Main: 5}, Revision{Main: 6}}

	ti := newTreeIndex(zaptest.NewLogger(t))
	for i := range allKeys {
		ti.Put(allKeys[i], allRevs[i])
	}

	tests := []struct {
		key, end []byte
		atRev    int64
		limit    int
		wrevs    []Revision
		wcounts  int
	}{
		// single key that not found
		{
			[]byte("bar"), nil, 6, 0, nil, 0,
		},
		// single key that found
		{
			[]byte("foo"), nil, 6, 0, []Revision{Revision{Main: 6}}, 1,
		},
		// various range keys, fixed atRev, unlimited
		{
			[]byte("foo"), []byte("foo1"), 6, 0, []Revision{Revision{Main: 6}}, 1,
		},
		{
			[]byte("foo"), []byte("foo2"), 6, 0, []Revision{Revision{Main: 6}, Revision{Main: 5}}, 2,
		},
		{
			[]byte("foo"), []byte("fop"), 6, 0, []Revision{Revision{Main: 6}, Revision{Main: 5}, Revision{Main: 4}}, 3,
		},
		{
			[]byte("foo1"), []byte("fop"), 6, 0, []Revision{Revision{Main: 5}, Revision{Main: 4}}, 2,
		},
		{
			[]byte("foo2"), []byte("fop"), 6, 0, []Revision{Revision{Main: 4}}, 1,
		},
		{
			[]byte("foo3"), []byte("fop"), 6, 0, nil, 0,
		},
		// fixed range keys, various atRev, unlimited
		{
			[]byte("foo1"), []byte("fop"), 1, 0, nil, 0,
		},
		{
			[]byte("foo1"), []byte("fop"), 2, 0, []Revision{Revision{Main: 2}}, 1,
		},
		{
			[]byte("foo1"), []byte("fop"), 3, 0, []Revision{Revision{Main: 2}, Revision{Main: 3}}, 2,
		},
		{
			[]byte("foo1"), []byte("fop"), 4, 0, []Revision{Revision{Main: 2}, Revision{Main: 4}}, 2,
		},
		{
			[]byte("foo1"), []byte("fop"), 5, 0, []Revision{Revision{Main: 5}, Revision{Main: 4}}, 2,
		},
		{
			[]byte("foo1"), []byte("fop"), 6, 0, []Revision{Revision{Main: 5}, Revision{Main: 4}}, 2,
		},
		// fixed range keys, fixed atRev, various limit
		{
			[]byte("foo"), []byte("fop"), 6, 1, []Revision{Revision{Main: 6}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 6, 2, []Revision{Revision{Main: 6}, Revision{Main: 5}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 6, 3, []Revision{Revision{Main: 6}, Revision{Main: 5}, Revision{Main: 4}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 3, 1, []Revision{Revision{Main: 1}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 3, 2, []Revision{Revision{Main: 1}, Revision{Main: 2}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 3, 3, []Revision{Revision{Main: 1}, Revision{Main: 2}, Revision{Main: 3}}, 3,
		},
	}
	for i, tt := range tests {
		revs, _ := ti.Revisions(tt.key, tt.end, tt.atRev, tt.limit)
		if !reflect.DeepEqual(revs, tt.wrevs) {
			t.Errorf("#%d limit %d: revs = %+v, want %+v", i, tt.limit, revs, tt.wrevs)
		}
		count := ti.CountRevisions(tt.key, tt.end, tt.atRev)
		if count != tt.wcounts {
			t.Errorf("#%d: count = %d, want %v", i, count, tt.wcounts)
		}
	}
}

//func TestIndexCompactAndKeep(t *testing.T) {
//	maxRev := int64(20)
//	tests := []struct {
//		key     []byte
//		remove  bool
//		rev     Revision
//		created Revision
//		ver     int64
//	}{
//		{[]byte("foo"), false, Revision{Main: 1}, Revision{Main: 1}, 1},
//		{[]byte("foo1"), false, Revision{Main: 2}, Revision{Main: 2}, 1},
//		{[]byte("foo2"), false, Revision{Main: 3}, Revision{Main: 3}, 1},
//		{[]byte("foo2"), false, Revision{Main: 4}, Revision{Main: 3}, 2},
//		{[]byte("foo"), false, Revision{Main: 5}, Revision{Main: 1}, 2},
//		{[]byte("foo1"), false, Revision{Main: 6}, Revision{Main: 2}, 2},
//		{[]byte("foo1"), true, Revision{Main: 7}, Revision{}, 0},
//		{[]byte("foo2"), true, Revision{Main: 8}, Revision{}, 0},
//		{[]byte("foo"), true, Revision{Main: 9}, Revision{}, 0},
//		{[]byte("foo"), false, Revision{Main: 10}, Revision{Main: 10}, 1},
//		{[]byte("foo1"), false, Revision{Main: 10, Sub: 1}, Revision{Main: 10, Sub: 1}, 1},
//	}
//
//	// Continuous Compact and Keep
//	ti := newTreeIndex(zaptest.NewLogger(t))
//	for _, tt := range tests {
//		if tt.remove {
//			ti.Tombstone(tt.key, tt.rev)
//		} else {
//			ti.Put(tt.key, tt.rev)
//		}
//	}
//	for i := int64(1); i < maxRev; i++ {
//		am := ti.Compact(i)
//		keep := ti.Keep(i)
//		if !(reflect.DeepEqual(am, keep)) {
//			t.Errorf("#%d: compact keep %v != Keep keep %v", i, am, keep)
//		}
//		wti := &treeIndex{tree: btree.NewG(32, func(aki *keyIndex, bki *keyIndex) bool {
//			return aki.Less(bki)
//		})}
//		for _, tt := range tests {
//			if _, ok := am[tt.rev]; ok || tt.rev.GreaterThan(Revision{Main: i}) {
//				if tt.remove {
//					wti.Tombstone(tt.key, tt.rev)
//				} else {
//					restore(wti, tt.key, tt.created, tt.rev, tt.ver)
//				}
//			}
//		}
//		if !ti.Equal(wti) {
//			t.Errorf("#%d: not equal ti", i)
//		}
//	}
//
//	// Once Compact and Keep
//	for i := int64(1); i < maxRev; i++ {
//		ti := newTreeIndex(zaptest.NewLogger(t))
//		for _, tt := range tests {
//			if tt.remove {
//				ti.Tombstone(tt.key, tt.rev)
//			} else {
//				ti.Put(tt.key, tt.rev)
//			}
//		}
//		am := ti.Compact(i)
//		keep := ti.Keep(i)
//		if !(reflect.DeepEqual(am, keep)) {
//			t.Errorf("#%d: compact keep %v != Keep keep %v", i, am, keep)
//		}
//		wti := &treeIndex{tree: btree.NewG(32, func(aki *keyIndex, bki *keyIndex) bool {
//			return aki.Less(bki)
//		})}
//		for _, tt := range tests {
//			if _, ok := am[tt.rev]; ok || tt.rev.GreaterThan(Revision{Main: i}) {
//				if tt.remove {
//					wti.Tombstone(tt.key, tt.rev)
//				} else {
//					restore(wti, tt.key, tt.created, tt.rev, tt.ver)
//				}
//			}
//		}
//		if !ti.Equal(wti) {
//			t.Errorf("#%d: not equal ti", i)
//		}
//	}
//}
//
//func restore(ti *treeIndex, key []byte, created, modified Revision, ver int64) {
//	keyi := &keyIndex{key: key}
//
//	okeyi, _ := ti.tree.Get(keyi)
//	if okeyi == nil {
//		keyi.restore(ti.lg, created, modified, ver)
//		ti.tree.ReplaceOrInsert(keyi)
//		return
//	}
//	okeyi.put(ti.lg, modified.Main, modified.Sub)
//}
