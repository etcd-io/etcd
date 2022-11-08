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
		buffer: make([]uint64, 10),
	}

	for i := 0; i < 5; i++ {
		in.Add(uint64(i))
	}

	wantIn := &Inflights{
		start: 0,
		count: 5,
		size:  10,
		//               ↓------------
		buffer: []uint64{0, 1, 2, 3, 4, 0, 0, 0, 0, 0},
	}
	require.Equal(t, wantIn, in)

	for i := 5; i < 10; i++ {
		in.Add(uint64(i))
	}

	wantIn2 := &Inflights{
		start: 0,
		count: 10,
		size:  10,
		//               ↓---------------------------
		buffer: []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	require.Equal(t, wantIn2, in)

	// rotating case
	in2 := &Inflights{
		start:  5,
		size:   10,
		buffer: make([]uint64, 10),
	}

	for i := 0; i < 5; i++ {
		in2.Add(uint64(i))
	}

	wantIn21 := &Inflights{
		start: 5,
		count: 5,
		size:  10,
		//                              ↓------------
		buffer: []uint64{0, 0, 0, 0, 0, 0, 1, 2, 3, 4},
	}
	require.Equal(t, wantIn21, in2)

	for i := 5; i < 10; i++ {
		in2.Add(uint64(i))
	}

	wantIn22 := &Inflights{
		start: 5,
		count: 10,
		size:  10,
		//               -------------- ↓------------
		buffer: []uint64{5, 6, 7, 8, 9, 0, 1, 2, 3, 4},
	}
	require.Equal(t, wantIn22, in2)
}

func TestInflightFreeTo(t *testing.T) {
	// no rotating case
	in := NewInflights(10)
	for i := 0; i < 10; i++ {
		in.Add(uint64(i))
	}

	in.FreeLE(0)

	wantIn0 := &Inflights{
		start: 1,
		count: 9,
		size:  10,
		//                  ↓------------------------
		buffer: []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	require.Equal(t, wantIn0, in)

	in.FreeLE(4)

	wantIn := &Inflights{
		start: 5,
		count: 5,
		size:  10,
		//                              ↓------------
		buffer: []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	require.Equal(t, wantIn, in)

	in.FreeLE(8)

	wantIn2 := &Inflights{
		start: 9,
		count: 1,
		size:  10,
		//                                          ↓
		buffer: []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	require.Equal(t, wantIn2, in)

	// rotating case
	for i := 10; i < 15; i++ {
		in.Add(uint64(i))
	}

	in.FreeLE(12)

	wantIn3 := &Inflights{
		start: 3,
		count: 2,
		size:  10,
		//                           ↓-----
		buffer: []uint64{10, 11, 12, 13, 14, 5, 6, 7, 8, 9},
	}
	require.Equal(t, wantIn3, in)

	in.FreeLE(14)

	wantIn4 := &Inflights{
		start: 0,
		count: 0,
		size:  10,
		//               ↓
		buffer: []uint64{10, 11, 12, 13, 14, 5, 6, 7, 8, 9},
	}
	require.Equal(t, wantIn4, in)
}
