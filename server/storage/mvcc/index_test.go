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
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestIndexGet(t *testing.T) {
	ti := newTreeIndex(zaptest.NewLogger(t))
	ti.Put([]byte("foo"), Revision{Main: 2})
	ti.Put([]byte("foo"), Revision{Main: 4})
	ti.Tombstone([]byte("foo"), Revision{Main: 6})

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
		{3, Revision{Main: 2}, Revision{Main: 2}, 1, nil},
		{4, Revision{Main: 4}, Revision{Main: 2}, 2, nil},
		{5, Revision{Main: 4}, Revision{Main: 2}, 2, nil},
		{6, Revision{}, Revision{}, 0, ErrRevisionNotFound},
	}
	for i, tt := range tests {
		rev, created, ver, err := ti.Get([]byte("foo"), tt.rev)
		if !errors.Is(err, tt.werr) {
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
	allRevs := []Revision{{Main: 1}, {Main: 2}, {Main: 3}}

	ti := newTreeIndex(zaptest.NewLogger(t))
	for i := range allKeys {
		ti.Put(allKeys[i], allRevs[i])
	}

	atRev := int64(3)
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
			t.Errorf("#%d: keys = %+v, want %+v", i, keys, tt.wkeys)
		}
		if !reflect.DeepEqual(revs, tt.wrevs) {
			t.Errorf("#%d: revs = %+v, want %+v", i, revs, tt.wrevs)
		}
	}
}

func TestIndexTombstone(t *testing.T) {
	ti := newTreeIndex(zaptest.NewLogger(t))
	ti.Put([]byte("foo"), Revision{Main: 1})

	err := ti.Tombstone([]byte("foo"), Revision{Main: 2})
	if err != nil {
		t.Errorf("tombstone error = %v, want nil", err)
	}

	_, _, _, err = ti.Get([]byte("foo"), 2)
	if !errors.Is(err, ErrRevisionNotFound) {
		t.Errorf("get error = %v, want ErrRevisionNotFound", err)
	}
	err = ti.Tombstone([]byte("foo"), Revision{Main: 3})
	if !errors.Is(err, ErrRevisionNotFound) {
		t.Errorf("tombstone error = %v, want %v", err, ErrRevisionNotFound)
	}
}

func TestIndexRevision(t *testing.T) {
	allKeys := [][]byte{[]byte("foo"), []byte("foo1"), []byte("foo2"), []byte("foo2"), []byte("foo1"), []byte("foo")}
	allRevs := []Revision{{Main: 1}, {Main: 2}, {Main: 3}, {Main: 4}, {Main: 5}, {Main: 6}}

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
			[]byte("foo"), nil, 6, 0, []Revision{{Main: 6}}, 1,
		},
		// various range keys, fixed atRev, unlimited
		{
			[]byte("foo"), []byte("foo1"), 6, 0, []Revision{{Main: 6}}, 1,
		},
		{
			[]byte("foo"), []byte("foo2"), 6, 0, []Revision{{Main: 6}, {Main: 5}}, 2,
		},
		{
			[]byte("foo"), []byte("fop"), 6, 0, []Revision{{Main: 6}, {Main: 5}, {Main: 4}}, 3,
		},
		{
			[]byte("foo1"), []byte("fop"), 6, 0, []Revision{{Main: 5}, {Main: 4}}, 2,
		},
		{
			[]byte("foo2"), []byte("fop"), 6, 0, []Revision{{Main: 4}}, 1,
		},
		{
			[]byte("foo3"), []byte("fop"), 6, 0, nil, 0,
		},
		// fixed range keys, various atRev, unlimited
		{
			[]byte("foo1"), []byte("fop"), 1, 0, nil, 0,
		},
		{
			[]byte("foo1"), []byte("fop"), 2, 0, []Revision{{Main: 2}}, 1,
		},
		{
			[]byte("foo1"), []byte("fop"), 3, 0, []Revision{{Main: 2}, {Main: 3}}, 2,
		},
		{
			[]byte("foo1"), []byte("fop"), 4, 0, []Revision{{Main: 2}, {Main: 4}}, 2,
		},
		{
			[]byte("foo1"), []byte("fop"), 5, 0, []Revision{{Main: 5}, {Main: 4}}, 2,
		},
		{
			[]byte("foo1"), []byte("fop"), 6, 0, []Revision{{Main: 5}, {Main: 4}}, 2,
		},
		// fixed range keys, fixed atRev, various limit
		{
			[]byte("foo"), []byte("fop"), 6, 1, []Revision{{Main: 6}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 6, 2, []Revision{{Main: 6}, {Main: 5}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 6, 3, []Revision{{Main: 6}, {Main: 5}, {Main: 4}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 3, 1, []Revision{{Main: 1}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 3, 2, []Revision{{Main: 1}, {Main: 2}}, 3,
		},
		{
			[]byte("foo"), []byte("fop"), 3, 3, []Revision{{Main: 1}, {Main: 2}, {Main: 3}}, 3,
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

func TestIndexCompactAndKeep(t *testing.T) {
	maxRev := int64(20)

	// key: "foo"
	// modified: 10
	// generations:
	//	{{10, 0}}
	//	{{1, 0}, {5, 0}, {9, 0}(t)}
	//
	// key: "foo1"
	// modified: 10, 1
	// generations:
	//	{{10, 1}}
	//	{{2, 0}, {6, 0}, {7, 0}(t)}
	//
	// key: "foo2"
	// modified: 8
	// generations:
	//	{empty}
	//	{{3, 0}, {4, 0}, {8, 0}(t)}
	//
	buildTreeIndex := func() index {
		ti := newTreeIndex(zaptest.NewLogger(t))

		ti.Put([]byte("foo"), Revision{Main: 1})
		ti.Put([]byte("foo1"), Revision{Main: 2})
		ti.Put([]byte("foo2"), Revision{Main: 3})
		ti.Put([]byte("foo2"), Revision{Main: 4})
		ti.Put([]byte("foo"), Revision{Main: 5})
		ti.Put([]byte("foo1"), Revision{Main: 6})
		require.NoError(t, ti.Tombstone([]byte("foo1"), Revision{Main: 7}))
		require.NoError(t, ti.Tombstone([]byte("foo2"), Revision{Main: 8}))
		require.NoError(t, ti.Tombstone([]byte("foo"), Revision{Main: 9}))
		ti.Put([]byte("foo"), Revision{Main: 10})
		ti.Put([]byte("foo1"), Revision{Main: 10, Sub: 1})
		return ti
	}

	afterCompacts := []struct {
		atRev      int
		keyIndexes []keyIndex
		keep       map[Revision]struct{}
		compacted  map[Revision]struct{}
	}{
		{
			atRev: 1,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 1}, {Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 3, created: Revision{Main: 2}, revs: []Revision{{Main: 2}, {Main: 6}, {Main: 7}}},
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 3}, {Main: 4}, {Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 1}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 1}: {},
			},
		},
		{
			atRev: 2,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 1}, {Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 3, created: Revision{Main: 2}, revs: []Revision{{Main: 2}, {Main: 6}, {Main: 7}}},
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 3}, {Main: 4}, {Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 1}: {},
				{Main: 2}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 1}: {},
				{Main: 2}: {},
			},
		},
		{
			atRev: 3,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 1}, {Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 3, created: Revision{Main: 2}, revs: []Revision{{Main: 2}, {Main: 6}, {Main: 7}}},
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 3}, {Main: 4}, {Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 1}: {},
				{Main: 2}: {},
				{Main: 3}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 1}: {},
				{Main: 2}: {},
				{Main: 3}: {},
			},
		},
		{
			atRev: 4,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 1}, {Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 3, created: Revision{Main: 2}, revs: []Revision{{Main: 2}, {Main: 6}, {Main: 7}}},
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 4}, {Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 1}: {},
				{Main: 2}: {},
				{Main: 4}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 1}: {},
				{Main: 2}: {},
				{Main: 4}: {},
			},
		},
		{
			atRev: 5,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 3, created: Revision{Main: 2}, revs: []Revision{{Main: 2}, {Main: 6}, {Main: 7}}},
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 4}, {Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 2}: {},
				{Main: 4}: {},
				{Main: 5}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 2}: {},
				{Main: 4}: {},
				{Main: 5}: {},
			},
		},
		{
			atRev: 6,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 3, created: Revision{Main: 2}, revs: []Revision{{Main: 6}, {Main: 7}}},
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 4}, {Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 6}: {},
				{Main: 4}: {},
				{Main: 5}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 6}: {},
				{Main: 4}: {},
				{Main: 5}: {},
			},
		},
		{
			atRev: 7,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 3, created: Revision{Main: 2}, revs: []Revision{{Main: 7}}},
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 4}, {Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 4}: {},
				{Main: 5}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 7}: {},
				{Main: 4}: {},
				{Main: 5}: {},
			},
		},
		{
			atRev: 8,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 5}, {Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
				{
					key:      []byte("foo2"),
					modified: Revision{Main: 8},
					generations: []generation{
						{ver: 3, created: Revision{Main: 3}, revs: []Revision{{Main: 8}}},
						{},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 5}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 8}: {},
				{Main: 5}: {},
			},
		},
		{
			atRev: 9,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 3, created: Revision{Main: 1}, revs: []Revision{{Main: 9}}},
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
			},
			keep: map[Revision]struct{}{},
			compacted: map[Revision]struct{}{
				{Main: 9}: {},
			},
		},
		{
			atRev: 10,
			keyIndexes: []keyIndex{
				{
					key:      []byte("foo"),
					modified: Revision{Main: 10},
					generations: []generation{
						{ver: 1, created: Revision{Main: 10}, revs: []Revision{{Main: 10}}},
					},
				},
				{
					key:      []byte("foo1"),
					modified: Revision{Main: 10, Sub: 1},
					generations: []generation{
						{ver: 1, created: Revision{Main: 10, Sub: 1}, revs: []Revision{{Main: 10, Sub: 1}}},
					},
				},
			},
			keep: map[Revision]struct{}{
				{Main: 10}:         {},
				{Main: 10, Sub: 1}: {},
			},
			compacted: map[Revision]struct{}{
				{Main: 10}:         {},
				{Main: 10, Sub: 1}: {},
			},
		},
	}

	ti := buildTreeIndex()
	// Continuous Compact and Keep
	for i := int64(1); i < maxRev; i++ {
		j := i - 1
		if i >= int64(len(afterCompacts)) {
			j = int64(len(afterCompacts)) - 1
		}

		am := ti.Compact(i)
		require.Equalf(t, afterCompacts[j].compacted, am, "#%d: compact(%d) != expected", i, i)

		keep := ti.Keep(i)
		require.Equalf(t, afterCompacts[j].keep, keep, "#%d: keep(%d) != expected", i, i)

		nti := newTreeIndex(zaptest.NewLogger(t)).(*treeIndex)
		for k := range afterCompacts[j].keyIndexes {
			ki := afterCompacts[j].keyIndexes[k]
			nti.tree.ReplaceOrInsert(&ki)
		}
		require.Truef(t, ti.Equal(nti), "#%d: not equal ti", i)
	}

	// Once Compact and Keep
	for i := int64(1); i < maxRev; i++ {
		ti := buildTreeIndex()

		j := i - 1
		if i >= int64(len(afterCompacts)) {
			j = int64(len(afterCompacts)) - 1
		}

		am := ti.Compact(i)
		require.Equalf(t, afterCompacts[j].compacted, am, "#%d: compact(%d) != expected", i, i)

		keep := ti.Keep(i)
		require.Equalf(t, afterCompacts[j].keep, keep, "#%d: keep(%d) != expected", i, i)

		nti := newTreeIndex(zaptest.NewLogger(t)).(*treeIndex)
		for k := range afterCompacts[j].keyIndexes {
			ki := afterCompacts[j].keyIndexes[k]
			nti.tree.ReplaceOrInsert(&ki)
		}

		require.Truef(t, ti.Equal(nti), "#%d: not equal ti", i)
	}
}
