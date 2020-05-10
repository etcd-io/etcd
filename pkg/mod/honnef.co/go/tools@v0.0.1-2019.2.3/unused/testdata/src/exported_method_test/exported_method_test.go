package pkg

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

type countReadSeeker struct {
	io.ReadSeeker
	N int64
}

func (rs *countReadSeeker) Read(buf []byte) (int, error) {
	n, err := rs.ReadSeeker.Read(buf)
	rs.N += int64(n)
	return n, err
}

func TestFoo(t *testing.T) {
	r := bytes.NewReader([]byte("Hello, world!"))
	cr := &countReadSeeker{ReadSeeker: r}
	ioutil.ReadAll(cr)
	if cr.N != 13 {
		t.Errorf("got %d, want 13", cr.N)
	}
}

var sink int

func BenchmarkFoo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sink = fn()
	}
}

func fn() int { return 0 }
