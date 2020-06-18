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

// Inflights limits the number of MsgApp (represented by the largest index
// contained within) sent to followers but not yet acknowledged by them. Callers
// use Full() to check whether more messages can be sent, call Add() whenever
// they are sending a new append, and release "quota" via FreeLE() whenever an
// ack is received.
type Inflights struct {
	// the starting index in the buffer
	start int
	// number of inflights in the buffer
	count int

	// the size of the buffer
	size int

	pivot int

	// buffer contains the index of the last entry
	// inside one message.
	buffer []uint64
}

// NewInflights sets up an Inflights that allows up to 'size' inflight messages.
func NewInflights(size int) *Inflights {
	return &Inflights{
		size: size,
	}
}

// Clone returns an *Inflights that is identical to but shares no memory with
// the receiver.
func (in *Inflights) Clone() *Inflights {
	ins := *in
	ins.buffer = append([]uint64(nil), in.buffer...)
	return &ins
}

// Add notifies the Inflights that a new message with the given index is being
// dispatched. Full() must be called prior to Add() to verify that there is room
// for one more message, and consecutive calls to add Add() must provide a
// monotonic sequence of indexes.
func (in *Inflights) Add(inflight uint64) {
	if in.Full() {
		panic("cannot add into a Full inflights")
	}
	next := in.start + in.count
	size := in.size
	if next >= size {
		next -= size
	}
	if next >= len(in.buffer) {
		in.grow()
	}
	in.buffer[next] = inflight
	in.count++
}

// grow the inflight buffer by doubling up to inflights.size. We grow on demand
// instead of preallocating to inflights.size to handle systems which have
// thousands of Raft groups per process.
func (in *Inflights) grow() {
	newSize := len(in.buffer) * 2
	if newSize == 0 {
		newSize = 1
	} else if newSize > in.size {
		newSize = in.size
	}
	newBuffer := make([]uint64, newSize)
	copy(newBuffer, in.buffer)
	in.buffer = newBuffer
}

// FreeLE frees the inflights smaller or equal to the given `to` flight.
func (in *Inflights) FreeLE(to uint64, useBinary bool) {
	if in.count == 0 || to < in.buffer[in.start] {
		// out of the left side of the window
		return
	}
	if useBinary && to < in.buffer[in.rotate(in.start+in.pivot)] {
		start, end := in.start, in.start+in.count
		mid := (start + end) / 2
		// find the first inflight <= to
		for start < end {
			v := in.buffer[in.rotate(mid)]
			if v > to {
				end = mid
			} else if v < to {
				start = mid + 1
			} else {
				break
			}
			mid = (start + end) / 2
		}
		in.count -= mid + 1 - in.start
		if in.count == 0 {
			// inflights is empty, reset the start index so that we don't grow the
			// buffer unnecessarily.
			in.start = 0
		} else {
			in.start = in.rotate(mid + 1)
		}
	} else {
		idx := in.start
		var i int
		for i = 0; i < in.count; i++ {
			if to < in.buffer[idx] { // found the first large inflight
				break
			}

			// increase index and maybe rotate
			size := in.size
			if idx++; idx >= size {
				idx -= size

			}
		}
		// free i inflights and set new start index
		in.count -= i
		in.start = idx
		if in.count == 0 {
			// inflights is empty, reset the start index so that we don't grow the
			// buffer unnecessarily.
			in.start = 0
		}
	}
}

func (in *Inflights) rotate(idx int) int {
	if idx >= in.size {
		return idx - in.size
	} else {
		return idx
	}
}

// FreeFirstOne releases the first inflight. This is a no-op if nothing is
// inflight.
func (in *Inflights) FreeFirstOne() { in.FreeLE(in.buffer[in.start], false) }

// Full returns true if no more messages can be sent at the moment.
func (in *Inflights) Full() bool {
	return in.count == in.size
}

// Count returns the number of inflight messages.
func (in *Inflights) Count() int { return in.count }

// reset frees all inflights.
func (in *Inflights) reset() {
	in.count = 0
	in.start = 0
}

func (in *Inflights) updatePivot() {
	if in.count == 0 {
		in.pivot = 0
	} else {
		in.pivot = log2(in.count)
	}
}

var tab64 = []int{
	63, 0, 58, 1, 59, 47, 53, 2,
	60, 39, 48, 27, 54, 33, 42, 3,
	61, 51, 37, 40, 49, 18, 28, 20,
	55, 30, 34, 11, 43, 14, 22, 4,
	62, 57, 46, 52, 38, 26, 32, 41,
	50, 36, 17, 19, 29, 10, 13, 21,
	56, 45, 25, 31, 35, 16, 9, 12,
	44, 24, 15, 8, 23, 7, 6, 5,
}

// Only for 64
// See http://graphics.stanford.edu/~seander/bithacks.html#IntegerLogDeBruijn
func log2(n int) int {
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return tab64[((n-(n>>1))*0x07ED_D5E5_9A4E_28C2)>>58]
}
