package storage

import (
	"reflect"
	"testing"
)

func TestKeyIndexGet(t *testing.T) {
	// key:     "foo"
	// index: 12
	// generations:
	//    {empty}
	//    {8[1], 10[2], 12(t)[3]}
	//    {4[2], 6(t)[3]}
	ki := newTestKeyIndex()
	ki.compact(4, make(map[uint64]struct{}))

	tests := []struct {
		index uint64

		windex uint64
		werr   error
	}{
		// expected not exist on an index that is greater than the last tombstone
		{13, 0, ErrIndexNotFound},
		{13, 0, ErrIndexNotFound},

		// get on generation 2
		{12, 12, nil},
		{11, 10, nil},
		{10, 10, nil},
		{9, 8, nil},
		{8, 8, nil},
		{7, 0, ErrIndexNotFound},

		// get on generation 1
		{6, 6, nil},
		{5, 4, nil},
		{4, 4, nil},
	}

	for i, tt := range tests {
		index, err := ki.get(tt.index)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if index != tt.windex {
			t.Errorf("#%d: index = %d, want %d", i, index, tt.index)
		}
	}
}

func TestKeyIndexPut(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.put(5)

	wki := &keyIndex{
		key:         []byte("foo"),
		index:       5,
		generations: []generation{{ver: 1, cont: []uint64{5}}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}

	ki.put(7)

	wki = &keyIndex{
		key:         []byte("foo"),
		index:       7,
		generations: []generation{{ver: 2, cont: []uint64{5, 7}}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}
}

func TestKeyIndexTombstone(t *testing.T) {
	ki := &keyIndex{key: []byte("foo")}
	ki.put(5)

	ki.tombstone(7)

	wki := &keyIndex{
		key:         []byte("foo"),
		index:       7,
		generations: []generation{{ver: 2, cont: []uint64{5, 7}}, {}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}

	ki.put(8)
	ki.put(9)
	ki.tombstone(15)

	wki = &keyIndex{
		key:         []byte("foo"),
		index:       15,
		generations: []generation{{ver: 2, cont: []uint64{5, 7}}, {ver: 3, cont: []uint64{8, 9, 15}}, {}},
	}
	if !reflect.DeepEqual(ki, wki) {
		t.Errorf("ki = %+v, want %+v", ki, wki)
	}
}

func TestKeyIndexCompact(t *testing.T) {
	tests := []struct {
		compact uint64

		wki *keyIndex
		wam map[uint64]struct{}
	}{
		{
			1,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{2, 4, 6}},
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				2: struct{}{}, 4: struct{}{}, 6: struct{}{},
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			2,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{2, 4, 6}},
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				2: struct{}{}, 4: struct{}{}, 6: struct{}{},
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			3,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{2, 4, 6}},
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				2: struct{}{}, 4: struct{}{}, 6: struct{}{},
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			4,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{4, 6}},
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				4: struct{}{}, 6: struct{}{},
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			5,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{4, 6}},
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				4: struct{}{}, 6: struct{}{},
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			6,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{6}},
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				6: struct{}{},
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			7,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			8,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			9,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{8, 10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				8: struct{}{}, 10: struct{}{}, 12: struct{}{},
			},
		},
		{
			10,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				10: struct{}{}, 12: struct{}{},
			},
		},
		{
			11,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{10, 12}},
					{},
				},
			},
			map[uint64]struct{}{
				10: struct{}{}, 12: struct{}{},
			},
		},
		{
			12,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{ver: 3, cont: []uint64{12}},
					{},
				},
			},
			map[uint64]struct{}{
				12: struct{}{},
			},
		},
		{
			13,
			&keyIndex{
				key:   []byte("foo"),
				index: 12,
				generations: []generation{
					{},
				},
			},
			map[uint64]struct{}{},
		},
	}

	// Continous Compaction
	ki := newTestKeyIndex()
	for i, tt := range tests {
		am := make(map[uint64]struct{})
		ki.compact(tt.compact, am)
		if !reflect.DeepEqual(ki, tt.wki) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", am, tt.wam)
		}
	}

	// Jump Compaction
	for i, tt := range tests {
		if (i%2 == 0 && i < 6) && (i%2 == 1 && i > 6) {
			am := make(map[uint64]struct{})
			ki.compact(tt.compact, am)
			if !reflect.DeepEqual(ki, tt.wki) {
				t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
			}
			if !reflect.DeepEqual(am, tt.wam) {
				t.Errorf("#%d: am = %+v, want %+v", am, tt.wam)
			}
		}
	}

	// OnceCompaction
	for i, tt := range tests {
		ki := newTestKeyIndex()
		am := make(map[uint64]struct{})
		ki.compact(tt.compact, am)
		if !reflect.DeepEqual(ki, tt.wki) {
			t.Errorf("#%d: ki = %+v, want %+v", i, ki, tt.wki)
		}
		if !reflect.DeepEqual(am, tt.wam) {
			t.Errorf("#%d: am = %+v, want %+v", am, tt.wam)
		}
	}
}

func newTestKeyIndex() *keyIndex {
	// key:     "foo"
	// index: 12
	// generations:
	//    {empty}
	//    {8[1], 10[2], 12(t)[3]}
	//    {2[1], 4[2], 6(t)[3]}

	ki := &keyIndex{key: []byte("foo")}
	ki.put(2)
	ki.put(4)
	ki.tombstone(6)
	ki.put(8)
	ki.put(10)
	ki.tombstone(12)
	return ki
}
