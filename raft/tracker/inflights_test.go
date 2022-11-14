// Copyright 2019 The etcd Authors
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

package tracker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInflightsAdd(t *testing.T) {
	// no rotating case
	in := &Inflights{
		size:   10,
		buffer: make([]inflight, 10),
	}

	for i := 0; i < 5; i++ {
		in.Add(uint64(i), uint64(100+i))
	}

	wantIn := &Inflights{
		start: 0,
		count: 5,
		bytes: 510,
		size:  10,
		buffer: inflightsBuffer(
			//       ↓------------
			[]uint64{0, 1, 2, 3, 4, 0, 0, 0, 0, 0},
			[]uint64{100, 101, 102, 103, 104, 0, 0, 0, 0, 0}),
	}
	require.Equal(t, wantIn, in)

	for i := 5; i < 10; i++ {
		in.Add(uint64(i), uint64(100+i))
	}

	wantIn2 := &Inflights{
		start: 0,
		count: 10,
		bytes: 1045,
		size:  10,
		buffer: inflightsBuffer(
			//       ↓---------------------------
			[]uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			[]uint64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109}),
	}
	require.Equal(t, wantIn2, in)

	// rotating case
	in2 := &Inflights{
		start:  5,
		size:   10,
		buffer: make([]inflight, 10),
	}

	for i := 0; i < 5; i++ {
		in2.Add(uint64(i), uint64(100+i))
	}

	wantIn21 := &Inflights{
		start: 5,
		count: 5,
		bytes: 510,
		size:  10,
		buffer: inflightsBuffer(
			//                      ↓------------
			[]uint64{0, 0, 0, 0, 0, 0, 1, 2, 3, 4},
			[]uint64{0, 0, 0, 0, 0, 100, 101, 102, 103, 104}),
	}
	require.Equal(t, wantIn21, in2)

	for i := 5; i < 10; i++ {
		in2.Add(uint64(i), uint64(100+i))
	}

	wantIn22 := &Inflights{
		start: 5,
		count: 10,
		bytes: 1045,
		size:  10,
		buffer: inflightsBuffer(
			//       -------------- ↓------------
			[]uint64{5, 6, 7, 8, 9, 0, 1, 2, 3, 4},
			[]uint64{105, 106, 107, 108, 109, 100, 101, 102, 103, 104}),
	}
	require.Equal(t, wantIn22, in2)
}

func TestInflightFreeTo(t *testing.T) {
	// no rotating case
	in := NewInflights(10, 0)
	for i := 0; i < 10; i++ {
		in.Add(uint64(i), uint64(100+i))
	}

	in.FreeLE(0)

	wantIn0 := &Inflights{
		start: 1,
		count: 9,
		bytes: 945,
		size:  10,
		buffer: inflightsBuffer(
			//          ↓------------------------
			[]uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			[]uint64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109}),
	}
	require.Equal(t, wantIn0, in)

	in.FreeLE(4)

	wantIn := &Inflights{
		start: 5,
		count: 5,
		bytes: 535,
		size:  10,
		buffer: inflightsBuffer(
			//                      ↓------------
			[]uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			[]uint64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109}),
	}
	require.Equal(t, wantIn, in)

	in.FreeLE(8)

	wantIn2 := &Inflights{
		start: 9,
		count: 1,
		bytes: 109,
		size:  10,
		buffer: inflightsBuffer(
			//                                  ↓
			[]uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			[]uint64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109}),
	}
	require.Equal(t, wantIn2, in)

	// rotating case
	for i := 10; i < 15; i++ {
		in.Add(uint64(i), uint64(100+i))
	}

	in.FreeLE(12)

	wantIn3 := &Inflights{
		start: 3,
		count: 2,
		bytes: 227,
		size:  10,
		buffer: inflightsBuffer(
			//                   ↓-----
			[]uint64{10, 11, 12, 13, 14, 5, 6, 7, 8, 9},
			[]uint64{110, 111, 112, 113, 114, 105, 106, 107, 108, 109}),
	}
	require.Equal(t, wantIn3, in)

	in.FreeLE(14)

	wantIn4 := &Inflights{
		start: 0,
		count: 0,
		size:  10,
		buffer: inflightsBuffer(
			//       ↓
			[]uint64{10, 11, 12, 13, 14, 5, 6, 7, 8, 9},
			[]uint64{110, 111, 112, 113, 114, 105, 106, 107, 108, 109}),
	}
	require.Equal(t, wantIn4, in)
}

func TestInflightsFull(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     int
		maxBytes uint64
		fullAt   int
		freeLE   uint64
		againAt  int
	}{
		{name: "always-full", size: 0, fullAt: 0},
		{name: "single-entry", size: 1, fullAt: 1, freeLE: 1, againAt: 2},
		{name: "single-entry-overflow", size: 1, maxBytes: 10, fullAt: 1, freeLE: 1, againAt: 2},
		{name: "multi-entry", size: 15, fullAt: 15, freeLE: 6, againAt: 22},
		{name: "slight-overflow", size: 8, maxBytes: 400, fullAt: 4, freeLE: 2, againAt: 7},
		{name: "exact-max-bytes", size: 8, maxBytes: 406, fullAt: 4, freeLE: 3, againAt: 8},
		{name: "larger-overflow", size: 15, maxBytes: 408, fullAt: 5, freeLE: 1, againAt: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := NewInflights(tc.size, tc.maxBytes)

			addUntilFull := func(begin, end int) {
				for i := begin; i < end; i++ {
					if in.Full() {
						t.Fatalf("full at %d, want %d", i, end)
					}
					in.Add(uint64(i), uint64(100+i))
				}
				if !in.Full() {
					t.Fatalf("not full at %d", end)
				}
			}

			addUntilFull(0, tc.fullAt)
			in.FreeLE(tc.freeLE)
			addUntilFull(tc.fullAt, tc.againAt)

			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Add() did not panic")
				}
			}()
			in.Add(100, 1024)
		})
	}
}

func inflightsBuffer(indices []uint64, sizes []uint64) []inflight {
	if len(indices) != len(sizes) {
		panic("len(indices) != len(sizes)")
	}
	buffer := make([]inflight, 0, len(indices))
	for i, idx := range indices {
		buffer = append(buffer, inflight{index: idx, bytes: sizes[i]})
	}
	return buffer
}
