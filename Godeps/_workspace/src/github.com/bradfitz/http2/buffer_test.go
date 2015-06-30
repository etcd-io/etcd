// Copyright 2014 The Go Authors.
// See https://code.google.com/p/go/source/browse/CONTRIBUTORS
// Licensed under the same terms as Go itself:
// https://code.google.com/p/go/source/browse/LICENSE

package http2

import (
	"io"
	"reflect"
	"testing"
)

var bufferReadTests = []struct {
	buf      buffer
	read, wn int
	werr     error
	wp       []byte
	wbuf     buffer
}{
	{
		buffer{[]byte{'a', 0}, 0, 1, false, nil},
		5, 1, nil, []byte{'a'},
		buffer{[]byte{'a', 0}, 1, 1, false, nil},
	},
	{
		buffer{[]byte{'a', 0}, 0, 1, true, io.EOF},
		5, 1, io.EOF, []byte{'a'},
		buffer{[]byte{'a', 0}, 1, 1, true, io.EOF},
	},
	{
		buffer{[]byte{0, 'a'}, 1, 2, false, nil},
		5, 1, nil, []byte{'a'},
		buffer{[]byte{0, 'a'}, 2, 2, false, nil},
	},
	{
		buffer{[]byte{0, 'a'}, 1, 2, true, io.EOF},
		5, 1, io.EOF, []byte{'a'},
		buffer{[]byte{0, 'a'}, 2, 2, true, io.EOF},
	},
	{
		buffer{[]byte{}, 0, 0, false, nil},
		5, 0, errReadEmpty, []byte{},
		buffer{[]byte{}, 0, 0, false, nil},
	},
	{
		buffer{[]byte{}, 0, 0, true, io.EOF},
		5, 0, io.EOF, []byte{},
		buffer{[]byte{}, 0, 0, true, io.EOF},
	},
}

func TestBufferRead(t *testing.T) {
	for i, tt := range bufferReadTests {
		read := make([]byte, tt.read)
		n, err := tt.buf.Read(read)
		if n != tt.wn {
			t.Errorf("#%d: wn = %d want %d", i, n, tt.wn)
			continue
		}
		if err != tt.werr {
			t.Errorf("#%d: werr = %v want %v", i, err, tt.werr)
			continue
		}
		read = read[:n]
		if !reflect.DeepEqual(read, tt.wp) {
			t.Errorf("#%d: read = %+v want %+v", i, read, tt.wp)
		}
		if !reflect.DeepEqual(tt.buf, tt.wbuf) {
			t.Errorf("#%d: buf = %+v want %+v", i, tt.buf, tt.wbuf)
		}
	}
}
