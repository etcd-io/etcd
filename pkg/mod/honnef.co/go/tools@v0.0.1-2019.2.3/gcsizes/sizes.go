// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gcsizes provides a types.Sizes implementation that adheres
// to the rules used by the gc compiler.
package gcsizes // import "honnef.co/go/tools/gcsizes"

import (
	"go/build"
	"go/types"
)

type Sizes struct {
	WordSize int64
	MaxAlign int64
}

// ForArch returns a correct Sizes for the given architecture.
func ForArch(arch string) *Sizes {
	wordSize := int64(8)
	maxAlign := int64(8)
	switch build.Default.GOARCH {
	case "386", "arm":
		wordSize, maxAlign = 4, 4
	case "amd64p32":
		wordSize = 4
	}
	return &Sizes{WordSize: wordSize, MaxAlign: maxAlign}
}

func (s *Sizes) Alignof(T types.Type) int64 {
	switch t := T.Underlying().(type) {
	case *types.Array:
		return s.Alignof(t.Elem())
	case *types.Struct:
		max := int64(1)
		n := t.NumFields()
		var fields []*types.Var
		for i := 0; i < n; i++ {
			fields = append(fields, t.Field(i))
		}
		for _, f := range fields {
			if a := s.Alignof(f.Type()); a > max {
				max = a
			}
		}
		return max
	}
	a := s.Sizeof(T) // may be 0
	if a < 1 {
		return 1
	}
	if a > s.MaxAlign {
		return s.MaxAlign
	}
	return a
}

func (s *Sizes) Offsetsof(fields []*types.Var) []int64 {
	offsets := make([]int64, len(fields))
	var o int64
	for i, f := range fields {
		a := s.Alignof(f.Type())
		o = align(o, a)
		offsets[i] = o
		o += s.Sizeof(f.Type())
	}
	return offsets
}

var basicSizes = [...]byte{
	types.Bool:       1,
	types.Int8:       1,
	types.Int16:      2,
	types.Int32:      4,
	types.Int64:      8,
	types.Uint8:      1,
	types.Uint16:     2,
	types.Uint32:     4,
	types.Uint64:     8,
	types.Float32:    4,
	types.Float64:    8,
	types.Complex64:  8,
	types.Complex128: 16,
}

func (s *Sizes) Sizeof(T types.Type) int64 {
	switch t := T.Underlying().(type) {
	case *types.Basic:
		k := t.Kind()
		if int(k) < len(basicSizes) {
			if s := basicSizes[k]; s > 0 {
				return int64(s)
			}
		}
		if k == types.String {
			return s.WordSize * 2
		}
	case *types.Array:
		n := t.Len()
		if n == 0 {
			return 0
		}
		a := s.Alignof(t.Elem())
		z := s.Sizeof(t.Elem())
		return align(z, a)*(n-1) + z
	case *types.Slice:
		return s.WordSize * 3
	case *types.Struct:
		n := t.NumFields()
		if n == 0 {
			return 0
		}

		var fields []*types.Var
		for i := 0; i < n; i++ {
			fields = append(fields, t.Field(i))
		}
		offsets := s.Offsetsof(fields)
		a := s.Alignof(T)
		lsz := s.Sizeof(fields[n-1].Type())
		if lsz == 0 {
			lsz = 1
		}
		z := offsets[n-1] + lsz
		return align(z, a)
	case *types.Interface:
		return s.WordSize * 2
	}
	return s.WordSize // catch-all
}

// align returns the smallest y >= x such that y % a == 0.
func align(x, a int64) int64 {
	y := x + a - 1
	return y - y%a
}
