/*
 * Copyright 2017 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package table

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"

	"github.com/AndreasBriese/bbloom"
	"github.com/dgraph-io/badger/y"
)

var (
	restartInterval = 100 // Might want to change this to be based on total size instead of numKeys.
)

func newBuffer(sz int) *bytes.Buffer {
	b := new(bytes.Buffer)
	b.Grow(sz)
	return b
}

type header struct {
	plen uint16 // Overlap with base key.
	klen uint16 // Length of the diff.
	vlen uint16 // Length of value.
	prev uint32 // Offset for the previous key-value pair. The offset is relative to block base offset.
}

// Encode encodes the header.
func (h header) Encode(b []byte) {
	binary.BigEndian.PutUint16(b[0:2], h.plen)
	binary.BigEndian.PutUint16(b[2:4], h.klen)
	binary.BigEndian.PutUint16(b[4:6], h.vlen)
	binary.BigEndian.PutUint32(b[6:10], h.prev)
}

// Decode decodes the header.
func (h *header) Decode(buf []byte) int {
	h.plen = binary.BigEndian.Uint16(buf[0:2])
	h.klen = binary.BigEndian.Uint16(buf[2:4])
	h.vlen = binary.BigEndian.Uint16(buf[4:6])
	h.prev = binary.BigEndian.Uint32(buf[6:10])
	return h.Size()
}

// Size returns size of the header. Currently it's just a constant.
func (h header) Size() int { return 10 }

// Builder is used in building a table.
type Builder struct {
	counter int // Number of keys written for the current block.

	// Typically tens or hundreds of meg. This is for one single file.
	buf *bytes.Buffer

	baseKey    []byte // Base key for the current block.
	baseOffset uint32 // Offset for the current block.

	restarts []uint32 // Base offsets of every block.

	// Tracks offset for the previous key-value pair. Offset is relative to block base offset.
	prevOffset uint32

	keyBuf   *bytes.Buffer
	keyCount int
}

// NewTableBuilder makes a new TableBuilder.
func NewTableBuilder() *Builder {
	return &Builder{
		keyBuf:     newBuffer(1 << 20),
		buf:        newBuffer(1 << 20),
		prevOffset: math.MaxUint32, // Used for the first element!
	}
}

// Close closes the TableBuilder.
func (b *Builder) Close() {}

// Empty returns whether it's empty.
func (b *Builder) Empty() bool { return b.buf.Len() == 0 }

// keyDiff returns a suffix of newKey that is different from b.baseKey.
func (b Builder) keyDiff(newKey []byte) []byte {
	var i int
	for i = 0; i < len(newKey) && i < len(b.baseKey); i++ {
		if newKey[i] != b.baseKey[i] {
			break
		}
	}
	return newKey[i:]
}

func (b *Builder) addHelper(key []byte, v y.ValueStruct) {
	// Add key to bloom filter.
	if len(key) > 0 {
		var klen [2]byte
		keyNoTs := y.ParseKey(key)
		binary.BigEndian.PutUint16(klen[:], uint16(len(keyNoTs)))
		b.keyBuf.Write(klen[:])
		b.keyBuf.Write(keyNoTs)
		b.keyCount++
	}

	// diffKey stores the difference of key with baseKey.
	var diffKey []byte
	if len(b.baseKey) == 0 {
		// Make a copy. Builder should not keep references. Otherwise, caller has to be very careful
		// and will have to make copies of keys every time they add to builder, which is even worse.
		b.baseKey = append(b.baseKey[:0], key...)
		diffKey = key
	} else {
		diffKey = b.keyDiff(key)
	}

	h := header{
		plen: uint16(len(key) - len(diffKey)),
		klen: uint16(len(diffKey)),
		vlen: uint16(v.EncodedSize()),
		prev: b.prevOffset, // prevOffset is the location of the last key-value added.
	}
	b.prevOffset = uint32(b.buf.Len()) - b.baseOffset // Remember current offset for the next Add call.

	// Layout: header, diffKey, value.
	var hbuf [10]byte
	h.Encode(hbuf[:])
	b.buf.Write(hbuf[:])
	b.buf.Write(diffKey) // We only need to store the key difference.

	v.EncodeTo(b.buf)
	b.counter++ // Increment number of keys added for this current block.
}

func (b *Builder) finishBlock() {
	// When we are at the end of the block and Valid=false, and the user wants to do a Prev,
	// we need a dummy header to tell us the offset of the previous key-value pair.
	b.addHelper([]byte{}, y.ValueStruct{})
}

// Add adds a key-value pair to the block.
// If doNotRestart is true, we will not restart even if b.counter >= restartInterval.
func (b *Builder) Add(key []byte, value y.ValueStruct) error {
	if b.counter >= restartInterval {
		b.finishBlock()
		// Start a new block. Initialize the block.
		b.restarts = append(b.restarts, uint32(b.buf.Len()))
		b.counter = 0
		b.baseKey = []byte{}
		b.baseOffset = uint32(b.buf.Len())
		b.prevOffset = math.MaxUint32 // First key-value pair of block has header.prev=MaxInt.
	}
	b.addHelper(key, value)
	return nil // Currently, there is no meaningful error.
}

// TODO: vvv this was the comment on ReachedCapacity.
// FinalSize returns the *rough* final size of the array, counting the header which is not yet written.
// TODO: Look into why there is a discrepancy. I suspect it is because of Write(empty, empty)
// at the end. The diff can vary.

// ReachedCapacity returns true if we... roughly (?) reached capacity?
func (b *Builder) ReachedCapacity(cap int64) bool {
	estimateSz := b.buf.Len() + 8 /* empty header */ + 4*len(b.restarts) + 8 // 8 = end of buf offset + len(restarts).
	return int64(estimateSz) > cap
}

// blockIndex generates the block index for the table.
// It is mainly a list of all the block base offsets.
func (b *Builder) blockIndex() []byte {
	// Store the end offset, so we know the length of the final block.
	b.restarts = append(b.restarts, uint32(b.buf.Len()))

	// Add 4 because we want to write out number of restarts at the end.
	sz := 4*len(b.restarts) + 4
	out := make([]byte, sz)
	buf := out
	for _, r := range b.restarts {
		binary.BigEndian.PutUint32(buf[:4], r)
		buf = buf[4:]
	}
	binary.BigEndian.PutUint32(buf[:4], uint32(len(b.restarts)))
	return out
}

// Finish finishes the table by appending the index.
func (b *Builder) Finish() []byte {
	bf := bbloom.New(float64(b.keyCount), 0.01)
	var klen [2]byte
	key := make([]byte, 1024)
	for {
		if _, err := b.keyBuf.Read(klen[:]); err == io.EOF {
			break
		} else if err != nil {
			y.Check(err)
		}
		kl := int(binary.BigEndian.Uint16(klen[:]))
		if cap(key) < kl {
			key = make([]byte, 2*int(kl)) // 2 * uint16 will overflow
		}
		key = key[:kl]
		y.Check2(b.keyBuf.Read(key))
		bf.Add(key)
	}

	b.finishBlock() // This will never start a new block.
	index := b.blockIndex()
	b.buf.Write(index)

	// Write bloom filter.
	bdata := bf.JSONMarshal()
	n, err := b.buf.Write(bdata)
	y.Check(err)
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(n))
	b.buf.Write(buf[:])

	return b.buf.Bytes()
}
