// Copyright 2017 PingCAP, Inc.
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

package mvmap

import (
	"bytes"
)

type entry struct {
	addr   dataAddr
	keyLen uint32
	valLen uint32
	next   entryAddr
}

type entryStore struct {
	slices   [][]entry
	sliceIdx uint32
	sliceLen uint32
}

type dataStore struct {
	slices   [][]byte
	sliceIdx uint32
	sliceLen uint32
}

type entryAddr struct {
	sliceIdx uint32
	offset   uint32
}

type dataAddr struct {
	sliceIdx uint32
	offset   uint32
}

const (
	maxDataSliceLen  = 64 * 1024
	maxEntrySliceLen = 8 * 1024
)

func (ds *dataStore) put(key, value []byte) dataAddr {
	dataLen := uint32(len(key) + len(value))
	if ds.sliceLen != 0 && ds.sliceLen+dataLen > maxDataSliceLen {
		ds.slices = append(ds.slices, make([]byte, 0, max(maxDataSliceLen, int(dataLen))))
		ds.sliceLen = 0
		ds.sliceIdx++
	}
	addr := dataAddr{sliceIdx: ds.sliceIdx, offset: ds.sliceLen}
	slice := ds.slices[ds.sliceIdx]
	slice = append(slice, key...)
	slice = append(slice, value...)
	ds.slices[ds.sliceIdx] = slice
	ds.sliceLen += dataLen
	return addr
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (ds *dataStore) get(e entry, key []byte) []byte {
	slice := ds.slices[e.addr.sliceIdx]
	valOffset := e.addr.offset + e.keyLen
	if bytes.Compare(key, slice[e.addr.offset:valOffset]) != 0 {
		return nil
	}
	return slice[valOffset : valOffset+e.valLen]
}

func (ds *dataStore) getEntryData(e entry) (key, value []byte) {
	slice := ds.slices[e.addr.sliceIdx]
	keyOffset := e.addr.offset
	key = slice[keyOffset : keyOffset+e.keyLen]
	valOffset := e.addr.offset + e.keyLen
	value = slice[valOffset : valOffset+e.valLen]
	return
}

var nullEntryAddr = entryAddr{}

func (es *entryStore) put(e entry) entryAddr {
	if es.sliceLen == maxEntrySliceLen {
		es.slices = append(es.slices, make([]entry, 0, maxEntrySliceLen))
		es.sliceLen = 0
		es.sliceIdx++
	}
	addr := entryAddr{sliceIdx: es.sliceIdx, offset: es.sliceLen}
	slice := es.slices[es.sliceIdx]
	slice = append(slice, e)
	es.slices[es.sliceIdx] = slice
	es.sliceLen++
	return addr
}

func (es *entryStore) get(addr entryAddr) entry {
	return es.slices[addr.sliceIdx][addr.offset]
}

// MVMap stores multiple value for a given key with minimum GC overhead.
// A given key can store multiple values.
// It is not thread-safe, should only be used in one goroutine.
type MVMap struct {
	entryStore entryStore
	dataStore  dataStore
	hashTable  map[uint64]entryAddr
	length     int
}

// NewMVMap creates a new multi-value map.
func NewMVMap() *MVMap {
	m := new(MVMap)
	m.hashTable = make(map[uint64]entryAddr)
	m.entryStore.slices = [][]entry{make([]entry, 0, 64)}
	// Append the first empty entry, so the zero entryAddr can represent null.
	m.entryStore.put(entry{})
	m.dataStore.slices = [][]byte{make([]byte, 0, 1024)}
	return m
}

// Put puts the key/value pairs to the MVMap, if the key already exists, old value will not be overwritten,
// values are stored in a list.
func (m *MVMap) Put(key, value []byte) {
	hashKey := fnvHash64(key)
	oldEntryAddr := m.hashTable[hashKey]
	dataAddr := m.dataStore.put(key, value)
	e := entry{
		addr:   dataAddr,
		keyLen: uint32(len(key)),
		valLen: uint32(len(value)),
		next:   oldEntryAddr,
	}
	newEntryAddr := m.entryStore.put(e)
	m.hashTable[hashKey] = newEntryAddr
	m.length++
}

// Get gets the values of the "key" and appends them to "values".
func (m *MVMap) Get(key []byte, values [][]byte) [][]byte {
	hashKey := fnvHash64(key)
	entryAddr := m.hashTable[hashKey]
	for entryAddr != nullEntryAddr {
		e := m.entryStore.get(entryAddr)
		entryAddr = e.next
		val := m.dataStore.get(e, key)
		if val == nil {
			continue
		}
		values = append(values, val)
	}
	// Keep the order of input.
	for i := 0; i < len(values)/2; i++ {
		j := len(values) - 1 - i
		values[i], values[j] = values[j], values[i]
	}
	return values
}

// Len returns the number of values in th mv map, the number of keys may be less than Len
// if the same key is put more than once.
func (m *MVMap) Len() int {
	return m.length
}

// Iterator is used to iterate the MVMap.
type Iterator struct {
	m        *MVMap
	sliceCur int
	entryCur int
}

// Next returns the next key/value pair of the MVMap.
// It returns (nil, nil) when there is no more entries to iterate.
func (i *Iterator) Next() (key, value []byte) {
	for {
		if i.sliceCur >= len(i.m.entryStore.slices) {
			return nil, nil
		}
		entrySlice := i.m.entryStore.slices[i.sliceCur]
		if i.entryCur >= len(entrySlice) {
			i.sliceCur++
			i.entryCur = 0
			continue
		}
		entry := entrySlice[i.entryCur]
		key, value = i.m.dataStore.getEntryData(entry)
		i.entryCur++
		return
	}
}

// NewIterator creates a iterator for the MVMap.
func (m *MVMap) NewIterator() *Iterator {
	// The first entry is empty, so init entryCur to 1.
	return &Iterator{m: m, entryCur: 1}
}
