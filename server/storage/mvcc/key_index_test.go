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

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestKeyIndexGet(t *testing.T) {
	// key: "foo"
	// modified: 16
	// generations:
	//    {empty}
	//    {{14, 0}[1], {14, 1}[2], {16, 0}(t)[3]}
	//    {{8, 0}[1], {10, 0}[2], {12, 0}(t)[3]}
	//    {{2, 0}[1], {4, 0}[2], {6, 0}(t)[3]}
	ki := newTestKeyIndex(zaptest.NewLogger(t))
	ki.compact(zaptest.NewLogger(t), 4, make(map[Revision]struct{}))

	tests := []struct {
		rev int64

		wmod   Revision
		wcreat Revision
		wver   int64
		werr   error
	}{
		{17, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{16, Revision{}, Revision{}, 0, ErrRevisionNotFound},

		// get on generation 3
		{15, Revision{Main: 14, Sub: 1}, Revision{Main: 14}, 2, nil},
		{14, Revision{Main: 14, Sub: 1}, Revision{Main: 14}, 2, nil},

		{13, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{12, Revision{}, Revision{}, 0, ErrRevisionNotFound},

		// get on generation 2
		{11, Revision{Main: 10}, Revision{Main: 8}, 2, nil},
		{10, Revision{Main: 10}, Revision{Main: 8}, 2, nil},
		{9, Revision{Main: 8}, Revision{Main: 8}, 1, nil},
		{8, Revision{Main: 8}, Revision{Main: 8}, 1, nil},

		{7, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{6, Revision{}, Revision{}, 0, ErrRevisionNotFound},

		// get on generation 1
		{5, Revision{Main: 4}, Revision{Main: 2}, 2, nil},
		{4, Revision{Main: 4}, Revision{Main: 2}, 2, nil},

		{3, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{2, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{1, Revision{}, Revision{}, 0, ErrRevisionNotFound},
		{0, Revision{}, Revision{}, 0, ErrRevisionNotFound},
	}

	for i, tt := range tests {
		mod, creat, ver, err := ki.get(zaptest.NewLogger(t), tt.rev)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if mod != tt.wmod {
			t.Errorf("#%d: modified = %+v, want %+v", i, mod, tt.wmod)
		}
		if creat != tt.wcreat {
			t.Errorf("#%d: created = %+v, want %+v", i, creat, tt.wcreat)
		}
		if ver != tt.wver {
			t.Errorf("#%d: version = %d, want %d", i, ver, tt.wver)
		}
	}
}

func TestKeyIndexSince(t *testing.T) {
	ki := newTestKeyIndex(zaptest.NewLogger(t))
	ki.compact(zaptest.NewLogger(t), 4, make(map[Revision]struct{}))

	allRevs := []Revision{
		Revision{Main: 4},
		Revision{Main: 6},
		Revision{Main: 8},
		Revision{Main: 10},
		Revision{Main: 12},
		Revision{Main: 14, Sub: 1},
		Revision{Main: 16},
	}
	tests := []struct {
		rev int64

		wrevs []Revision
	}{
		{17, nil},
		{16, allRevs[6:]},
		{15, allRevs[6:]},
		{14, allRevs[5:]},
		{13, allRevs[5:]},
		{12, allRevs[4:]},
		{11, allRevs[4:]},
		{10, allRevs[3:]},
		{9, allRevs[3:]},
		{8, allRevs[2:]},
		{7, allRevs[2:]},
		{6, allRevs[1:]},
		{5, allRevs[1:]},
		{4, allRevs},
		{3, allRevs},
		{2, allRevs},
		{1, allRevs},
		{0, allRevs},
	}

	for i, tt := range tests {
		revs := ki.since(zaptest.NewLogger(t), tt.rev)
		if !reflect.DeepEqual(revs, tt.wrevs) {
			t.Errorf("#%d: revs = %+v, want %+v", i, revs, tt.wrevs)
		}
	}
}

func TestKeyIndexPut(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.put(zaptest.NewLogger(t), 5, 0)

	wki := &keyIndex{
		key:         []byte("foo"),
		modified:    Revision{Main: 5},
		generations: []generation{{created: Revision{Main: 5}, ver: 1, revs: []Revision{Revision{Main: 5}}}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}

	ki.put(zaptest.NewLogger(t), 7, 0)

	wki = &keyIndex{
		key:         []byte("foo"),
		modified:    Revision{Main: 7},
		generations: []generation{{created: Revision{Main: 5}, ver: 2, revs: []Revision{Revision{Main: 5}, Revision{Main: 7}}}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}
}

func TestKeyIndexRestore(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.restore(zaptest.NewLogger(t), Revision{Main: 5}, Revision{Main: 7}, 2)

	wki := &keyIndex{
		key:         []byte("foo"),
		modified:    Revision{Main: 7},
		generations: []generation{{created: Revision{Main: 5}, ver: 2, revs: []Revision{Revision{Main: 7}}}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}
}

func TestKeyIndexTombstone(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.put(zaptest.NewLogger(t), 5, 0)

	err := ki.tombstone(zaptest.NewLogger(t), 7, 0)
	if err != nil {
		t.Errorf("unexpected tombstone error: %v", err)
	}

	wki := &keyIndex{
		key:         []byte("foo"),
		modified:    Revision{Main: 7},
		generations: []generation{{created: Revision{Main: 5}, ver: 2, revs: []Revision{Revision{Main: 5}, Revision{Main: 7}}}, {}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}

	ki.put(zaptest.NewLogger(t), 8, 0)
	ki.put(zaptest.NewLogger(t), 9, 0)
	err = ki.tombstone(zaptest.NewLogger(t), 15, 0)
	if err != nil {
		t.Errorf("unexpected tombstone error: %v", err)
	}

	wki = &keyIndex{
		key:      []byte("foo"),
		modified: Revision{Main: 15},
		generations: []generation{
			{created: Revision{Main: 5}, ver: 2, revs: []Revision{Revision{Main: 5}, Revision{Main: 7}}},
			{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 9}, Revision{Main: 15}}},
			{},
		},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}

	err = ki.tombstone(zaptest.NewLogger(t), 16, 0)
	if err != ErrRevisionNotFound {
		t.Errorf("tombstone error = %v, want %v", err, ErrRevisionNotFound)
	}
}

func TestKeyIndexCompactAndKeep(t *testing.T) {
	tests := []struct {
		compact int64

		wki *keyIndex
		wam map[Revision]struct{}
	}{
		{
			1,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 2}, ver: 3, revs: []Revision{Revision{Main: 2}, Revision{Main: 4}, Revision{Main: 6}}},
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{},
		},
		{
			2,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 2}, ver: 3, revs: []Revision{Revision{Main: 2}, Revision{Main: 4}, Revision{Main: 6}}},
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 2}: {},
			},
		},
		{
			3,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 2}, ver: 3, revs: []Revision{Revision{Main: 2}, Revision{Main: 4}, Revision{Main: 6}}},
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 2}: {},
			},
		},
		{
			4,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 2}, ver: 3, revs: []Revision{Revision{Main: 4}, Revision{Main: 6}}},
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 4}: {},
			},
		},
		{
			5,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 2}, ver: 3, revs: []Revision{Revision{Main: 4}, Revision{Main: 6}}},
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 4}: {},
			},
		},
		{
			6,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{},
		},
		{
			7,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{},
		},
		{
			8,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 8}: {},
			},
		},
		{
			9,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 8}, Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 8}: {},
			},
		},
		{
			10,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 10}: {},
			},
		},
		{
			11,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 8}, ver: 3, revs: []Revision{Revision{Main: 10}, Revision{Main: 12}}},
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 10}: {},
			},
		},
		{
			12,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{},
		},
		{
			13,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14}, Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{},
		},
		{
			14,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 14, Sub: 1}: {},
			},
		},
		{
			15,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{created: Revision{Main: 14}, ver: 3, revs: []Revision{Revision{Main: 14, Sub: 1}, Revision{Main: 16}}},
					{},
				},
			},
			map[Revision]struct{}{
				Revision{Main: 14, Sub: 1}: {},
			},
		},
		{
			16,
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 16},
				generations: []generation{
					{},
				},
			},
			map[Revision]struct{}{},
		},
	}

	// Continuous Compaction and finding Keep
	ki := newTestKeyIndex(zaptest.NewLogger(t))
	for i, tt := range tests {
		am := make(map[Revision]struct{})
		kiclone := cloneKeyIndex(ki)
		ki.keep(tt.compact, am)
		if !reflect.DeepEqual(ki, kiclone) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, kiclone)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
		}
		am = make(map[Revision]struct{})
		ki.compact(zaptest.NewLogger(t), tt.compact, am)
		if !reflect.DeepEqual(ki, tt.wki) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
		}
	}

	// Jump Compaction and finding Keep
	ki = newTestKeyIndex(zaptest.NewLogger(t))
	for i, tt := range tests {
		if (i%2 == 0 && i < 6) || (i%2 == 1 && i > 6) {
			am := make(map[Revision]struct{})
			kiclone := cloneKeyIndex(ki)
			ki.keep(tt.compact, am)
			if !reflect.DeepEqual(ki, kiclone) {
				t.Errorf("#%d: ki = %+v, want %+v", i, ki, kiclone)
			}
			if !reflect.DeepEqual(am, tt.wam) {
				t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
			}
			am = make(map[Revision]struct{})
			ki.compact(zaptest.NewLogger(t), tt.compact, am)
			if !reflect.DeepEqual(ki, tt.wki) {
				t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
			}
			if !reflect.DeepEqual(am, tt.wam) {
				t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
			}
		}
	}

	kiClone := newTestKeyIndex(zaptest.NewLogger(t))
	// Once Compaction and finding Keep
	for i, tt := range tests {
		ki := newTestKeyIndex(zaptest.NewLogger(t))
		am := make(map[Revision]struct{})
		ki.keep(tt.compact, am)
		if !reflect.DeepEqual(ki, kiClone) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, kiClone)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
		}
		am = make(map[Revision]struct{})
		ki.compact(zaptest.NewLogger(t), tt.compact, am)
		if !reflect.DeepEqual(ki, tt.wki) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
		}
	}
}

func cloneKeyIndex(ki *keyIndex) *keyIndex {
	generations := make([]generation, len(ki.generations))
	for i, gen := range ki.generations {
		generations[i] = *cloneGeneration(&gen)
	}
	return &keyIndex{ki.key, ki.modified, generations}
}

func cloneGeneration(g *generation) *generation {
	if g.revs == nil {
		return &generation{g.ver, g.created, nil}
	}
	tmp := make([]Revision, len(g.revs))
	copy(tmp, g.revs)
	return &generation{g.ver, g.created, tmp}
}

// TestKeyIndexCompactOnFurtherRev tests that compact on version that
// higher than last modified version works well
func TestKeyIndexCompactOnFurtherRev(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.put(zaptest.NewLogger(t), 1, 0)
	ki.put(zaptest.NewLogger(t), 2, 0)
	am := make(map[Revision]struct{})
	ki.compact(zaptest.NewLogger(t), 3, am)

	wki := &keyIndex{
		key:      []byte("foo"),
		modified: Revision{Main: 2},
		generations: []generation{
			{created: Revision{Main: 1}, ver: 2, revs: []Revision{Revision{Main: 2}}},
		},
	}
	wam := map[Revision]struct{}{
		Revision{Main: 2}: {},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}
	if !reflect.DeepEqual(am, wam) {
		t.Errorf("am = %+v, want %+v", am, wam)
	}
}

func TestKeyIndexIsEmpty(t *testing.T) {
	tests := []struct {
		ki *keyIndex
		w  bool
	}{
		{
			&keyIndex{
				key:         []byte("foo"),
				generations: []generation{{}},
			},
			true,
		},
		{
			&keyIndex{
				key:      []byte("foo"),
				modified: Revision{Main: 2},
				generations: []generation{
					{created: Revision{Main: 1}, ver: 2, revs: []Revision{Revision{Main: 2}}},
				},
			},
			false,
		},
	}
	for i, tt := range tests {
		g := tt.ki.isEmpty()
		if g != tt.w {
			t.Errorf("#%d: isEmpty = %v, want %v", i, g, tt.w)
		}
	}
}

func TestKeyIndexFindGeneration(t *testing.T) {
	ki := newTestKeyIndex(zaptest.NewLogger(t))

	tests := []struct {
		rev int64
		wg  *generation
	}{
		{0, nil},
		{1, nil},
		{2, &ki.generations[0]},
		{3, &ki.generations[0]},
		{4, &ki.generations[0]},
		{5, &ki.generations[0]},
		{6, nil},
		{7, nil},
		{8, &ki.generations[1]},
		{9, &ki.generations[1]},
		{10, &ki.generations[1]},
		{11, &ki.generations[1]},
		{12, nil},
		{13, nil},
	}
	for i, tt := range tests {
		g := ki.findGeneration(tt.rev)
		if g != tt.wg {
			t.Errorf("#%d: generation = %+v, want %+v", i, g, tt.wg)
		}
	}
}

func TestKeyIndexLess(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}

	tests := []struct {
		ki *keyIndex
		w  bool
	}{
		{&keyIndex{key: []byte("doo")}, false},
		{&keyIndex{key: []byte("foo")}, false},
		{&keyIndex{key: []byte("goo")}, true},
	}
	for i, tt := range tests {
		g := ki.Less(tt.ki)
		if g != tt.w {
			t.Errorf("#%d: Less = %v, want %v", i, g, tt.w)
		}
	}
}

func TestGenerationIsEmpty(t *testing.T) {
	tests := []struct {
		g *generation
		w bool
	}{
		{nil, true},
		{&generation{}, true},
		{&generation{revs: []Revision{Revision{Main: 1}}}, false},
	}
	for i, tt := range tests {
		g := tt.g.isEmpty()
		if g != tt.w {
			t.Errorf("#%d: isEmpty = %v, want %v", i, g, tt.w)
		}
	}
}

func TestGenerationWalk(t *testing.T) {
	g := &generation{
		ver:     3,
		created: Revision{Main: 2},
		revs:    []Revision{Revision{Main: 2}, Revision{Main: 4}, Revision{Main: 6}},
	}
	tests := []struct {
		f  func(rev Revision) bool
		wi int
	}{
		{func(rev Revision) bool { return rev.Main >= 7 }, 2},
		{func(rev Revision) bool { return rev.Main >= 6 }, 1},
		{func(rev Revision) bool { return rev.Main >= 5 }, 1},
		{func(rev Revision) bool { return rev.Main >= 4 }, 0},
		{func(rev Revision) bool { return rev.Main >= 3 }, 0},
		{func(rev Revision) bool { return rev.Main >= 2 }, -1},
	}
	for i, tt := range tests {
		idx := g.walk(tt.f)
		if idx != tt.wi {
			t.Errorf("#%d: index = %d, want %d", i, idx, tt.wi)
		}
	}
}

func newTestKeyIndex(lg *zap.Logger) *keyIndex {
	// key: "foo"
	// modified: 16
	// generations:
	//    {empty}
	//    {{14, 0}[1], {14, 1}[2], {16, 0}(t)[3]}
	//    {{8, 0}[1], {10, 0}[2], {12, 0}(t)[3]}
	//    {{2, 0}[1], {4, 0}[2], {6, 0}(t)[3]}

	ki := &keyIndex{key: []byte("foo")}
	ki.put(lg, 2, 0)
	ki.put(lg, 4, 0)
	ki.tombstone(lg, 6, 0)
	ki.put(lg, 8, 0)
	ki.put(lg, 10, 0)
	ki.tombstone(lg, 12, 0)
	ki.put(lg, 14, 0)
	ki.put(lg, 14, 1)
	ki.tombstone(lg, 16, 0)
	return ki
}
