// Copyright ©2021 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fpdf

import (
	"bytes"
	"encoding/binary"
	"io"
)

type rbuffer struct {
	p []byte
	c int
}

// newRBuffer returns a new buffer populated with the contents of the specified Reader
func newRBuffer(r io.Reader) (b *rbuffer, err error) {
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(r)
	b = &rbuffer{p: buf.Bytes()}
	return
}

func (r *rbuffer) Read(p []byte) (int, error) {
	if r.c >= len(r.p) {
		return 0, io.EOF
	}
	n := copy(p, r.p[r.c:])
	r.c += n
	return n, nil
}

func (r *rbuffer) ReadByte() (byte, error) {
	if r.c >= len(r.p) {
		return 0, io.EOF
	}
	v := r.p[r.c]
	r.c++
	return v, nil
}

func (r *rbuffer) u8() uint8 {
	if r.c >= len(r.p) {
		panic(io.ErrShortBuffer)
	}
	v := r.p[r.c]
	r.c++
	return v
}

func (r *rbuffer) u32() uint32 {
	const n = 4
	if r.c+n >= len(r.p) {
		panic(io.ErrShortBuffer)
	}
	beg := r.c
	r.c += n
	v := binary.BigEndian.Uint32(r.p[beg:])
	return v
}

func (r *rbuffer) i32() int32 {
	return int32(r.u32())
}

func (r *rbuffer) Next(n int) []byte {
	c := r.c
	r.c += n
	return r.p[c:r.c]
}
