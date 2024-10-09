// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wal

import (
	"bytes"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"reflect"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
)

var (
	infoData   = []byte("\b\xef\xfd\x02")
	infoRecord = append([]byte("\x0e\x00\x00\x00\x00\x00\x00\x00\b\x01\x10\x99\xb5\xe4\xd0\x03\x1a\x04"), infoData...)
)

func TestReadRecord(t *testing.T) {
	badInfoRecord := make([]byte, len(infoRecord))
	copy(badInfoRecord, infoRecord)
	badInfoRecord[len(badInfoRecord)-1] = 'a'

	tests := []struct {
		data []byte
		wr   *walpb.Record
		we   error
	}{
		{infoRecord, &walpb.Record{Type: 1, Crc: crc32.Checksum(infoData, crcTable), Data: infoData}, nil},
		{[]byte(""), &walpb.Record{}, io.EOF},
		{infoRecord[:14], &walpb.Record{}, io.ErrUnexpectedEOF},
		{infoRecord[:len(infoRecord)-len(infoData)], &walpb.Record{}, io.ErrUnexpectedEOF},
		{infoRecord[:len(infoRecord)-8], &walpb.Record{}, io.ErrUnexpectedEOF},
		{badInfoRecord, &walpb.Record{}, walpb.ErrCRCMismatch},
	}

	rec := &walpb.Record{}
	for i, tt := range tests {
		buf := bytes.NewBuffer(tt.data)
		f, err := createFileWithData(t, buf)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		decoder := NewDecoder(fileutil.NewFileReader(f))
		e := decoder.Decode(rec)
		if !reflect.DeepEqual(rec, tt.wr) {
			t.Errorf("#%d: block = %v, want %v", i, rec, tt.wr)
		}
		if !errors.Is(e, tt.we) {
			t.Errorf("#%d: err = %v, want %v", i, e, tt.we)
		}
		rec = &walpb.Record{}
	}
}

func TestWriteRecord(t *testing.T) {
	b := &walpb.Record{}
	typ := int64(0xABCD)
	d := []byte("Hello world!")
	buf := new(bytes.Buffer)
	e := newEncoder(buf, 0, 0)
	e.encode(&walpb.Record{Type: typ, Data: d})
	e.flush()
	f, err := createFileWithData(t, buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	decoder := NewDecoder(fileutil.NewFileReader(f))
	err = decoder.Decode(b)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if b.Type != typ {
		t.Errorf("type = %d, want %d", b.Type, typ)
	}
	if !reflect.DeepEqual(b.Data, d) {
		t.Errorf("data = %v, want %v", b.Data, d)
	}
}

func createFileWithData(t *testing.T, bf *bytes.Buffer) (*os.File, error) {
	f, err := os.CreateTemp(t.TempDir(), "wal")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(bf.Bytes()); err != nil {
		return nil, err
	}
	f.Seek(0, 0)
	return f, nil
}
