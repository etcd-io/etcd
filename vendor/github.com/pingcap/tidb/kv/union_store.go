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
	"bytes"

	"github.com/pingcap/errors"
)

// UnionStore is a store that wraps a snapshot for read and a BufferStore for buffered write.
// Also, it provides some transaction related utilities.
type UnionStore interface {
	MemBuffer
	// CheckLazyConditionPairs loads all lazy values from store then checks if all values are matched.
	// Lazy condition pairs should be checked before transaction commit.
	CheckLazyConditionPairs() error
	// WalkBuffer iterates all buffered kv pairs.
	WalkBuffer(f func(k Key, v []byte) error) error
	// SetOption sets an option with a value, when val is nil, uses the default
	// value of this option.
	SetOption(opt Option, val interface{})
	// DelOption deletes an option.
	DelOption(opt Option)
	// GetOption gets an option.
	GetOption(opt Option) interface{}
	// GetMemBuffer return the MemBuffer binding to this UnionStore.
	GetMemBuffer() MemBuffer
}

// Option is used for customizing kv store's behaviors during a transaction.
type Option int

// Options is an interface of a set of options. Each option is associated with a value.
type Options interface {
	// Get gets an option value.
	Get(opt Option) (v interface{}, ok bool)
}

// conditionPair is used to store lazy check condition.
// If condition not match (value is not equal as expected one), returns err.
type conditionPair struct {
	key   Key
	value []byte
	err   error
}

// unionStore is an in-memory Store which contains a buffer for write and a
// snapshot for read.
type unionStore struct {
	*BufferStore
	snapshot           Snapshot                  // for read
	lazyConditionPairs map[string]*conditionPair // for delay check
	opts               options
}

// NewUnionStore builds a new UnionStore.
func NewUnionStore(snapshot Snapshot) UnionStore {
	return &unionStore{
		BufferStore:        NewBufferStore(snapshot, DefaultTxnMembufCap),
		snapshot:           snapshot,
		lazyConditionPairs: make(map[string]*conditionPair),
		opts:               make(map[Option]interface{}),
	}
}

// invalidIterator implements Iterator interface.
// It is used for read-only transaction which has no data written, the iterator is always invalid.
type invalidIterator struct{}

func (it invalidIterator) Valid() bool {
	return false
}

func (it invalidIterator) Next() error {
	return nil
}

func (it invalidIterator) Key() Key {
	return nil
}

func (it invalidIterator) Value() []byte {
	return nil
}

func (it invalidIterator) Close() {}

// lazyMemBuffer wraps a MemBuffer which is to be initialized when it is modified.
type lazyMemBuffer struct {
	mb  MemBuffer
	cap int
}

func (lmb *lazyMemBuffer) Get(k Key) ([]byte, error) {
	if lmb.mb == nil {
		return nil, ErrNotExist
	}

	return lmb.mb.Get(k)
}

func (lmb *lazyMemBuffer) Set(key Key, value []byte) error {
	if lmb.mb == nil {
		lmb.mb = NewMemDbBuffer(lmb.cap)
	}

	return lmb.mb.Set(key, value)
}

func (lmb *lazyMemBuffer) Delete(k Key) error {
	if lmb.mb == nil {
		lmb.mb = NewMemDbBuffer(lmb.cap)
	}

	return lmb.mb.Delete(k)
}

func (lmb *lazyMemBuffer) Iter(k Key, upperBound Key) (Iterator, error) {
	if lmb.mb == nil {
		return invalidIterator{}, nil
	}
	return lmb.mb.Iter(k, upperBound)
}

func (lmb *lazyMemBuffer) IterReverse(k Key) (Iterator, error) {
	if lmb.mb == nil {
		return invalidIterator{}, nil
	}
	return lmb.mb.IterReverse(k)
}

func (lmb *lazyMemBuffer) Size() int {
	if lmb.mb == nil {
		return 0
	}
	return lmb.mb.Size()
}

func (lmb *lazyMemBuffer) Len() int {
	if lmb.mb == nil {
		return 0
	}
	return lmb.mb.Len()
}

func (lmb *lazyMemBuffer) Reset() {
	if lmb.mb != nil {
		lmb.mb.Reset()
	}
}

func (lmb *lazyMemBuffer) SetCap(cap int) {
	lmb.cap = cap
}

// Get implements the Retriever interface.
func (us *unionStore) Get(k Key) ([]byte, error) {
	v, err := us.MemBuffer.Get(k)
	if IsErrNotFound(err) {
		if _, ok := us.opts.Get(PresumeKeyNotExists); ok {
			e, ok := us.opts.Get(PresumeKeyNotExistsError)
			if ok && e != nil {
				us.markLazyConditionPair(k, nil, e.(error))
			} else {
				us.markLazyConditionPair(k, nil, ErrKeyExists)
			}
			return nil, ErrNotExist
		}
	}
	if IsErrNotFound(err) {
		v, err = us.BufferStore.r.Get(k)
	}
	if err != nil {
		return v, errors.Trace(err)
	}
	if len(v) == 0 {
		return nil, ErrNotExist
	}
	return v, nil
}

// markLazyConditionPair marks a kv pair for later check.
// If condition not match, should return e as error.
func (us *unionStore) markLazyConditionPair(k Key, v []byte, e error) {
	us.lazyConditionPairs[string(k)] = &conditionPair{
		key:   k.Clone(),
		value: v,
		err:   e,
	}
}

// CheckLazyConditionPairs implements the UnionStore interface.
func (us *unionStore) CheckLazyConditionPairs() error {
	if len(us.lazyConditionPairs) == 0 {
		return nil
	}
	keys := make([]Key, 0, len(us.lazyConditionPairs))
	for _, v := range us.lazyConditionPairs {
		keys = append(keys, v.key)
	}
	values, err := us.snapshot.BatchGet(keys)
	if err != nil {
		return errors.Trace(err)
	}

	for k, v := range us.lazyConditionPairs {
		if len(v.value) == 0 {
			if _, exist := values[k]; exist {
				return errors.Trace(v.err)
			}
		} else {
			if bytes.Compare(values[k], v.value) != 0 {
				return errors.Trace(ErrLazyConditionPairsNotMatch)
			}
		}
	}
	return nil
}

// SetOption implements the UnionStore SetOption interface.
func (us *unionStore) SetOption(opt Option, val interface{}) {
	us.opts[opt] = val
}

// DelOption implements the UnionStore DelOption interface.
func (us *unionStore) DelOption(opt Option) {
	delete(us.opts, opt)
}

// GetOption implements the UnionStore GetOption interface.
func (us *unionStore) GetOption(opt Option) interface{} {
	return us.opts[opt]
}

// GetMemBuffer return the MemBuffer binding to this UnionStore.
func (us *unionStore) GetMemBuffer() MemBuffer {
	return us.BufferStore.MemBuffer
}

// SetCap sets membuffer capability.
func (us *unionStore) SetCap(cap int) {
	us.BufferStore.SetCap(cap)
}

func (us *unionStore) Reset() {
	us.BufferStore.Reset()
}

type options map[Option]interface{}

func (opts options) Get(opt Option) (interface{}, bool) {
	v, ok := opts[opt]
	return v, ok
}
