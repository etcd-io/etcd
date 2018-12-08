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
	"io"
	"math"
	"sort"

	"github.com/dgraph-io/badger/y"
	"github.com/pkg/errors"
)

type blockIterator struct {
	data    []byte
	pos     uint32
	err     error
	baseKey []byte

	key  []byte
	val  []byte
	init bool

	last header // The last header we saw.
}

func (itr *blockIterator) Reset() {
	itr.pos = 0
	itr.err = nil
	itr.baseKey = []byte{}
	itr.key = []byte{}
	itr.val = []byte{}
	itr.init = false
	itr.last = header{}
}

func (itr *blockIterator) Init() {
	if !itr.init {
		itr.Next()
	}
}

func (itr *blockIterator) Valid() bool {
	return itr != nil && itr.err == nil
}

func (itr *blockIterator) Error() error {
	return itr.err
}

func (itr *blockIterator) Close() {}

var (
	origin  = 0
	current = 1
)

// Seek brings us to the first block element that is >= input key.
func (itr *blockIterator) Seek(key []byte, whence int) {
	itr.err = nil

	switch whence {
	case origin:
		itr.Reset()
	case current:
	}

	var done bool
	for itr.Init(); itr.Valid(); itr.Next() {
		k := itr.Key()
		if y.CompareKeys(k, key) >= 0 {
			// We are done as k is >= key.
			done = true
			break
		}
	}
	if !done {
		itr.err = io.EOF
	}
}

func (itr *blockIterator) SeekToFirst() {
	itr.err = nil
	itr.Init()
}

// SeekToLast brings us to the last element. Valid should return true.
func (itr *blockIterator) SeekToLast() {
	itr.err = nil
	for itr.Init(); itr.Valid(); itr.Next() {
	}
	itr.Prev()
}

// parseKV would allocate a new byte slice for key and for value.
func (itr *blockIterator) parseKV(h header) {
	if cap(itr.key) < int(h.plen+h.klen) {
		sz := int(h.plen) + int(h.klen) // Convert to int before adding to avoid uint16 overflow.
		itr.key = make([]byte, 2*sz)
	}
	itr.key = itr.key[:h.plen+h.klen]
	copy(itr.key, itr.baseKey[:h.plen])
	copy(itr.key[h.plen:], itr.data[itr.pos:itr.pos+uint32(h.klen)])
	itr.pos += uint32(h.klen)

	if itr.pos+uint32(h.vlen) > uint32(len(itr.data)) {
		itr.err = errors.Errorf("Value exceeded size of block: %d %d %d %d %v",
			itr.pos, h.klen, h.vlen, len(itr.data), h)
		return
	}
	itr.val = y.SafeCopy(itr.val, itr.data[itr.pos:itr.pos+uint32(h.vlen)])
	itr.pos += uint32(h.vlen)
}

func (itr *blockIterator) Next() {
	itr.init = true
	itr.err = nil
	if itr.pos >= uint32(len(itr.data)) {
		itr.err = io.EOF
		return
	}

	var h header
	itr.pos += uint32(h.Decode(itr.data[itr.pos:]))
	itr.last = h // Store the last header.

	if h.klen == 0 && h.plen == 0 {
		// Last entry in the table.
		itr.err = io.EOF
		return
	}

	// Populate baseKey if it isn't set yet. This would only happen for the first Next.
	if len(itr.baseKey) == 0 {
		// This should be the first Next() for this block. Hence, prefix length should be zero.
		y.AssertTrue(h.plen == 0)
		itr.baseKey = itr.data[itr.pos : itr.pos+uint32(h.klen)]
	}
	itr.parseKV(h)
}

func (itr *blockIterator) Prev() {
	if !itr.init {
		return
	}
	itr.err = nil
	if itr.last.prev == math.MaxUint32 {
		// This is the first element of the block!
		itr.err = io.EOF
		itr.pos = 0
		return
	}

	// Move back using current header's prev.
	itr.pos = itr.last.prev

	var h header
	y.AssertTruef(itr.pos < uint32(len(itr.data)), "%d %d", itr.pos, len(itr.data))
	itr.pos += uint32(h.Decode(itr.data[itr.pos:]))
	itr.parseKV(h)
	itr.last = h
}

func (itr *blockIterator) Key() []byte {
	if itr.err != nil {
		return nil
	}
	return itr.key
}

func (itr *blockIterator) Value() []byte {
	if itr.err != nil {
		return nil
	}
	return itr.val
}

// Iterator is an iterator for a Table.
type Iterator struct {
	t    *Table
	bpos int
	bi   *blockIterator
	err  error

	// Internally, Iterator is bidirectional. However, we only expose the
	// unidirectional functionality for now.
	reversed bool
}

// NewIterator returns a new iterator of the Table
func (t *Table) NewIterator(reversed bool) *Iterator {
	t.IncrRef() // Important.
	ti := &Iterator{t: t, reversed: reversed}
	ti.next()
	return ti
}

// Close closes the iterator (and it must be called).
func (itr *Iterator) Close() error {
	return itr.t.DecrRef()
}

func (itr *Iterator) reset() {
	itr.bpos = 0
	itr.err = nil
}

// Valid follows the y.Iterator interface
func (itr *Iterator) Valid() bool {
	return itr.err == nil
}

func (itr *Iterator) seekToFirst() {
	numBlocks := len(itr.t.blockIndex)
	if numBlocks == 0 {
		itr.err = io.EOF
		return
	}
	itr.bpos = 0
	block, err := itr.t.block(itr.bpos)
	if err != nil {
		itr.err = err
		return
	}
	itr.bi = block.NewIterator()
	itr.bi.SeekToFirst()
	itr.err = itr.bi.Error()
}

func (itr *Iterator) seekToLast() {
	numBlocks := len(itr.t.blockIndex)
	if numBlocks == 0 {
		itr.err = io.EOF
		return
	}
	itr.bpos = numBlocks - 1
	block, err := itr.t.block(itr.bpos)
	if err != nil {
		itr.err = err
		return
	}
	itr.bi = block.NewIterator()
	itr.bi.SeekToLast()
	itr.err = itr.bi.Error()
}

func (itr *Iterator) seekHelper(blockIdx int, key []byte) {
	itr.bpos = blockIdx
	block, err := itr.t.block(blockIdx)
	if err != nil {
		itr.err = err
		return
	}
	itr.bi = block.NewIterator()
	itr.bi.Seek(key, origin)
	itr.err = itr.bi.Error()
}

// seekFrom brings us to a key that is >= input key.
func (itr *Iterator) seekFrom(key []byte, whence int) {
	itr.err = nil
	switch whence {
	case origin:
		itr.reset()
	case current:
	}

	idx := sort.Search(len(itr.t.blockIndex), func(idx int) bool {
		ko := itr.t.blockIndex[idx]
		return y.CompareKeys(ko.key, key) > 0
	})
	if idx == 0 {
		// The smallest key in our table is already strictly > key. We can return that.
		// This is like a SeekToFirst.
		itr.seekHelper(0, key)
		return
	}

	// block[idx].smallest is > key.
	// Since idx>0, we know block[idx-1].smallest is <= key.
	// There are two cases.
	// 1) Everything in block[idx-1] is strictly < key. In this case, we should go to the first
	//    element of block[idx].
	// 2) Some element in block[idx-1] is >= key. We should go to that element.
	itr.seekHelper(idx-1, key)
	if itr.err == io.EOF {
		// Case 1. Need to visit block[idx].
		if idx == len(itr.t.blockIndex) {
			// If idx == len(itr.t.blockIndex), then input key is greater than ANY element of table.
			// There's nothing we can do. Valid() should return false as we seek to end of table.
			return
		}
		// Since block[idx].smallest is > key. This is essentially a block[idx].SeekToFirst.
		itr.seekHelper(idx, key)
	}
	// Case 2: No need to do anything. We already did the seek in block[idx-1].
}

// seek will reset iterator and seek to >= key.
func (itr *Iterator) seek(key []byte) {
	itr.seekFrom(key, origin)
}

// seekForPrev will reset iterator and seek to <= key.
func (itr *Iterator) seekForPrev(key []byte) {
	// TODO: Optimize this. We shouldn't have to take a Prev step.
	itr.seekFrom(key, origin)
	if !bytes.Equal(itr.Key(), key) {
		itr.prev()
	}
}

func (itr *Iterator) next() {
	itr.err = nil

	if itr.bpos >= len(itr.t.blockIndex) {
		itr.err = io.EOF
		return
	}

	if itr.bi == nil {
		block, err := itr.t.block(itr.bpos)
		if err != nil {
			itr.err = err
			return
		}
		itr.bi = block.NewIterator()
		itr.bi.SeekToFirst()
		itr.err = itr.bi.Error()
		return
	}

	itr.bi.Next()
	if !itr.bi.Valid() {
		itr.bpos++
		itr.bi = nil
		itr.next()
		return
	}
}

func (itr *Iterator) prev() {
	itr.err = nil
	if itr.bpos < 0 {
		itr.err = io.EOF
		return
	}

	if itr.bi == nil {
		block, err := itr.t.block(itr.bpos)
		if err != nil {
			itr.err = err
			return
		}
		itr.bi = block.NewIterator()
		itr.bi.SeekToLast()
		itr.err = itr.bi.Error()
		return
	}

	itr.bi.Prev()
	if !itr.bi.Valid() {
		itr.bpos--
		itr.bi = nil
		itr.prev()
		return
	}
}

// Key follows the y.Iterator interface
func (itr *Iterator) Key() []byte {
	return itr.bi.Key()
}

// Value follows the y.Iterator interface
func (itr *Iterator) Value() (ret y.ValueStruct) {
	ret.Decode(itr.bi.Value())
	return
}

// Next follows the y.Iterator interface
func (itr *Iterator) Next() {
	if !itr.reversed {
		itr.next()
	} else {
		itr.prev()
	}
}

// Rewind follows the y.Iterator interface
func (itr *Iterator) Rewind() {
	if !itr.reversed {
		itr.seekToFirst()
	} else {
		itr.seekToLast()
	}
}

// Seek follows the y.Iterator interface
func (itr *Iterator) Seek(key []byte) {
	if !itr.reversed {
		itr.seek(key)
	} else {
		itr.seekForPrev(key)
	}
}

// ConcatIterator concatenates the sequences defined by several iterators.  (It only works with
// TableIterators, probably just because it's faster to not be so generic.)
type ConcatIterator struct {
	idx      int // Which iterator is active now.
	cur      *Iterator
	iters    []*Iterator // Corresponds to tables.
	tables   []*Table    // Disregarding reversed, this is in ascending order.
	reversed bool
}

// NewConcatIterator creates a new concatenated iterator
func NewConcatIterator(tbls []*Table, reversed bool) *ConcatIterator {
	iters := make([]*Iterator, len(tbls))
	for i := 0; i < len(tbls); i++ {
		iters[i] = tbls[i].NewIterator(reversed)
	}
	return &ConcatIterator{
		reversed: reversed,
		iters:    iters,
		tables:   tbls,
		idx:      -1, // Not really necessary because s.it.Valid()=false, but good to have.
	}
}

func (s *ConcatIterator) setIdx(idx int) {
	s.idx = idx
	if idx < 0 || idx >= len(s.iters) {
		s.cur = nil
	} else {
		s.cur = s.iters[s.idx]
	}
}

// Rewind implements y.Interface
func (s *ConcatIterator) Rewind() {
	if len(s.iters) == 0 {
		return
	}
	if !s.reversed {
		s.setIdx(0)
	} else {
		s.setIdx(len(s.iters) - 1)
	}
	s.cur.Rewind()
}

// Valid implements y.Interface
func (s *ConcatIterator) Valid() bool {
	return s.cur != nil && s.cur.Valid()
}

// Key implements y.Interface
func (s *ConcatIterator) Key() []byte {
	return s.cur.Key()
}

// Value implements y.Interface
func (s *ConcatIterator) Value() y.ValueStruct {
	return s.cur.Value()
}

// Seek brings us to element >= key if reversed is false. Otherwise, <= key.
func (s *ConcatIterator) Seek(key []byte) {
	var idx int
	if !s.reversed {
		idx = sort.Search(len(s.tables), func(i int) bool {
			return y.CompareKeys(s.tables[i].Biggest(), key) >= 0
		})
	} else {
		n := len(s.tables)
		idx = n - 1 - sort.Search(n, func(i int) bool {
			return y.CompareKeys(s.tables[n-1-i].Smallest(), key) <= 0
		})
	}
	if idx >= len(s.tables) || idx < 0 {
		s.setIdx(-1)
		return
	}
	// For reversed=false, we know s.tables[i-1].Biggest() < key. Thus, the
	// previous table cannot possibly contain key.
	s.setIdx(idx)
	s.cur.Seek(key)
}

// Next advances our concat iterator.
func (s *ConcatIterator) Next() {
	s.cur.Next()
	if s.cur.Valid() {
		// Nothing to do. Just stay with the current table.
		return
	}
	for { // In case there are empty tables.
		if !s.reversed {
			s.setIdx(s.idx + 1)
		} else {
			s.setIdx(s.idx - 1)
		}
		if s.cur == nil {
			// End of list. Valid will become false.
			return
		}
		s.cur.Rewind()
		if s.cur.Valid() {
			break
		}
	}
}

// Close implements y.Interface.
func (s *ConcatIterator) Close() error {
	for _, it := range s.iters {
		if err := it.Close(); err != nil {
			return errors.Wrap(err, "ConcatIterator")
		}
	}
	return nil
}
