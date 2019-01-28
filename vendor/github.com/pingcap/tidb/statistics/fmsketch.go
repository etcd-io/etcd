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

package statistics

import (
	"hash"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tipb/go-tipb"
	"github.com/spaolacci/murmur3"
)

// FMSketch is used to count the number of distinct elements in a set.
type FMSketch struct {
	hashset  map[uint64]bool
	mask     uint64
	maxSize  int
	hashFunc hash.Hash64
}

// NewFMSketch returns a new FM sketch.
func NewFMSketch(maxSize int) *FMSketch {
	return &FMSketch{
		hashset:  make(map[uint64]bool),
		maxSize:  maxSize,
		hashFunc: murmur3.New64(),
	}
}

// NDV returns the ndv of the sketch.
func (s *FMSketch) NDV() int64 {
	return int64(s.mask+1) * int64(len(s.hashset))
}

func (s *FMSketch) insertHashValue(hashVal uint64) {
	if (hashVal & s.mask) != 0 {
		return
	}
	s.hashset[hashVal] = true
	if len(s.hashset) > s.maxSize {
		s.mask = s.mask*2 + 1
		for key := range s.hashset {
			if (key & s.mask) != 0 {
				delete(s.hashset, key)
			}
		}
	}
}

// InsertValue inserts a value into the FM sketch.
func (s *FMSketch) InsertValue(sc *stmtctx.StatementContext, value types.Datum) error {
	bytes, err := codec.EncodeValue(sc, nil, value)
	if err != nil {
		return errors.Trace(err)
	}
	s.hashFunc.Reset()
	_, err = s.hashFunc.Write(bytes)
	if err != nil {
		return errors.Trace(err)
	}
	s.insertHashValue(s.hashFunc.Sum64())
	return nil
}

func buildFMSketch(sc *stmtctx.StatementContext, values []types.Datum, maxSize int) (*FMSketch, int64, error) {
	s := NewFMSketch(maxSize)
	for _, value := range values {
		err := s.InsertValue(sc, value)
		if err != nil {
			return nil, 0, errors.Trace(err)
		}
	}
	return s, s.NDV(), nil
}

func (s *FMSketch) mergeFMSketch(rs *FMSketch) {
	if s.mask < rs.mask {
		s.mask = rs.mask
		for key := range s.hashset {
			if (key & s.mask) != 0 {
				delete(s.hashset, key)
			}
		}
	}
	for key := range rs.hashset {
		s.insertHashValue(key)
	}
}

// FMSketchToProto converts FMSketch to its protobuf representation.
func FMSketchToProto(s *FMSketch) *tipb.FMSketch {
	protoSketch := new(tipb.FMSketch)
	protoSketch.Mask = s.mask
	for val := range s.hashset {
		protoSketch.Hashset = append(protoSketch.Hashset, val)
	}
	return protoSketch
}

// FMSketchFromProto converts FMSketch from its protobuf representation.
func FMSketchFromProto(protoSketch *tipb.FMSketch) *FMSketch {
	sketch := &FMSketch{
		hashset: make(map[uint64]bool),
		mask:    protoSketch.Mask,
	}
	for _, val := range protoSketch.Hashset {
		sketch.hashset[val] = true
	}
	return sketch
}
