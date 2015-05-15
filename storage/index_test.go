package storage

import (
	"reflect"
	"testing"
)

func TestIndexPutAndGet(t *testing.T) {
	index := newTestTreeIndex()

	tests := []T{
		{[]byte("foo"), 0, ErrIndexNotFound, 0},
		{[]byte("foo"), 1, nil, 1},
		{[]byte("foo"), 3, nil, 1},
		{[]byte("foo"), 5, nil, 5},
		{[]byte("foo"), 6, nil, 5},

		{[]byte("foo1"), 0, ErrIndexNotFound, 0},
		{[]byte("foo1"), 1, ErrIndexNotFound, 0},
		{[]byte("foo1"), 2, nil, 2},
		{[]byte("foo1"), 5, nil, 2},
		{[]byte("foo1"), 6, nil, 6},

		{[]byte("foo2"), 0, ErrIndexNotFound, 0},
		{[]byte("foo2"), 1, ErrIndexNotFound, 0},
		{[]byte("foo2"), 3, nil, 3},
		{[]byte("foo2"), 4, nil, 4},
		{[]byte("foo2"), 6, nil, 4},
	}
	verify(t, index, tests)
}

func TestContinuousCompact(t *testing.T) {
	index := newTestTreeIndex()

	tests := []T{
		{[]byte("foo"), 0, ErrIndexNotFound, 0},
		{[]byte("foo"), 1, nil, 1},
		{[]byte("foo"), 3, nil, 1},
		{[]byte("foo"), 5, nil, 5},
		{[]byte("foo"), 6, nil, 5},

		{[]byte("foo1"), 0, ErrIndexNotFound, 0},
		{[]byte("foo1"), 1, ErrIndexNotFound, 0},
		{[]byte("foo1"), 2, nil, 2},
		{[]byte("foo1"), 5, nil, 2},
		{[]byte("foo1"), 6, nil, 6},

		{[]byte("foo2"), 0, ErrIndexNotFound, 0},
		{[]byte("foo2"), 1, ErrIndexNotFound, 0},
		{[]byte("foo2"), 3, nil, 3},
		{[]byte("foo2"), 4, nil, 4},
		{[]byte("foo2"), 6, nil, 4},
	}
	wa := map[uint64]struct{}{
		1: struct{}{},
		2: struct{}{},
		3: struct{}{},
		4: struct{}{},
		5: struct{}{},
		6: struct{}{},
	}
	ga := index.Compact(1)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	ga = index.Compact(2)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	ga = index.Compact(3)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	ga = index.Compact(4)
	delete(wa, 3)
	tests[12] = T{[]byte("foo2"), 3, ErrIndexNotFound, 0}
	if !reflect.DeepEqual(wa, ga) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	verify(t, index, tests)

	ga = index.Compact(5)
	delete(wa, 1)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	tests[1] = T{[]byte("foo"), 1, ErrIndexNotFound, 0}
	tests[2] = T{[]byte("foo"), 3, ErrIndexNotFound, 0}
	verify(t, index, tests)

	ga = index.Compact(6)
	delete(wa, 2)
	if !reflect.DeepEqual(ga, wa) {
		t.Errorf("a = %v, want %v", ga, wa)
	}
	tests[7] = T{[]byte("foo1"), 2, ErrIndexNotFound, 0}
	tests[8] = T{[]byte("foo1"), 5, ErrIndexNotFound, 0}
	verify(t, index, tests)
}

func verify(t *testing.T, index index, tests []T) {
	for i, tt := range tests {
		h, err := index.Get(tt.key, tt.index)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if h != tt.windex {
			t.Errorf("#%d: index = %d, want %d", i, h, tt.windex)
		}
	}
}

type T struct {
	key   []byte
	index uint64

	werr   error
	windex uint64
}

func newTestTreeIndex() index {
	index := newTreeIndex()
	index.Put([]byte("foo"), 1)
	index.Put([]byte("foo1"), 2)
	index.Put([]byte("foo2"), 3)
	index.Put([]byte("foo2"), 4)
	index.Put([]byte("foo"), 5)
	index.Put([]byte("foo1"), 6)
	return index
}
