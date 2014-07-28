package wal

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestReadBlock(t *testing.T) {
	tests := []struct {
		data []byte
		wb   *block
		we   error
	}{
		{infoBlock, &block{1, 8, infoData}, nil},
		{[]byte(""), nil, io.EOF},
		{infoBlock[:len(infoBlock)-len(infoData)-8], nil, io.ErrUnexpectedEOF},
		{infoBlock[:len(infoBlock)-len(infoData)], nil, io.ErrUnexpectedEOF},
		{infoBlock[:len(infoBlock)-8], nil, io.ErrUnexpectedEOF},
	}

	for i, tt := range tests {
		buf := bytes.NewBuffer(tt.data)
		b, e := readBlock(buf)
		if !reflect.DeepEqual(b, tt.wb) {
			t.Errorf("#%d: block = %v, want %v", i, b, tt.wb)
		}
		if !reflect.DeepEqual(e, tt.we) {
			t.Errorf("#%d: err = %v, want %v", i, e, tt.we)
		}
	}
}

func TestWriteBlock(t *testing.T) {
	typ := int64(0xABCD)
	d := []byte("Hello world!")
	buf := new(bytes.Buffer)
	writeBlock(buf, typ, d)
	b, err := readBlock(buf)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if b.t != typ {
		t.Errorf("type = %d, want %d", b.t, typ)
	}
	if !reflect.DeepEqual(b.d, d) {
		t.Errorf("data = %v, want %v", b.d, d)
	}
}
