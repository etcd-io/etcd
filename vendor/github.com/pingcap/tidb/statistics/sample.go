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
	"fmt"
	"math/rand"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/sqlexec"
	"github.com/pingcap/tipb/go-tipb"
	"golang.org/x/net/context"
)

// SampleCollector will collect Samples and calculate the count and ndv of an attribute.
type SampleCollector struct {
	Samples       []types.Datum
	seenValues    int64 // seenValues is the current seen values.
	IsMerger      bool
	NullCount     int64
	Count         int64 // Count is the number of non-null rows.
	MaxSampleSize int64
	FMSketch      *FMSketch
	CMSketch      *CMSketch
	TotalSize     int64 // TotalSize is the total size of column.
}

// MergeSampleCollector merges two sample collectors.
func (c *SampleCollector) MergeSampleCollector(sc *stmtctx.StatementContext, rc *SampleCollector) {
	c.NullCount += rc.NullCount
	c.Count += rc.Count
	c.TotalSize += rc.TotalSize
	c.FMSketch.mergeFMSketch(rc.FMSketch)
	if rc.CMSketch != nil {
		err := c.CMSketch.MergeCMSketch(rc.CMSketch)
		terror.Log(errors.Trace(err))
	}
	for _, val := range rc.Samples {
		err := c.collect(sc, val)
		terror.Log(errors.Trace(err))
	}
}

// SampleCollectorToProto converts SampleCollector to its protobuf representation.
func SampleCollectorToProto(c *SampleCollector) *tipb.SampleCollector {
	collector := &tipb.SampleCollector{
		NullCount: c.NullCount,
		Count:     c.Count,
		FmSketch:  FMSketchToProto(c.FMSketch),
		TotalSize: &c.TotalSize,
	}
	if c.CMSketch != nil {
		collector.CmSketch = CMSketchToProto(c.CMSketch)
	}
	for _, sample := range c.Samples {
		collector.Samples = append(collector.Samples, sample.GetBytes())
	}
	return collector
}

const maxSampleValueLength = mysql.MaxFieldVarCharLength / 2

// SampleCollectorFromProto converts SampleCollector from its protobuf representation.
func SampleCollectorFromProto(collector *tipb.SampleCollector) *SampleCollector {
	s := &SampleCollector{
		NullCount: collector.NullCount,
		Count:     collector.Count,
		FMSketch:  FMSketchFromProto(collector.FmSketch),
	}
	if collector.TotalSize != nil {
		s.TotalSize = *collector.TotalSize
	}
	s.CMSketch = CMSketchFromProto(collector.CmSketch)
	for _, val := range collector.Samples {
		// When store the histogram bucket boundaries to kv, we need to limit the length of the value.
		if len(val) <= maxSampleValueLength {
			s.Samples = append(s.Samples, types.NewBytesDatum(val))
		}
	}
	return s
}

func (c *SampleCollector) collect(sc *stmtctx.StatementContext, d types.Datum) error {
	if !c.IsMerger {
		if d.IsNull() {
			c.NullCount++
			return nil
		}
		c.Count++
		if err := c.FMSketch.InsertValue(sc, d); err != nil {
			return errors.Trace(err)
		}
		if c.CMSketch != nil {
			c.CMSketch.InsertBytes(d.GetBytes())
		}
		// Minus one is to remove the flag byte.
		c.TotalSize += int64(len(d.GetBytes()) - 1)
	}
	c.seenValues++
	// The following code use types.CopyDatum(d) because d may have a deep reference
	// to the underlying slice, GC can't free them which lead to memory leak eventually.
	// TODO: Refactor the proto to avoid copying here.
	if len(c.Samples) < int(c.MaxSampleSize) {
		c.Samples = append(c.Samples, types.CopyDatum(d))
	} else {
		shouldAdd := rand.Int63n(c.seenValues) < c.MaxSampleSize
		if shouldAdd {
			idx := rand.Intn(int(c.MaxSampleSize))
			c.Samples[idx] = types.CopyDatum(d)
		}
	}
	return nil
}

// SampleBuilder is used to build samples for columns.
// Also, if primary key is handle, it will directly build histogram for it.
type SampleBuilder struct {
	Sc              *stmtctx.StatementContext
	RecordSet       sqlexec.RecordSet
	ColLen          int // ColLen is the number of columns need to be sampled.
	PkBuilder       *SortedBuilder
	MaxBucketSize   int64
	MaxSampleSize   int64
	MaxFMSketchSize int64
	CMSketchDepth   int32
	CMSketchWidth   int32
}

// CollectColumnStats collects sample from the result set using Reservoir Sampling algorithm,
// and estimates NDVs using FM Sketch during the collecting process.
// It returns the sample collectors which contain total count, null count, distinct values count and CM Sketch.
// It also returns the statistic builder for PK which contains the histogram.
// See https://en.wikipedia.org/wiki/Reservoir_sampling
func (s SampleBuilder) CollectColumnStats() ([]*SampleCollector, *SortedBuilder, error) {
	collectors := make([]*SampleCollector, s.ColLen)
	for i := range collectors {
		collectors[i] = &SampleCollector{
			MaxSampleSize: s.MaxSampleSize,
			FMSketch:      NewFMSketch(int(s.MaxFMSketchSize)),
		}
	}
	if s.CMSketchDepth > 0 && s.CMSketchWidth > 0 {
		for i := range collectors {
			collectors[i].CMSketch = NewCMSketch(s.CMSketchDepth, s.CMSketchWidth)
		}
	}
	ctx := context.TODO()
	chk := s.RecordSet.NewChunk()
	it := chunk.NewIterator4Chunk(chk)
	for {
		err := s.RecordSet.Next(ctx, chk)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		if chk.NumRows() == 0 {
			return collectors, s.PkBuilder, nil
		}
		if len(s.RecordSet.Fields()) == 0 {
			panic(fmt.Sprintf("%T", s.RecordSet))
		}
		for row := it.Begin(); row != it.End(); row = it.Next() {
			datums := RowToDatums(row, s.RecordSet.Fields())
			if s.PkBuilder != nil {
				err = s.PkBuilder.Iterate(datums[0])
				if err != nil {
					return nil, nil, errors.Trace(err)
				}
				datums = datums[1:]
			}
			for i, val := range datums {
				err = collectors[i].collect(s.Sc, val)
				if err != nil {
					return nil, nil, errors.Trace(err)
				}
			}
		}
	}
}

// RowToDatums converts row to datum slice.
func RowToDatums(row chunk.Row, fields []*ast.ResultField) []types.Datum {
	datums := make([]types.Datum, len(fields))
	for i, f := range fields {
		datums[i] = row.GetDatum(i, &f.Column.FieldType)
	}
	return datums
}
