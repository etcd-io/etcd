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
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
)

// SortedBuilder is used to build histograms for PK and index.
type SortedBuilder struct {
	sc              *stmtctx.StatementContext
	numBuckets      int64
	valuesPerBucket int64
	lastNumber      int64
	bucketIdx       int64
	Count           int64
	hist            *Histogram
}

// NewSortedBuilder creates a new SortedBuilder.
func NewSortedBuilder(sc *stmtctx.StatementContext, numBuckets, id int64, tp *types.FieldType) *SortedBuilder {
	return &SortedBuilder{
		sc:              sc,
		numBuckets:      numBuckets,
		valuesPerBucket: 1,
		hist:            NewHistogram(id, 0, 0, 0, tp, int(numBuckets), 0),
	}
}

// Hist returns the histogram built by SortedBuilder.
func (b *SortedBuilder) Hist() *Histogram {
	return b.hist
}

// Iterate updates the histogram incrementally.
func (b *SortedBuilder) Iterate(data types.Datum) error {
	b.Count++
	if b.Count == 1 {
		b.hist.AppendBucket(&data, &data, 1, 1)
		b.hist.NDV = 1
		return nil
	}
	cmp, err := b.hist.GetUpper(int(b.bucketIdx)).CompareDatum(b.sc, &data)
	if err != nil {
		return errors.Trace(err)
	}
	if cmp == 0 {
		// The new item has the same value as current bucket value, to ensure that
		// a same value only stored in a single bucket, we do not increase bucketIdx even if it exceeds
		// valuesPerBucket.
		b.hist.Buckets[b.bucketIdx].Count++
		b.hist.Buckets[b.bucketIdx].Repeat++
	} else if b.hist.Buckets[b.bucketIdx].Count+1-b.lastNumber <= b.valuesPerBucket {
		// The bucket still have room to store a new item, update the bucket.
		b.hist.updateLastBucket(&data, b.hist.Buckets[b.bucketIdx].Count+1, 1)
		b.hist.NDV++
	} else {
		// All buckets are full, we should merge buckets.
		if b.bucketIdx+1 == b.numBuckets {
			b.hist.mergeBuckets(int(b.bucketIdx))
			b.valuesPerBucket *= 2
			b.bucketIdx = b.bucketIdx / 2
			if b.bucketIdx == 0 {
				b.lastNumber = 0
			} else {
				b.lastNumber = b.hist.Buckets[b.bucketIdx-1].Count
			}
		}
		// We may merge buckets, so we should check it again.
		if b.hist.Buckets[b.bucketIdx].Count+1-b.lastNumber <= b.valuesPerBucket {
			b.hist.updateLastBucket(&data, b.hist.Buckets[b.bucketIdx].Count+1, 1)
		} else {
			b.lastNumber = b.hist.Buckets[b.bucketIdx].Count
			b.bucketIdx++
			b.hist.AppendBucket(&data, &data, b.lastNumber+1, 1)
		}
		b.hist.NDV++
	}
	return nil
}

// BuildColumn builds histogram from samples for column.
func BuildColumn(ctx sessionctx.Context, numBuckets, id int64, collector *SampleCollector, tp *types.FieldType) (*Histogram, error) {
	count := collector.Count
	ndv := collector.FMSketch.NDV()
	if ndv > count {
		ndv = count
	}
	if count == 0 || len(collector.Samples) == 0 {
		return NewHistogram(id, ndv, collector.NullCount, 0, tp, 0, collector.TotalSize), nil
	}
	sc := ctx.GetSessionVars().StmtCtx
	samples := collector.Samples
	err := types.SortDatums(sc, samples)
	if err != nil {
		return nil, errors.Trace(err)
	}
	hg := NewHistogram(id, ndv, collector.NullCount, 0, tp, int(numBuckets), collector.TotalSize)

	sampleNum := int64(len(samples))
	// As we use samples to build the histogram, the bucket number and repeat should multiply a factor.
	sampleFactor := float64(count) / float64(len(samples))
	// Since bucket count is increased by sampleFactor, so the actual max values per bucket is
	// floor(valuesPerBucket/sampleFactor)*sampleFactor, which may less than valuesPerBucket,
	// thus we need to add a sampleFactor to avoid building too many buckets.
	valuesPerBucket := float64(count)/float64(numBuckets) + sampleFactor
	ndvFactor := float64(count) / float64(hg.NDV)
	if ndvFactor > sampleFactor {
		ndvFactor = sampleFactor
	}
	bucketIdx := 0
	var lastCount int64
	hg.AppendBucket(&samples[0], &samples[0], int64(sampleFactor), int64(ndvFactor))
	for i := int64(1); i < sampleNum; i++ {
		cmp, err := hg.GetUpper(bucketIdx).CompareDatum(sc, &samples[i])
		if err != nil {
			return nil, errors.Trace(err)
		}
		totalCount := float64(i+1) * sampleFactor
		if cmp == 0 {
			// The new item has the same value as current bucket value, to ensure that
			// a same value only stored in a single bucket, we do not increase bucketIdx even if it exceeds
			// valuesPerBucket.
			hg.Buckets[bucketIdx].Count = int64(totalCount)
			if float64(hg.Buckets[bucketIdx].Repeat) == ndvFactor {
				hg.Buckets[bucketIdx].Repeat = int64(2 * sampleFactor)
			} else {
				hg.Buckets[bucketIdx].Repeat += int64(sampleFactor)
			}
		} else if totalCount-float64(lastCount) <= valuesPerBucket {
			// The bucket still have room to store a new item, update the bucket.
			hg.updateLastBucket(&samples[i], int64(totalCount), int64(ndvFactor))
		} else {
			lastCount = hg.Buckets[bucketIdx].Count
			// The bucket is full, store the item in the next bucket.
			bucketIdx++
			hg.AppendBucket(&samples[i], &samples[i], int64(totalCount), int64(ndvFactor))
		}
	}
	return hg, nil
}

// AnalyzeResult is used to represent analyze result.
type AnalyzeResult struct {
	// PhysicalTableID is the id of a partition or a table.
	PhysicalTableID int64
	Hist            []*Histogram
	Cms             []*CMSketch
	Count           int64
	IsIndex         int
	Err             error
}
