/*
Copyright 2014 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
		{infoBlock, &block{1, infoData}, nil},
		{[]byte(""), &block{}, io.EOF},
		{infoBlock[:len(infoBlock)-len(infoData)-8], &block{}, io.ErrUnexpectedEOF},
		{infoBlock[:len(infoBlock)-len(infoData)], &block{}, io.ErrUnexpectedEOF},
		{infoBlock[:len(infoBlock)-8], &block{}, io.ErrUnexpectedEOF},
	}

	b := &block{}
	for i, tt := range tests {
		buf := bytes.NewBuffer(tt.data)
		e := readBlock(buf, b)
		if !reflect.DeepEqual(b, tt.wb) {
			t.Errorf("#%d: block = %v, want %v", i, b, tt.wb)
		}
		if !reflect.DeepEqual(e, tt.we) {
			t.Errorf("#%d: err = %v, want %v", i, e, tt.we)
		}
		b = &block{}
	}
}

func TestWriteBlock(t *testing.T) {
	b := &block{}
	typ := int64(0xABCD)
	d := []byte("Hello world!")
	buf := new(bytes.Buffer)
	writeBlock(buf, typ, d)
	err := readBlock(buf, b)
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
