package storage

import (
	"bytes"
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
		rev      int64

		wrev int64
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
		kvs, rev, err := s.Range(tt.key, tt.end, 0, tt.rev)
		if err != nil {
			t.Fatal(err)
		}
		if len(kvs) != int(tt.wN) {
			t.Errorf("#%d: len(kvs) = %d, want %d", i, len(kvs), tt.wN)
		}
		if rev != tt.wrev {
			t.Errorf("#%d: rev = %d, want %d", i, tt.rev, tt.wrev)
		}
	}
}

func TestSimpleDeleteRange(t *testing.T) {
	tests := []struct {
		key, end []byte

		wrev int64
		wN   int64
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

		n, rev := s.DeleteRange(tt.key, tt.end)
		if n != tt.wN {
			t.Errorf("#%d: n = %d, want %d", i, n, tt.wN)
		}
		if rev != tt.wrev {
			t.Errorf("#%d: rev = %d, wang %d", i, rev, tt.wrev)
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
	n, rev := s.DeleteRange([]byte("foo"), nil)
	if n != 1 || rev != 4 {
		t.Fatalf("n = %d, index = %d, want (%d, %d)", n, rev, 1, 4)
	}

	// before removal foo
	kvs, rev, err := s.Range([]byte("foo"), []byte("foo3"), 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(kvs) != 3 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 3)
	}

	// after removal foo
	kvs, rev, err = s.Range([]byte("foo"), []byte("foo3"), 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(kvs) != 2 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 2)
	}

	// remove again -> expect nothing
	n, rev = s.DeleteRange([]byte("foo"), nil)
	if n != 0 || rev != 4 {
		t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 0, 4)
	}

	// remove foo1
	n, rev = s.DeleteRange([]byte("foo"), []byte("foo2"))
	if n != 1 || rev != 5 {
		t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 1, 5)
	}

	// after removal foo1
	kvs, rev, err = s.Range([]byte("foo"), []byte("foo3"), 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(kvs) != 1 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 1)
	}

	// remove foo2
	n, rev = s.DeleteRange([]byte("foo2"), []byte("foo3"))
	if n != 1 || rev != 6 {
		t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 1, 6)
	}

	// after removal foo2
	kvs, rev, err = s.Range([]byte("foo"), []byte("foo3"), 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(kvs) != 0 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 0)
	}
}

func TestOneTnx(t *testing.T) {
	s := newStore("test")
	defer os.Remove("test")

	id := s.TnxBegin()
	for i := 0; i < 3; i++ {
		s.TnxPut(id, []byte("foo"), []byte("bar"))
		s.TnxPut(id, []byte("foo1"), []byte("bar1"))
		s.TnxPut(id, []byte("foo2"), []byte("bar2"))

		// remove foo
		n, rev, err := s.TnxDeleteRange(id, []byte("foo"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || rev != 1 {
			t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 1, 1)
		}

		kvs, rev, err := s.TnxRange(id, []byte("foo"), []byte("foo3"), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(kvs) != 2 {
			t.Fatalf("len(kvs) = %d, want %d", len(kvs), 2)
		}

		// remove again -> expect nothing
		n, rev, err = s.TnxDeleteRange(id, []byte("foo"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 || rev != 1 {
			t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 0, 1)
		}

		// remove foo1
		n, rev, err = s.TnxDeleteRange(id, []byte("foo"), []byte("foo2"))
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || rev != 1 {
			t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 1, 1)
		}

		// after removal foo1
		kvs, rev, err = s.TnxRange(id, []byte("foo"), []byte("foo3"), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(kvs) != 1 {
			t.Fatalf("len(kvs) = %d, want %d", len(kvs), 1)
		}

		// remove foo2
		n, rev, err = s.TnxDeleteRange(id, []byte("foo2"), []byte("foo3"))
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || rev != 1 {
			t.Fatalf("n = %d, rev = %d, want (%d, %d)", n, rev, 1, 1)
		}

		// after removal foo2
		kvs, rev, err = s.TnxRange(id, []byte("foo"), []byte("foo3"), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(kvs) != 0 {
			t.Fatalf("len(kvs) = %d, want %d", len(kvs), 0)
		}
	}
	err := s.TnxEnd(id)
	if err != nil {
		t.Fatal(err)
	}

	// After tnx
	kvs, rev, err := s.Range([]byte("foo"), []byte("foo3"), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(kvs) != 0 {
		t.Fatalf("len(kvs) = %d, want %d", len(kvs), 0)
	}
	if rev != 1 {
		t.Fatalf("rev = %d, want %d", rev, 1)
	}
}

func TestCompaction(t *testing.T) {
	s := newStore("test")
	defer os.Remove("test")

	s.Put([]byte("foo"), []byte("bar"))
	s.Put([]byte("foo1"), []byte("bar1"))
	s.Put([]byte("foo2"), []byte("bar2"))
	s.Put([]byte("foo"), []byte("bar11"))
	s.Put([]byte("foo1"), []byte("bar12"))
	s.Put([]byte("foo2"), []byte("bar13"))
	s.Put([]byte("foo1"), []byte("bar14"))
	s.DeleteRange([]byte("foo"), []byte("foo200"))
	s.Put([]byte("foo4"), []byte("bar4"))

	err := s.Compact(4)
	if err != nil {
		t.Errorf("unexpect compact error %v", err)
	}

	err = s.Compact(4)
	if err != ErrCompacted {
		t.Errorf("err = %v, want %v", err, ErrCompacted)
	}

	_, _, err = s.Range([]byte("foo"), nil, 0, 4)
	if err != ErrCompacted {
		t.Errorf("err = %v, want %v", err, ErrCompacted)
	}

	// compact should not compact the last value of foo
	kvs, rev, err := s.Range([]byte("foo"), nil, 0, 5)
	if err != nil {
		t.Errorf("unexpected range error %v", err)
	}
	if !bytes.Equal(kvs[0].Value, []byte("bar11")) {
		t.Errorf("value = %s, want %s", string(kvs[0].Value), "bar11")
	}
	if rev != 5 {
		t.Errorf("rev = %d, want %d", rev, 5)
	}

	// compact everything
	err = s.Compact(8)
	if err != nil {
		t.Errorf("unexpect compact error %v", err)
	}

	kvs, rev, err = s.Range([]byte("foo"), []byte("fop"), 0, 0)
	if err != nil {
		t.Errorf("unexpected range error %v", err)
	}
	if len(kvs) != 1 {
		t.Errorf("len(kvs) = %d, want %d", len(kvs), 1)
	}
	if !bytes.Equal(kvs[0].Value, []byte("bar4")) {
		t.Errorf("value = %s, want %s", string(kvs[0].Value), "bar4")
	}
	if rev != 9 {
		t.Errorf("rev = %d, want %d", rev, 9)
	}
}

// TODO: test more complicated cases:
// with unfinished compaction
// with removed keys
func TestRestore(t *testing.T) {
	s0 := newStore("test")
	defer os.Remove("test")

	s0.Put([]byte("foo"), []byte("bar"))
	s0.Put([]byte("foo1"), []byte("bar1"))
	s0.Put([]byte("foo2"), []byte("bar2"))
	s0.Put([]byte("foo"), []byte("bar11"))
	s0.Put([]byte("foo1"), []byte("bar12"))
	s0.Put([]byte("foo2"), []byte("bar13"))
	s0.Put([]byte("foo1"), []byte("bar14"))

	s0.Close()

	s1 := newStore("test")
	s1.Restore()

	if !s0.Equal(s1) {
		t.Errorf("not equal!")
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
