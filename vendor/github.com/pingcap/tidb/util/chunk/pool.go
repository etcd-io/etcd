// Copyright 2018 PingCAP, Inc.
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

package chunk

import (
	"sync"

	"github.com/pingcap/tidb/types"
)

// Pool is the column pool.
// NOTE: Pool is non-copyable.
type Pool struct {
	initCap int

	varLenColPool   *sync.Pool
	fixLenColPool4  *sync.Pool
	fixLenColPool8  *sync.Pool
	fixLenColPool16 *sync.Pool
	fixLenColPool40 *sync.Pool
}

// NewPool creates a new Pool.
func NewPool(initCap int) *Pool {
	return &Pool{
		initCap:         initCap,
		varLenColPool:   &sync.Pool{New: func() interface{} { return newVarLenColumn(initCap, nil) }},
		fixLenColPool4:  &sync.Pool{New: func() interface{} { return newFixedLenColumn(4, initCap) }},
		fixLenColPool8:  &sync.Pool{New: func() interface{} { return newFixedLenColumn(8, initCap) }},
		fixLenColPool16: &sync.Pool{New: func() interface{} { return newFixedLenColumn(16, initCap) }},
		fixLenColPool40: &sync.Pool{New: func() interface{} { return newFixedLenColumn(40, initCap) }},
	}
}

// GetChunk gets a Chunk from the Pool.
func (p *Pool) GetChunk(fields []*types.FieldType) *Chunk {
	chk := new(Chunk)
	chk.capacity = p.initCap
	chk.columns = make([]*column, len(fields))
	for i, f := range fields {
		switch elemLen := getFixedLen(f); elemLen {
		case varElemLen:
			chk.columns[i] = p.varLenColPool.Get().(*column)
		case 4:
			chk.columns[i] = p.fixLenColPool4.Get().(*column)
		case 8:
			chk.columns[i] = p.fixLenColPool8.Get().(*column)
		case 16:
			chk.columns[i] = p.fixLenColPool16.Get().(*column)
		case 40:
			chk.columns[i] = p.fixLenColPool40.Get().(*column)
		}
	}
	return chk
}

// PutChunk puts a Chunk back to the Pool.
func (p *Pool) PutChunk(fields []*types.FieldType, chk *Chunk) {
	for i, f := range fields {
		switch elemLen := getFixedLen(f); elemLen {
		case varElemLen:
			p.varLenColPool.Put(chk.columns[i])
		case 4:
			p.fixLenColPool4.Put(chk.columns[i])
		case 8:
			p.fixLenColPool8.Put(chk.columns[i])
		case 16:
			p.fixLenColPool16.Put(chk.columns[i])
		case 40:
			p.fixLenColPool40.Put(chk.columns[i])
		}
	}
	chk.columns = nil // release the column references.
}
