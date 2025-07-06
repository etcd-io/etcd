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
	"reflect"
	"testing"
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

	if !reflect.DeepEqual(in, wantIn) {
		t.Fatalf("in = %+v, want %+v", in, wantIn)
	}

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

	if !reflect.DeepEqual(in, wantIn2) {
		t.Fatalf("in = %+v, want %+v", in, wantIn2)
	}

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

	if !reflect.DeepEqual(in2, wantIn21) {
		t.Fatalf("in = %+v, want %+v", in2, wantIn21)
	}

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

	if !reflect.DeepEqual(in2, wantIn22) {
		t.Fatalf("in = %+v, want %+v", in2, wantIn22)
	}
}

func TestInflightFreeTo(t *testing.T) {
	// no rotating case
	in := NewInflights(10)
	for i := 0; i < 10; i++ {
		in.Add(uint64(i))
	}

	in.FreeLE(4)

	wantIn := &Inflights{
		start: 5,
		count: 5,
		size:  10,
		//                              ↓------------
		buffer: []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	if !reflect.DeepEqual(in, wantIn) {
		t.Fatalf("in = %+v, want %+v", in, wantIn)
	}

	in.FreeLE(8)

	wantIn2 := &Inflights{
		start: 9,
		count: 1,
		size:  10,
		//                                          ↓
		buffer: []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	if !reflect.DeepEqual(in, wantIn2) {
		t.Fatalf("in = %+v, want %+v", in, wantIn2)
	}

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

	if !reflect.DeepEqual(in, wantIn3) {
		t.Fatalf("in = %+v, want %+v", in, wantIn3)
	}

	in.FreeLE(14)

	wantIn4 := &Inflights{
		start: 0,
		count: 0,
		size:  10,
		//               ↓
		buffer: []uint64{10, 11, 12, 13, 14, 5, 6, 7, 8, 9},
	}

	if !reflect.DeepEqual(in, wantIn4) {
		t.Fatalf("in = %+v, want %+v", in, wantIn4)
	}
}

func TestInflightFreeFirstOne(t *testing.T) {
	in := NewInflights(10)
	for i := 0; i < 10; i++ {
		in.Add(uint64(i))
	}

	in.FreeFirstOne()

	wantIn := &Inflights{
		start: 1,
		count: 9,
		size:  10,
		//                  ↓------------------------
		buffer: []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	if !reflect.DeepEqual(in, wantIn) {
		t.Fatalf("in = %+v, want %+v", in, wantIn)
	}
}
