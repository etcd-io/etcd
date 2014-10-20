/*
   Copyright 2014 CoreOS, Inc.

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
	"bufio"
	"encoding/binary"
	"hash"
	"io"

	"github.com/coreos/etcd/pkg/crc"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal/walpb"
)

type decoder struct {
	br  *bufio.Reader
	c   io.Closer
	crc hash.Hash32
}

func newDecoder(rc io.ReadCloser) *decoder {
	return &decoder{
		br:  bufio.NewReader(rc),
		c:   rc,
		crc: crc.New(0, crcTable),
	}
}

func (d *decoder) decode(rec *walpb.Record) error {
	rec.Reset()
	l, err := readInt64(d.br)
	if err != nil {
		return err
	}
	data := make([]byte, l)
	if _, err = io.ReadFull(d.br, data); err != nil {
		return err
	}
	if err := rec.Unmarshal(data); err != nil {
		return err
	}
	// skip crc checking if the record type is crcType
	if rec.Type == crcType {
		return nil
	}
	d.crc.Write(rec.Data)
	return rec.Validate(d.crc.Sum32())
}

func (d *decoder) updateCRC(prevCrc uint32) {
	d.crc = crc.New(prevCrc, crcTable)
}

func (d *decoder) lastCRC() uint32 {
	return d.crc.Sum32()
}

func (d *decoder) close() error {
	return d.c.Close()
}

func mustUnmarshalEntry(d []byte) raftpb.Entry {
	var e raftpb.Entry
	pbutil.MustUnmarshal(&e, d)
	return e
}

func mustUnmarshalState(d []byte) raftpb.HardState {
	var s raftpb.HardState
	pbutil.MustUnmarshal(&s, d)
	return s
}

func readInt64(r io.Reader) (int64, error) {
	var n int64
	err := binary.Read(r, binary.LittleEndian, &n)
	return n, err
}
