// Copyright ©2021 The go-pdf Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fpdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"sync"
)

var xmem = xmempool{
	Pool: sync.Pool{
		New: func() any {
			var m membuffer
			return &m
		},
	},
}

type xmempool struct{ sync.Pool }

func (pool *xmempool) compress(data []byte) *membuffer {
	mem := pool.Get().(*membuffer)
	buf := &mem.buf
	buf.Grow(len(data))

	zw, err := zlib.NewWriterLevel(buf, zlib.BestSpeed)
	if err != nil {
		panic(fmt.Errorf("could not create zlib writer: %w", err))
	}
	_, err = zw.Write(data)
	if err != nil {
		panic(fmt.Errorf("could not zlib-compress slice: %w", err))
	}

	err = zw.Close()
	if err != nil {
		panic(fmt.Errorf("could not close zlib writer: %w", err))
	}
	return mem
}

func (pool *xmempool) uncompress(data []byte) (*membuffer, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	mem := pool.Get().(*membuffer)
	mem.buf.Reset()

	_, err = mem.buf.ReadFrom(zr)
	if err != nil {
		mem.release()
		return nil, err
	}

	return mem, nil
}

type membuffer struct {
	buf bytes.Buffer
}

func (mem *membuffer) bytes() []byte { return mem.buf.Bytes() }
func (mem *membuffer) release() {
	mem.buf.Reset()
	xmem.Put(mem)
}

func (mem *membuffer) copy() []byte {
	src := mem.bytes()
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
