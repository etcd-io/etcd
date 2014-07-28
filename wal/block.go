package wal

import (
	"fmt"
	"io"
)

type block struct {
	t int64
	l int64
	d []byte
}

func writeBlock(w io.Writer, t int64, d []byte) error {
	if err := writeInt64(w, t); err != nil {
		return err
	}
	if err := writeInt64(w, int64(len(d))); err != nil {
		return err
	}
	_, err := w.Write(d)
	return err
}

func readBlock(r io.Reader) (*block, error) {
	t, err := readInt64(r)
	if err != nil {
		return nil, err
	}
	l, err := readInt64(r)
	if err != nil {
		return nil, unexpectedEOF(err)
	}
	d := make([]byte, l)
	n, err := r.Read(d)
	if err != nil {
		return nil, unexpectedEOF(err)
	}
	if n != int(l) {
		return nil, fmt.Errorf("len(data) = %d, want %d", n, l)
	}
	return &block{t, l, d}, nil
}
