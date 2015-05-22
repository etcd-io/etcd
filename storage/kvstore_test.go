package storage

import (
	"crypto/rand"
	"os"
	"testing"
)

func TestRange(t *testing.T) {
	s := newStore("test")
	defer os.Remove("test")

	s.Put([]byte("foo"), []byte("bar"))
	s.Put([]byte("foo1"), []byte("bar1"))
	s.Put([]byte("foo2"), []byte("bar2"))

	tests := []struct {
		key, end []byte
		index    int64

		windex int64
		// TODO: change this to the actual kv
		wN int64
	}{
		{
			[]byte("foo"), []byte("foo3"), 0,
			3, 3,
		},
		{
			[]byte("foo"), []byte("foo1"), 0,
			3, 1,
		},
		{
			[]byte("foo"), []byte("foo3"), 1,
			1, 1,
		},
		{
			[]byte("foo"), []byte("foo3"), 2,
			2, 2,
		},
	}

	for i, tt := range tests {
		kvs, index := s.Range(tt.key, tt.end, 0, tt.index)
		if len(kvs) != int(tt.wN) {
			t.Errorf("#%d: len(kvs) = %d, want %d", i, len(kvs), tt.wN)
		}
		if index != tt.windex {
			t.Errorf("#%d: index = %d, wang %d", i, tt.index, tt.windex)
		}
	}
}

func TestSimpleDeleteRange(t *testing.T) {
	tests := []struct {
		key, end []byte

		windex int64
		wN     int64
	}{
		{
			[]byte("foo"), []byte("foo1"),
			4, 1,
		},
		{
			[]byte("foo"), []byte("foo2"),
			4, 2,
		},
		{
			[]byte("foo"), []byte("foo3"),
			4, 3,
		},
		{
			[]byte("foo3"), []byte("foo8"),
			3, 0,
		},
	}

	for i, tt := range tests {
		s := newStore("test")

		s.Put([]byte("foo"), []byte("bar"))
		s.Put([]byte("foo1"), []byte("bar1"))
		s.Put([]byte("foo2"), []byte("bar2"))

		n, index := s.DeleteRange(tt.key, tt.end)
		if n != tt.wN {
			t.Errorf("#%d: n = %d, want %d", i, n, tt.wN)
		}
		if index != tt.windex {
			t.Errorf("#%d: index = %d, wang %d", i, index, tt.windex)
		}

		os.Remove("test")
	}
}

func TestRangeInSequence(t *testing.T) {
	s := newStore("test")
	defer os.Remove("test")

	s.Put([]byte("foo"), []byte("bar"))
	s.Put([]byte("foo1"), []byte("bar1"))
	s.Put([]byte("foo2"), []byte("bar2"))

	// remove foo
	n, index := s.DeleteRange([]byte("foo"), nil)
	if n != 1 || index != 4 {
		t.Fatalf("n = %d, index = %d, want (%d, %d)", n, index, 1, 4)
	}

	// before removal foo
	kvs, index := s.Range([]byte("foo"), []byte("foo3"), 0, 3)
	if len(kvs) != 3 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 3)
	}

	// after removal foo
	kvs, index = s.Range([]byte("foo"), []byte("foo3"), 0, 4)
	if len(kvs) != 2 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 2)
	}

	// remove again -> expect nothing
	n, index = s.DeleteRange([]byte("foo"), nil)
	if n != 0 || index != 4 {
		t.Fatalf("n = %d, index = %d, want (%d, %d)", n, index, 0, 4)
	}

	// remove foo1
	n, index = s.DeleteRange([]byte("foo"), []byte("foo2"))
	if n != 1 || index != 5 {
		t.Fatalf("n = %d, index = %d, want (%d, %d)", n, index, 1, 5)
	}

	// after removal foo1
	kvs, index = s.Range([]byte("foo"), []byte("foo3"), 0, 5)
	if len(kvs) != 1 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 1)
	}

	// remove foo2
	n, index = s.DeleteRange([]byte("foo2"), []byte("foo3"))
	if n != 1 || index != 6 {
		t.Fatalf("n = %d, index = %d, want (%d, %d)", n, index, 1, 6)
	}

	// after removal foo2
	kvs, index = s.Range([]byte("foo"), []byte("foo3"), 0, 6)
	if len(kvs) != 0 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 0)
	}
}

func BenchmarkStorePut(b *testing.B) {
	s := newStore("test")
	defer os.Remove("test")

	// prepare keys
	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = make([]byte, 64)
		rand.Read(keys[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[i], []byte("foo"))
	}
}
