package wal

import (
	"bufio"
	"encoding/binary"
	"hash"
	"io"

	"github.com/coreos/etcd/crc"
	"github.com/coreos/etcd/wal/walpb"
)

type encoder struct {
	bw  *bufio.Writer
	crc hash.Hash32
}

func newEncoder(w io.Writer, prevCrc uint32) *encoder {
	return &encoder{
		bw:  bufio.NewWriter(w),
		crc: crc.New(prevCrc, crcTable),
	}
}

func (e *encoder) encode(rec *walpb.Record) error {
	e.crc.Write(rec.Data)
	rec.Crc = e.crc.Sum32()
	data, err := rec.Marshal()
	if err != nil {
		return err
	}
	if err := writeInt64(e.bw, int64(len(data))); err != nil {
		return err
	}
	_, err = e.bw.Write(data)
	return err
}

func (e *encoder) flush() error {
	return e.bw.Flush()
}

func (e *encoder) buffered() int {
	return e.bw.Buffered()
}

func writeInt64(w io.Writer, n int64) error {
	return binary.Write(w, binary.LittleEndian, n)
}
