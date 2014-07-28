package wal

import (
	"fmt"
	"io"
)

type block struct {
	t int64
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

func readBlock(r io.Reader, b *block) error {
	t, err := readInt64(r)
	if err != nil {
		return err
	}
	l, err := readInt64(r)
	if err != nil {
		return unexpectedEOF(err)
	}
	d := make([]byte, l)
	n, err := r.Read(d)
	if err != nil {
		return unexpectedEOF(err)
	}
	if n != int(l) {
		return fmt.Errorf("len(data) = %d, want %d", n, l)
	}
	b.t = t
	b.d = d
	return nil
}
