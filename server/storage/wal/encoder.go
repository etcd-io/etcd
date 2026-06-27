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
	"encoding/binary"
	"errors"
	"hash"
	"io"
	"os"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"go.etcd.io/etcd/pkg/v3/crc"
	"go.etcd.io/etcd/pkg/v3/ioutil"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
)

// walPageBytes is the alignment for flushing records to the backing Writer.
// It should be a multiple of the minimum sector size so that WAL can safely
// distinguish between torn writes and ordinary data corruption.
const walPageBytes = 8 * minSectorSize

type encoder struct {
	mu sync.Mutex
	bw *ioutil.PageWriter

	crc       hash.Hash32
	buf       []byte
	uint64buf []byte
}

func newEncoder(w io.Writer, prevCrc uint32, pageOffset int) *encoder {
	return &encoder{
		bw:  ioutil.NewPageWriter(w, walPageBytes, pageOffset),
		crc: crc.New(prevCrc, crcTable),
		// 1MB buffer
		buf:       make([]byte, 1024*1024),
		uint64buf: make([]byte, 8),
	}
}

// newFileEncoder creates a new encoder with current file offset for the page writer.
func newFileEncoder(f *os.File, prevCrc uint32) (*encoder, error) {
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	return newEncoder(f, prevCrc, int(offset)), nil
}

func (e *encoder) encode(rec *walpb.Record) error {
	if rec.Type == nil {
		return errors.New("record is missing type")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.crc.Write(rec.Data)
	rec.Crc = new(e.crc.Sum32())
	var (
		data []byte
		err  error
	)

	size := proto.Size(rec)
	opts := proto.MarshalOptions{UseCachedSize: true}
	if size > len(e.buf)-8 {
		raw, err := proto.Marshal(rec)
		if err != nil {
			return err
		}
		data = make([]byte, 8+len(raw))
		copy(data[8:], raw)
	} else {
		data, err = opts.MarshalAppend(e.buf[:8], rec)
		if err != nil {
			return err
		}
	}

	payloadSize := len(data) - 8
	lenField, padBytes := encodeFrameSize(payloadSize)
	if padBytes != 0 {
		data = append(data, make([]byte, padBytes)...)
	}

	binary.LittleEndian.PutUint64(data[0:8], lenField)

	start := time.Now()
	n, err := e.bw.Write(data)
	walWriteSec.Observe(time.Since(start).Seconds())
	walWriteBytes.Add(float64(n))
	return err
}

func encodeFrameSize(dataBytes int) (lenField uint64, padBytes int) {
	lenField = uint64(dataBytes)
	// force 8 byte alignment so length never gets a torn write
	padBytes = (8 - (dataBytes % 8)) % 8
	if padBytes != 0 {
		lenField |= uint64(0x80|padBytes) << 56
	}
	return lenField, padBytes
}

func (e *encoder) flush() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bw.Flush()
}

