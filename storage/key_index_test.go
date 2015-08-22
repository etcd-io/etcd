package storage

import (
	"reflect"
	"testing"
)

func TestKeyIndexGet(t *testing.T) {
	// key:     "foo"
	// rev: 12
	// generations:
	//    {empty}
	//    {8[1], 10[2], 12(t)[3]}
	//    {4[2], 6(t)[3]}
	ki := newTestKeyIndex()
	ki.compact(4, make(map[revision]struct{}))

	tests := []struct {
		rev int64

		wrev int64
		werr error
	}{
		{13, 12, nil},
		{13, 12, nil},

		// get on generation 2
		{12, 12, nil},
		{11, 10, nil},
		{10, 10, nil},
		{9, 8, nil},
		{8, 8, nil},
		{7, 6, nil},

		// get on generation 1
		{6, 6, nil},
		{5, 4, nil},
		{4, 4, nil},
	}

	for i, tt := range tests {
		rev, _, _, err := ki.get(tt.rev)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if rev.main != tt.wrev {
			t.Errorf("#%d: rev = %d, want %d", i, rev.main, tt.rev)
		}
	}
}

func TestKeyIndexPut(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.put(5, 0)

	wki := &keyIndex{
		key:         []byte("foo"),
		modified:    revision{5, 0},
		generations: []generation{{created: revision{5, 0}, ver: 1, revs: []revision{{main: 5}}}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}

	ki.put(7, 0)

	wki = &keyIndex{
		key:         []byte("foo"),
		modified:    revision{7, 0},
		generations: []generation{{created: revision{5, 0}, ver: 2, revs: []revision{{main: 5}, {main: 7}}}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}
}

func TestKeyIndexTombstone(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.put(5, 0)

	ki.tombstone(7, 0)

	wki := &keyIndex{
		key:         []byte("foo"),
		modified:    revision{7, 0},
		generations: []generation{{created: revision{5, 0}, ver: 2, revs: []revision{{main: 5}, {main: 7}}}, {}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}

	ki.put(8, 0)
	ki.put(9, 0)
	ki.tombstone(15, 0)

	wki = &keyIndex{
		key:      []byte("foo"),
		modified: revision{15, 0},
		generations: []generation{
			{created: revision{5, 0}, ver: 2, revs: []revision{{main: 5}, {main: 7}}},
			{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 9}, {main: 15}}},
			{},
		},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}
}

func TestKeyIndexCompact(t *testing.T) {
	tests := []struct {
		compact int64

		wki *keyIndex
		wam map[revision]struct{}
	}{
		{
			1,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{2, 0}, ver: 3, revs: []revision{{main: 2}, {main: 4}, {main: 6}}},
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{},
		},
		{
			2,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{2, 0}, ver: 3, revs: []revision{{main: 2}, {main: 4}, {main: 6}}},
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 2}: {},
			},
		},
		{
			3,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{2, 0}, ver: 3, revs: []revision{{main: 2}, {main: 4}, {main: 6}}},
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 2}: {},
			},
		},
		{
			4,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{2, 0}, ver: 3, revs: []revision{{main: 4}, {main: 6}}},
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 4}: {},
			},
		},
		{
			5,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{2, 0}, ver: 3, revs: []revision{{main: 4}, {main: 6}}},
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 4}: {},
			},
		},
		{
			6,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{},
		},
		{
			7,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{},
		},
		{
			8,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 8}: {},
			},
		},
		{
			9,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 8}, {main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 8}: {},
			},
		},
		{
			10,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 10}: {},
			},
		},
		{
			11,
			&keyIndex{
				key:      []byte("foo"),
				modified: revision{12, 0},
				generations: []generation{
					{created: revision{8, 0}, ver: 3, revs: []revision{{main: 10}, {main: 12}}},
					{},
				},
			},
			map[revision]struct{}{
				revision{main: 10}: {},
			},
		},
		{
			12,
			&keyIndex{
				key:         []byte("foo"),
				modified:    revision{12, 0},
				generations: []generation{{}},
			},
			map[revision]struct{}{},
		},
	}

	// Continous Compaction
	ki := newTestKeyIndex()
	for i, tt := range tests {
		am := make(map[revision]struct{})
		ki.compact(tt.compact, am)
		if !reflect.DeepEqual(ki, tt.wki) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
		}
	}

	// Jump Compaction
	for i, tt := range tests {
		if (i%2 == 0 && i < 6) || (i%2 == 1 && i > 6) {
			am := make(map[revision]struct{})
			ki.compact(tt.compact, am)
			if !reflect.DeepEqual(ki, tt.wki) {
				t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
			}
			if !reflect.DeepEqual(am, tt.wam) {
				t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
			}
		}
	}

	// OnceCompaction
	for i, tt := range tests {
		ki := newTestKeyIndex()
		am := make(map[revision]struct{})
		ki.compact(tt.compact, am)
		if !reflect.DeepEqual(ki, tt.wki) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", i, am, tt.wam)
		}
	}
}

func newTestKeyIndex() *keyIndex {
	// key:     "foo"
	// rev: 12
	// generations:
	//    {empty}
	//    {8[1], 10[2], 12(t)[3]}
	//    {2[1], 4[2], 6(t)[3]}

	ki := &keyIndex{key: []byte("foo")}
	ki.put(2, 0)
	ki.put(4, 0)
	ki.tombstone(6, 0)
	ki.put(8, 0)
	ki.put(10, 0)
	ki.tombstone(12, 0)
	return ki
}
