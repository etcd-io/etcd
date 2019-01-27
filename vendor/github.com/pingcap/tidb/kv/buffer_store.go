// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import (
	"github.com/pingcap/errors"
)

var (
	// DefaultTxnMembufCap is the default transaction membuf capability.
	DefaultTxnMembufCap = 4 * 1024
	// ImportingTxnMembufCap is the capability of tidb importing data situation.
	ImportingTxnMembufCap = 32 * 1024
	// TempTxnMemBufCap is the capability of temporary membuf.
	TempTxnMemBufCap = 64
)

// BufferStore wraps a Retriever for read and a MemBuffer for buffered write.
// Common usage pattern:
//	bs := NewBufferStore(r) // use BufferStore to wrap a Retriever
//	// ...
//	// read/write on bs
//	// ...
//	bs.SaveTo(m)	        // save above operations to a Mutator
type BufferStore struct {
	MemBuffer
	r Retriever
}

// NewBufferStore creates a BufferStore using r for read.
func NewBufferStore(r Retriever, cap int) *BufferStore {
	if cap <= 0 {
		cap = DefaultTxnMembufCap
	}
	return &BufferStore{
		r:         r,
		MemBuffer: &lazyMemBuffer{cap: cap},
	}
}

// Reset resets s.MemBuffer.
func (s *BufferStore) Reset() {
	s.MemBuffer.Reset()
}

// SetCap sets the MemBuffer capability.
func (s *BufferStore) SetCap(cap int) {
	s.MemBuffer.SetCap(cap)
}

// Get implements the Retriever interface.
func (s *BufferStore) Get(k Key) ([]byte, error) {
	val, err := s.MemBuffer.Get(k)
	if IsErrNotFound(err) {
		val, err = s.r.Get(k)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(val) == 0 {
		return nil, ErrNotExist
	}
	return val, nil
}

// Iter implements the Retriever interface.
func (s *BufferStore) Iter(k Key, upperBound Key) (Iterator, error) {
	bufferIt, err := s.MemBuffer.Iter(k, upperBound)
	if err != nil {
		return nil, errors.Trace(err)
	}
	retrieverIt, err := s.r.Iter(k, upperBound)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return NewUnionIter(bufferIt, retrieverIt, false)
}

// IterReverse implements the Retriever interface.
func (s *BufferStore) IterReverse(k Key) (Iterator, error) {
	bufferIt, err := s.MemBuffer.IterReverse(k)
	if err != nil {
		return nil, errors.Trace(err)
	}
	retrieverIt, err := s.r.IterReverse(k)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return NewUnionIter(bufferIt, retrieverIt, true)
}

// WalkBuffer iterates all buffered kv pairs.
func (s *BufferStore) WalkBuffer(f func(k Key, v []byte) error) error {
	return errors.Trace(WalkMemBuffer(s.MemBuffer, f))
}

// SaveTo saves all buffered kv pairs into a Mutator.
func (s *BufferStore) SaveTo(m Mutator) error {
	err := s.WalkBuffer(func(k Key, v []byte) error {
		if len(v) == 0 {
			return errors.Trace(m.Delete(k))
		}
		return errors.Trace(m.Set(k, v))
	})
	return errors.Trace(err)
}
