package storage

import (
	"reflect"
	"testing"
)

func TestIndexPutAndGet(t *testing.T) {
	index := newTestTreeIndex()

	tests := []T{
		{[]byte("foo"), 0, ErrReversionNotFound, 0},
		{[]byte("foo"), 1, nil, 1},
		{[]byte("foo"), 3, nil, 1},
		{[]byte("foo"), 5, nil, 5},
		{[]byte("foo"), 6, nil, 5},

		{[]byte("foo1"), 0, ErrReversionNotFound, 0},
		{[]byte("foo1"), 1, ErrReversionNotFound, 0},
		{[]byte("foo1"), 2, nil, 2},
		{[]byte("foo1"), 5, nil, 2},
		{[]byte("foo1"), 6, nil, 6},

		{[]byte("foo2"), 0, ErrReversionNotFound, 0},
		{[]byte("foo2"), 1, ErrReversionNotFound, 0},
		{[]byte("foo2"), 3, nil, 3},
		{[]byte("foo2"), 4, nil, 4},
		{[]byte("foo2"), 6, nil, 4},
	}
	verify(t, index, tests)
}

func TestIndexRange(t *testing.T) {
	atRev := int64(3)
	allKeys := [][]byte{[]byte("foo"), []byte("foo1"), []byte("foo2")}
	allRevs := []reversion{{main: 1}, {main: 2}, {main: 3}}
	tests := []struct {
		key, end []byte
		wkeys    [][]byte
		wrevs    []reversion
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
		index := newTestTreeIndex()
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
	index := newTestTreeIndex()

	err := index.Tombstone([]byte("foo"), reversion{main: 7})
	if err != nil {
		t.Errorf("tombstone error = %v, want nil", err)
	}
	rev, _, _, err := index.Get([]byte("foo"), 7)
	if err != nil {
		t.Errorf("get error = %v, want nil", err)
	}
	w := reversion{main: 7}
	if !reflect.DeepEqual(rev, w) {
		t.Errorf("get reversion = %+v, want %+v", rev, w)
	}
}

func TestContinuousCompact(t *testing.T) {
	index := newTestTreeIndex()

	tests := []T{
		{[]byte("foo"), 0, ErrReversionNotFound, 0},
		{[]byte("foo"), 1, nil, 1},
		{[]byte("foo"), 3, nil, 1},
		{[]byte("foo"), 5, nil, 5},
		{[]byte("foo"), 6, nil, 5},

		{[]byte("foo1"), 0, ErrReversionNotFound, 0},
		{[]byte("foo1"), 1, ErrReversionNotFound, 0},
		{[]byte("foo1"), 2, nil, 2},
		{[]byte("foo1"), 5, nil, 2},
		{[]byte("foo1"), 6, nil, 6},

		{[]byte("foo2"), 0, ErrReversionNotFound, 0},
		{[]byte("foo2"), 1, ErrReversionNotFound, 0},
		{[]byte("foo2"), 3, nil, 3},
		{[]byte("foo2"), 4, nil, 4},
		{[]byte("foo2"), 6, nil, 4},
	}
	wa := map[reversion]struct{}{
		reversion{main: 1}: struct{}{},
	}
	ga := index.Compact(1)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	wa = map[reversion]struct{}{
		reversion{main: 1}: struct{}{},
		reversion{main: 2}: struct{}{},
	}
	ga = index.Compact(2)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	wa = map[reversion]struct{}{
		reversion{main: 1}: struct{}{},
		reversion{main: 2}: struct{}{},
		reversion{main: 3}: struct{}{},
	}
	ga = index.Compact(3)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	wa = map[reversion]struct{}{
		reversion{main: 1}: struct{}{},
		reversion{main: 2}: struct{}{},
		reversion{main: 4}: struct{}{},
	}
	ga = index.Compact(4)
	delete(wa, reversion{main: 3})
	tests[12] = T{[]byte("foo2"), 3, ErrReversionNotFound, 0}
	if !reflect.DeepEqual(wa, ga) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	wa = map[reversion]struct{}{
		reversion{main: 2}: struct{}{},
		reversion{main: 4}: struct{}{},
		reversion{main: 5}: struct{}{},
	}
	ga = index.Compact(5)
	delete(wa, reversion{main: 1})
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	tests[1] = T{[]byte("foo"), 1, ErrReversionNotFound, 0}
	tests[2] = T{[]byte("foo"), 3, ErrReversionNotFound, 0}
	verify(t, index, tests)

	wa = map[reversion]struct{}{
		reversion{main: 4}: struct{}{},
		reversion{main: 5}: struct{}{},
		reversion{main: 6}: struct{}{},
	}
	ga = index.Compact(6)
	delete(wa, reversion{main: 2})
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	tests[7] = T{[]byte("foo1"), 2, ErrReversionNotFound, 0}
	tests[8] = T{[]byte("foo1"), 5, ErrReversionNotFound, 0}
	verify(t, index, tests)
}

func verify(t *testing.T, index index, tests []T) {
	for i, tt := range tests {
		h, _, _, err := index.Get(tt.key, tt.rev)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if h.main != tt.wrev {
			t.Errorf("#%d: rev = %d, want %d", i, h.main, tt.wrev)
		}
	}
}

type T struct {
	key []byte
	rev int64

	werr error
	wrev int64
}

func newTestTreeIndex() index {
	index := newTreeIndex()
	index.Put([]byte("foo"), reversion{main: 1})
	index.Put([]byte("foo1"), reversion{main: 2})
	index.Put([]byte("foo2"), reversion{main: 3})
	index.Put([]byte("foo2"), reversion{main: 4})
	index.Put([]byte("foo"), reversion{main: 5})
	index.Put([]byte("foo1"), reversion{main: 6})
	return index
}
