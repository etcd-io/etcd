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
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/ranger"
	"github.com/pingcap/tidb/util/sqlexec"
	"github.com/pingcap/tipb/go-tipb"
	"golang.org/x/net/context"
)

// Histogram represents statistics for a column or index.
type Histogram struct {
	ID        int64 // Column ID.
	NDV       int64 // Number of distinct values.
	NullCount int64 // Number of null values.
	// LastUpdateVersion is the version that this histogram updated last time.
	LastUpdateVersion uint64

	tp *types.FieldType

	// Histogram elements.
	//
	// A bucket bound is the smallest and greatest values stored in the bucket. The lower and upper bound
	// are stored in one column.
	//
	// A bucket count is the number of items stored in all previous buckets and the current bucket.
	// Bucket counts are always in increasing order.
	//
	// A bucket repeat is the number of repeats of the bucket value, it can be used to find popular values.
	Bounds  *chunk.Chunk
	Buckets []Bucket

	// Used for estimating fraction of the interval [lower, upper] that lies within the [lower, value].
	// For some types like `Int`, we do not build it because we can get them directly from `Bounds`.
	scalars []scalar
	// TotColSize is the total column size for the histogram.
	TotColSize int64
}

// Bucket store the bucket count and repeat.
type Bucket struct {
	Count  int64
	Repeat int64
}

type scalar struct {
	lower        float64
	upper        float64
	commonPfxLen int // commonPfxLen is the common prefix length of the lower bound and upper bound when the value type is KindString or KindBytes.
}

// NewHistogram creates a new histogram.
func NewHistogram(id, ndv, nullCount int64, version uint64, tp *types.FieldType, bucketSize int, totColSize int64) *Histogram {
	return &Histogram{
		ID:                id,
		NDV:               ndv,
		NullCount:         nullCount,
		LastUpdateVersion: version,
		tp:                tp,
		Bounds:            chunk.NewChunkWithCapacity([]*types.FieldType{tp}, 2*bucketSize),
		Buckets:           make([]Bucket, 0, bucketSize),
		TotColSize:        totColSize,
	}
}

// GetLower gets the lower bound of bucket `idx`.
func (hg *Histogram) GetLower(idx int) *types.Datum {
	d := hg.Bounds.GetRow(2*idx).GetDatum(0, hg.tp)
	return &d
}

// GetUpper gets the upper bound of bucket `idx`.
func (hg *Histogram) GetUpper(idx int) *types.Datum {
	d := hg.Bounds.GetRow(2*idx+1).GetDatum(0, hg.tp)
	return &d
}

// AvgColSize is the average column size of the histogram.
func (c *Column) AvgColSize(count int64) float64 {
	if count == 0 {
		return 0
	}
	switch c.Histogram.tp.Tp {
	case mysql.TypeFloat:
		return 4
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong,
		mysql.TypeDouble, mysql.TypeYear:
		return 8
	case mysql.TypeDuration, mysql.TypeDate, mysql.TypeDatetime, mysql.TypeTimestamp:
		return 16
	case mysql.TypeNewDecimal:
		return types.MyDecimalStructSize
	default:
		// Keep two decimal place.
		return math.Round(float64(c.TotColSize)/float64(count)*100) / 100
	}
}

// AppendBucket appends a bucket into `hg`.
func (hg *Histogram) AppendBucket(lower *types.Datum, upper *types.Datum, count, repeat int64) {
	hg.Buckets = append(hg.Buckets, Bucket{Count: count, Repeat: repeat})
	hg.Bounds.AppendDatum(0, lower)
	hg.Bounds.AppendDatum(0, upper)
}

func (hg *Histogram) updateLastBucket(upper *types.Datum, count, repeat int64) {
	len := hg.Len()
	hg.Bounds.TruncateTo(2*len - 1)
	hg.Bounds.AppendDatum(0, upper)
	hg.Buckets[len-1] = Bucket{Count: count, Repeat: repeat}
}

// DecodeTo decodes the histogram bucket values into `tp`.
func (hg *Histogram) DecodeTo(tp *types.FieldType, timeZone *time.Location) error {
	oldIter := chunk.NewIterator4Chunk(hg.Bounds)
	hg.Bounds = chunk.NewChunkWithCapacity([]*types.FieldType{tp}, oldIter.Len())
	hg.tp = tp
	for row := oldIter.Begin(); row != oldIter.End(); row = oldIter.Next() {
		datum, err := tablecodec.DecodeColumnValue(row.GetBytes(0), tp, timeZone)
		if err != nil {
			return errors.Trace(err)
		}
		hg.Bounds.AppendDatum(0, &datum)
	}
	return nil
}

// ConvertTo converts the histogram bucket values into `tp`.
func (hg *Histogram) ConvertTo(sc *stmtctx.StatementContext, tp *types.FieldType) (*Histogram, error) {
	hist := NewHistogram(hg.ID, hg.NDV, hg.NullCount, hg.LastUpdateVersion, tp, hg.Len(), hg.TotColSize)
	iter := chunk.NewIterator4Chunk(hg.Bounds)
	for row := iter.Begin(); row != iter.End(); row = iter.Next() {
		d := row.GetDatum(0, hg.tp)
		d, err := d.ConvertTo(sc, tp)
		if err != nil {
			return nil, errors.Trace(err)
		}
		hist.Bounds.AppendDatum(0, &d)
	}
	hist.Buckets = hg.Buckets
	return hist, nil
}

// Len is the number of buckets in the histogram.
func (hg *Histogram) Len() int {
	return len(hg.Buckets)
}

// HistogramEqual tests if two histograms are equal.
func HistogramEqual(a, b *Histogram, ignoreID bool) bool {
	if ignoreID {
		old := b.ID
		b.ID = a.ID
		defer func() { b.ID = old }()
	}
	return bytes.Equal([]byte(a.ToString(0)), []byte(b.ToString(0)))
}

const (
	// constants for stats version
	curStatsVersion = version1
	version1        = 1

	// constants for column flag
	analyzeFlag = 1
)

func isAnalyzed(flag int64) bool {
	return (flag & analyzeFlag) > 0
}

// SaveStatsToStorage saves the stats to storage.
func (h *Handle) SaveStatsToStorage(tableID int64, count int64, isIndex int, hg *Histogram, cms *CMSketch, isAnalyzed int64) (err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx := context.TODO()
	exec := h.mu.ctx.(sqlexec.SQLExecutor)
	_, err = exec.Execute(ctx, "begin")
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		err = finishTransaction(context.Background(), exec, err)
	}()
	txn, err := h.mu.ctx.Txn(true)
	if err != nil {
		return errors.Trace(err)
	}

	version := txn.StartTS()
	var sql string
	// If the count is less than 0, then we do not want to update the modify count and count.
	if count >= 0 {
		sql = fmt.Sprintf("replace into mysql.stats_meta (version, table_id, count) values (%d, %d, %d)", version, tableID, count)
	} else {
		sql = fmt.Sprintf("update mysql.stats_meta set version = %d where table_id = %d", version, tableID)
	}
	_, err = exec.Execute(ctx, sql)
	if err != nil {
		return
	}
	data, err := encodeCMSketch(cms)
	if err != nil {
		return
	}
	flag := 0
	if isAnalyzed == 1 {
		flag = analyzeFlag
	}
	replaceSQL := fmt.Sprintf("replace into mysql.stats_histograms (table_id, is_index, hist_id, distinct_count, version, null_count, cm_sketch, tot_col_size, stats_ver, flag) values (%d, %d, %d, %d, %d, %d, X'%X', %d, %d, %d)",
		tableID, isIndex, hg.ID, hg.NDV, version, hg.NullCount, data, hg.TotColSize, curStatsVersion, flag)
	_, err = exec.Execute(ctx, replaceSQL)
	if err != nil {
		return
	}
	deleteSQL := fmt.Sprintf("delete from mysql.stats_buckets where table_id = %d and is_index = %d and hist_id = %d", tableID, isIndex, hg.ID)
	_, err = exec.Execute(ctx, deleteSQL)
	if err != nil {
		return
	}
	sc := h.mu.ctx.GetSessionVars().StmtCtx
	for i := range hg.Buckets {
		count := hg.Buckets[i].Count
		if i > 0 {
			count -= hg.Buckets[i-1].Count
		}
		var upperBound types.Datum
		upperBound, err = hg.GetUpper(i).ConvertTo(sc, types.NewFieldType(mysql.TypeBlob))
		if err != nil {
			return
		}
		var lowerBound types.Datum
		lowerBound, err = hg.GetLower(i).ConvertTo(sc, types.NewFieldType(mysql.TypeBlob))
		if err != nil {
			return
		}
		insertSQL := fmt.Sprintf("insert into mysql.stats_buckets(table_id, is_index, hist_id, bucket_id, count, repeats, lower_bound, upper_bound) values(%d, %d, %d, %d, %d, %d, X'%X', X'%X')", tableID, isIndex, hg.ID, i, count, hg.Buckets[i].Repeat, lowerBound.GetBytes(), upperBound.GetBytes())
		_, err = exec.Execute(ctx, insertSQL)
		if err != nil {
			return
		}
	}
	return
}

// SaveMetaToStorage will save stats_meta to storage.
func (h *Handle) SaveMetaToStorage(tableID, count, modifyCount int64) (err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx := context.TODO()
	exec := h.mu.ctx.(sqlexec.SQLExecutor)
	_, err = exec.Execute(ctx, "begin")
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		err = finishTransaction(ctx, exec, err)
	}()
	txn, err := h.mu.ctx.Txn(true)
	if err != nil {
		return errors.Trace(err)
	}
	var sql string
	version := txn.StartTS()
	sql = fmt.Sprintf("replace into mysql.stats_meta (version, table_id, count, modify_count) values (%d, %d, %d, %d)", version, tableID, count, modifyCount)
	_, err = exec.Execute(ctx, sql)
	return
}

func (h *Handle) histogramFromStorage(tableID int64, colID int64, tp *types.FieldType, distinct int64, isIndex int, ver uint64, nullCount int64, totColSize int64) (*Histogram, error) {
	selSQL := fmt.Sprintf("select count, repeats, lower_bound, upper_bound from mysql.stats_buckets where table_id = %d and is_index = %d and hist_id = %d order by bucket_id", tableID, isIndex, colID)
	rows, fields, err := h.restrictedExec.ExecRestrictedSQL(nil, selSQL)
	if err != nil {
		return nil, errors.Trace(err)
	}
	bucketSize := len(rows)
	hg := NewHistogram(colID, distinct, nullCount, ver, tp, bucketSize, totColSize)
	totalCount := int64(0)
	for i := 0; i < bucketSize; i++ {
		count := rows[i].GetInt64(0)
		repeats := rows[i].GetInt64(1)
		var upperBound, lowerBound types.Datum
		if isIndex == 1 {
			lowerBound = rows[i].GetDatum(2, &fields[2].Column.FieldType)
			upperBound = rows[i].GetDatum(3, &fields[3].Column.FieldType)
		} else {
			sc := &stmtctx.StatementContext{TimeZone: time.UTC}
			d := rows[i].GetDatum(2, &fields[2].Column.FieldType)
			lowerBound, err = d.ConvertTo(sc, tp)
			if err != nil {
				return nil, errors.Trace(err)
			}
			d = rows[i].GetDatum(3, &fields[3].Column.FieldType)
			upperBound, err = d.ConvertTo(sc, tp)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		totalCount += count
		hg.AppendBucket(&lowerBound, &upperBound, totalCount, repeats)
	}
	hg.PreCalculateScalar()
	return hg, nil
}

func (h *Handle) columnCountFromStorage(tableID, colID int64) (int64, error) {
	selSQL := fmt.Sprintf("select sum(count) from mysql.stats_buckets where table_id = %d and is_index = %d and hist_id = %d", tableID, 0, colID)
	rows, _, err := h.restrictedExec.ExecRestrictedSQL(nil, selSQL)
	if err != nil {
		return 0, errors.Trace(err)
	}
	if rows[0].IsNull(0) {
		return 0, nil
	}
	return rows[0].GetMyDecimal(0).ToInt()
}

// ValueToString converts a possible encoded value to a formatted string. If the value is encoded, then
// idxCols equals to number of origin values, else idxCols is 0.
func ValueToString(value *types.Datum, idxCols int) (string, error) {
	if idxCols == 0 {
		return value.ToString()
	}
	decodedVals, err := codec.DecodeRange(value.GetBytes(), idxCols)
	if err != nil {
		return "", errors.Trace(err)
	}
	str, err := types.DatumsToString(decodedVals, true)
	if err != nil {
		return "", errors.Trace(err)
	}
	return str, nil
}

func (hg *Histogram) bucketToString(bktID, idxCols int) string {
	upperVal, err := ValueToString(hg.GetUpper(bktID), idxCols)
	terror.Log(errors.Trace(err))
	lowerVal, err := ValueToString(hg.GetLower(bktID), idxCols)
	terror.Log(errors.Trace(err))
	return fmt.Sprintf("num: %d lower_bound: %s upper_bound: %s repeats: %d", hg.bucketCount(bktID), lowerVal, upperVal, hg.Buckets[bktID].Repeat)
}

// ToString gets the string representation for the histogram.
func (hg *Histogram) ToString(idxCols int) string {
	strs := make([]string, 0, hg.Len()+1)
	if idxCols > 0 {
		strs = append(strs, fmt.Sprintf("index:%d ndv:%d", hg.ID, hg.NDV))
	} else {
		strs = append(strs, fmt.Sprintf("column:%d ndv:%d totColSize:%d", hg.ID, hg.NDV, hg.TotColSize))
	}
	for i := 0; i < hg.Len(); i++ {
		strs = append(strs, hg.bucketToString(i, idxCols))
	}
	return strings.Join(strs, "\n")
}

// equalRowCount estimates the row count where the column equals to value.
func (hg *Histogram) equalRowCount(value types.Datum) float64 {
	index, match := hg.Bounds.LowerBound(0, &value)
	// Since we store the lower and upper bound together, if the index is an odd number, then it points to a upper bound.
	if index%2 == 1 {
		if match {
			return float64(hg.Buckets[index/2].Repeat)
		}
		return hg.totalRowCount() / float64(hg.NDV)
	}
	if match {
		cmp := chunk.GetCompareFunc(hg.tp)
		if cmp(hg.Bounds.GetRow(index), 0, hg.Bounds.GetRow(index+1), 0) == 0 {
			return float64(hg.Buckets[index/2].Repeat)
		}
		return hg.totalRowCount() / float64(hg.NDV)
	}
	return 0
}

// greaterRowCount estimates the row count where the column greater than value.
func (hg *Histogram) greaterRowCount(value types.Datum) float64 {
	gtCount := hg.totalRowCount() - hg.lessRowCount(value) - hg.equalRowCount(value)
	if gtCount < 0 {
		gtCount = 0
	}
	return gtCount
}

// greaterAndEqRowCount estimates the row count where the column greater than or equal to value.
func (hg *Histogram) greaterAndEqRowCount(value types.Datum) float64 {
	return hg.totalRowCount() - hg.lessRowCount(value)
}

// lessRowCount estimates the row count where the column less than value.
func (hg *Histogram) lessRowCountWithBktIdx(value types.Datum) (float64, int) {
	// all the values is null
	if hg.Bounds.NumRows() == 0 {
		return 0, 0
	}
	index, match := hg.Bounds.LowerBound(0, &value)
	if index == hg.Bounds.NumRows() {
		return hg.totalRowCount(), hg.Len() - 1
	}
	// Since we store the lower and upper bound together, so dividing the index by 2 will get the bucket index.
	bucketIdx := index / 2
	curCount, curRepeat := float64(hg.Buckets[bucketIdx].Count), float64(hg.Buckets[bucketIdx].Repeat)
	preCount := float64(0)
	if bucketIdx > 0 {
		preCount = float64(hg.Buckets[bucketIdx-1].Count)
	}
	if index%2 == 1 {
		if match {
			return curCount - curRepeat, bucketIdx
		}
		return preCount + hg.calcFraction(bucketIdx, &value)*(curCount-curRepeat-preCount), bucketIdx
	}
	return preCount, bucketIdx
}

func (hg *Histogram) lessRowCount(value types.Datum) float64 {
	result, _ := hg.lessRowCountWithBktIdx(value)
	return result
}

// lessAndEqRowCount estimates the row count where the column less than or equal to value.
func (hg *Histogram) lessAndEqRowCount(value types.Datum) float64 {
	return hg.lessRowCount(value) + hg.equalRowCount(value)
}

// betweenRowCount estimates the row count where column greater or equal to a and less than b.
func (hg *Histogram) betweenRowCount(a, b types.Datum) float64 {
	lessCountA := hg.lessRowCount(a)
	lessCountB := hg.lessRowCount(b)
	// If lessCountA is not less than lessCountB, it may be that they fall to the same bucket and we cannot estimate
	// the fraction, so we use `totalCount / NDV` to estimate the row count, but the result should not greater than
	// lessCountB or totalRowCount-lessCountA.
	if lessCountA >= lessCountB && hg.NDV > 0 {
		result := math.Min(lessCountB, hg.totalRowCount()-lessCountA)
		return math.Min(result, hg.totalRowCount()/float64(hg.NDV))
	}
	return lessCountB - lessCountA
}

func (hg *Histogram) totalRowCount() float64 {
	if hg.Len() == 0 {
		return float64(hg.NullCount)
	}
	return float64(hg.Buckets[hg.Len()-1].Count + hg.NullCount)
}

// mergeBuckets is used to merge every two neighbor buckets.
func (hg *Histogram) mergeBuckets(bucketIdx int) {
	curBuck := 0
	c := chunk.NewChunkWithCapacity([]*types.FieldType{hg.tp}, bucketIdx)
	for i := 0; i+1 <= bucketIdx; i += 2 {
		hg.Buckets[curBuck] = hg.Buckets[i+1]
		c.AppendDatum(0, hg.GetLower(i))
		c.AppendDatum(0, hg.GetUpper(i+1))
		curBuck++
	}
	if bucketIdx%2 == 0 {
		hg.Buckets[curBuck] = hg.Buckets[bucketIdx]
		c.AppendDatum(0, hg.GetLower(bucketIdx))
		c.AppendDatum(0, hg.GetUpper(bucketIdx))
		curBuck++
	}
	hg.Bounds = c
	hg.Buckets = hg.Buckets[:curBuck]
	return
}

// getIncreaseFactor will return a factor of data increasing after the last analysis.
func (hg *Histogram) getIncreaseFactor(totalCount int64) float64 {
	columnCount := hg.totalRowCount()
	if columnCount == 0 {
		// avoid dividing by 0
		return 1.0
	}
	return float64(totalCount) / columnCount
}

// validRange checks if the range is valid, it is used by `SplitRange` to remove the invalid range,
// the possible types of range are index key range and handle key range.
func validRange(ran *ranger.Range) bool {
	var low, high []byte
	if ran.LowVal[0].Kind() == types.KindBytes {
		low, high = ran.LowVal[0].GetBytes(), ran.HighVal[0].GetBytes()
	} else {
		low, high = codec.EncodeInt(nil, ran.LowVal[0].GetInt64()), codec.EncodeInt(nil, ran.HighVal[0].GetInt64())
	}
	if ran.LowExclude {
		low = kv.Key(low).PrefixNext()
	}
	if !ran.HighExclude {
		high = kv.Key(high).PrefixNext()
	}
	return bytes.Compare(low, high) < 0
}

// SplitRange splits the range according to the histogram upper bound. Note that we treat last bucket's upper bound
// as inf, so all the split ranges will totally fall in one of the (-inf, u(0)], (u(0), u(1)],...(u(n-3), u(n-2)],
// (u(n-2), +inf), where n is the number of buckets, u(i) is the i-th bucket's upper bound.
func (hg *Histogram) SplitRange(ranges []*ranger.Range) []*ranger.Range {
	split := make([]*ranger.Range, 0, len(ranges))
	for len(ranges) > 0 {
		// Find the last bound that greater or equal to the LowVal.
		idx := hg.Bounds.UpperBound(0, &ranges[0].LowVal[0])
		if !ranges[0].LowExclude && idx > 0 {
			cmp := chunk.Compare(hg.Bounds.GetRow(idx-1), 0, &ranges[0].LowVal[0])
			if cmp == 0 {
				idx--
			}
		}
		// Treat last bucket's upper bound as inf, so we do not need split any more.
		if idx >= hg.Bounds.NumRows()-2 {
			split = append(split, ranges...)
			break
		}
		// Get the corresponding upper bound.
		if idx%2 == 0 {
			idx++
		}
		upperBound := hg.Bounds.GetRow(idx)
		var i int
		// Find the first range that need to be split by the upper bound.
		for ; i < len(ranges); i++ {
			if chunk.Compare(upperBound, 0, &ranges[i].HighVal[0]) < 0 {
				break
			}
		}
		split = append(split, ranges[:i]...)
		ranges = ranges[i:]
		if len(ranges) == 0 {
			break
		}
		// Split according to the upper bound.
		cmp := chunk.Compare(upperBound, 0, &ranges[0].LowVal[0])
		if cmp > 0 || (cmp == 0 && !ranges[0].LowExclude) {
			upper := upperBound.GetDatum(0, hg.tp)
			split = append(split, &ranger.Range{
				LowExclude:  ranges[0].LowExclude,
				LowVal:      []types.Datum{ranges[0].LowVal[0]},
				HighVal:     []types.Datum{upper},
				HighExclude: false})
			ranges[0].LowVal[0] = upper
			ranges[0].LowExclude = true
			if !validRange(ranges[0]) {
				ranges = ranges[1:]
			}
		}
	}
	return split
}

func (hg *Histogram) bucketCount(idx int) int64 {
	if idx == 0 {
		return hg.Buckets[0].Count
	}
	return hg.Buckets[idx].Count - hg.Buckets[idx-1].Count
}

// HistogramToProto converts Histogram to its protobuf representation.
// Note that when this is used, the lower/upper bound in the bucket must be BytesDatum.
func HistogramToProto(hg *Histogram) *tipb.Histogram {
	protoHg := &tipb.Histogram{
		Ndv: hg.NDV,
	}
	for i := 0; i < hg.Len(); i++ {
		bkt := &tipb.Bucket{
			Count:      hg.Buckets[i].Count,
			LowerBound: hg.GetLower(i).GetBytes(),
			UpperBound: hg.GetUpper(i).GetBytes(),
			Repeats:    hg.Buckets[i].Repeat,
		}
		protoHg.Buckets = append(protoHg.Buckets, bkt)
	}
	return protoHg
}

// HistogramFromProto converts Histogram from its protobuf representation.
// Note that we will set BytesDatum for the lower/upper bound in the bucket, the decode will
// be after all histograms merged.
func HistogramFromProto(protoHg *tipb.Histogram) *Histogram {
	tp := types.NewFieldType(mysql.TypeBlob)
	hg := NewHistogram(0, protoHg.Ndv, 0, 0, tp, len(protoHg.Buckets), 0)
	for _, bucket := range protoHg.Buckets {
		lower, upper := types.NewBytesDatum(bucket.LowerBound), types.NewBytesDatum(bucket.UpperBound)
		hg.AppendBucket(&lower, &upper, bucket.Count, bucket.Repeats)
	}
	return hg
}

func (hg *Histogram) popFirstBucket() {
	hg.Buckets = hg.Buckets[1:]
	c := chunk.NewChunkWithCapacity([]*types.FieldType{hg.tp, hg.tp}, hg.Bounds.NumRows()-2)
	c.Append(hg.Bounds, 2, hg.Bounds.NumRows())
	hg.Bounds = c
}

func (hg *Histogram) isIndexHist() bool {
	return hg.tp.Tp == mysql.TypeBlob
}

// MergeHistograms merges two histograms.
func MergeHistograms(sc *stmtctx.StatementContext, lh *Histogram, rh *Histogram, bucketSize int) (*Histogram, error) {
	if lh.Len() == 0 {
		return rh, nil
	}
	if rh.Len() == 0 {
		return lh, nil
	}
	lh.NDV += rh.NDV
	lLen := lh.Len()
	cmp, err := lh.GetUpper(lLen-1).CompareDatum(sc, rh.GetLower(0))
	if err != nil {
		return nil, errors.Trace(err)
	}
	offset := int64(0)
	if cmp == 0 {
		lh.NDV--
		lh.updateLastBucket(rh.GetUpper(0), lh.Buckets[lLen-1].Count+rh.Buckets[0].Count, rh.Buckets[0].Repeat)
		offset = rh.Buckets[0].Count
		rh.popFirstBucket()
	}
	for lh.Len() > bucketSize {
		lh.mergeBuckets(lh.Len() - 1)
	}
	if rh.Len() == 0 {
		return lh, nil
	}
	for rh.Len() > bucketSize {
		rh.mergeBuckets(rh.Len() - 1)
	}
	lCount := lh.Buckets[lh.Len()-1].Count
	rCount := rh.Buckets[rh.Len()-1].Count - offset
	lAvg := float64(lCount) / float64(lh.Len())
	rAvg := float64(rCount) / float64(rh.Len())
	for lh.Len() > 1 && lAvg*2 <= rAvg {
		lh.mergeBuckets(lh.Len() - 1)
		lAvg *= 2
	}
	for rh.Len() > 1 && rAvg*2 <= lAvg {
		rh.mergeBuckets(rh.Len() - 1)
		rAvg *= 2
	}
	for i := 0; i < rh.Len(); i++ {
		lh.AppendBucket(rh.GetLower(i), rh.GetUpper(i), rh.Buckets[i].Count+lCount-offset, rh.Buckets[i].Repeat)
	}
	for lh.Len() > bucketSize {
		lh.mergeBuckets(lh.Len() - 1)
	}
	return lh, nil
}

// AvgCountPerValue gets the average row count per value by the data of histogram.
func (hg *Histogram) AvgCountPerValue(totalCount int64) float64 {
	curNDV := float64(hg.NDV) * hg.getIncreaseFactor(totalCount)
	if curNDV == 0 {
		curNDV = 1
	}
	return float64(totalCount) / curNDV
}

func (hg *Histogram) outOfRange(val types.Datum) bool {
	if hg.Len() == 0 {
		return true
	}
	return chunk.Compare(hg.Bounds.GetRow(0), 0, &val) > 0 ||
		chunk.Compare(hg.Bounds.GetRow(hg.Bounds.NumRows()-1), 0, &val) < 0
}

// ErrorRate is the error rate of estimate row count by bucket and cm sketch.
type ErrorRate struct {
	ErrorTotal float64
	QueryTotal int64
}

// MaxErrorRate is the max error rate of estimate row count of a not pseudo column.
// If the table is pseudo, but the average error rate is less than MaxErrorRate,
// then the column is not pseudo.
const MaxErrorRate = 0.25

// NotAccurate is true when the total of query is zero or the average error
// rate is greater than MaxErrorRate.
func (e *ErrorRate) NotAccurate() bool {
	if e.QueryTotal == 0 {
		return true
	}
	return e.ErrorTotal/float64(e.QueryTotal) > MaxErrorRate
}

func (e *ErrorRate) update(rate float64) {
	e.QueryTotal++
	e.ErrorTotal += rate
}

func (e *ErrorRate) merge(rate *ErrorRate) {
	e.QueryTotal += rate.QueryTotal
	e.ErrorTotal += rate.ErrorTotal
}

// Column represents a column histogram.
type Column struct {
	Histogram
	*CMSketch
	Count    int64
	Info     *model.ColumnInfo
	isHandle bool
	ErrorRate
}

func (c *Column) String() string {
	return c.Histogram.ToString(0)
}

func (c *Column) equalRowCount(sc *stmtctx.StatementContext, val types.Datum, modifyCount int64) (float64, error) {
	if val.IsNull() {
		return float64(c.NullCount), nil
	}
	// all the values is null
	if c.Histogram.Bounds.NumRows() == 0 {
		return 0.0, nil
	}
	if c.NDV > 0 && c.outOfRange(val) {
		return float64(modifyCount) / float64(c.NDV), nil
	}
	if c.CMSketch != nil {
		count, err := c.CMSketch.queryValue(sc, val)
		return float64(count), errors.Trace(err)
	}
	return c.Histogram.equalRowCount(val), nil
}

// getColumnRowCount estimates the row count by a slice of Range.
func (c *Column) getColumnRowCount(sc *stmtctx.StatementContext, ranges []*ranger.Range, modifyCount int64) (float64, error) {
	var rowCount float64
	for _, rg := range ranges {
		cmp, err := rg.LowVal[0].CompareDatum(sc, &rg.HighVal[0])
		if err != nil {
			return 0, errors.Trace(err)
		}
		if cmp == 0 {
			// the point case.
			if !rg.LowExclude && !rg.HighExclude {
				var cnt float64
				cnt, err = c.equalRowCount(sc, rg.LowVal[0], modifyCount)
				if err != nil {
					return 0, errors.Trace(err)
				}
				rowCount += cnt
			}
			continue
		}
		// the interval case.
		cnt := c.betweenRowCount(rg.LowVal[0], rg.HighVal[0])
		if c.outOfRange(rg.LowVal[0]) || c.outOfRange(rg.HighVal[0]) {
			cnt += float64(modifyCount) / outOfRangeBetweenRate
		}
		if rg.LowExclude {
			lowCnt, err := c.equalRowCount(sc, rg.LowVal[0], modifyCount)
			if err != nil {
				return 0, errors.Trace(err)
			}
			cnt -= lowCnt
		}
		if !rg.HighExclude {
			highCnt, err := c.equalRowCount(sc, rg.HighVal[0], modifyCount)
			if err != nil {
				return 0, errors.Trace(err)
			}
			cnt += highCnt
		}
		rowCount += cnt
	}
	if rowCount > c.totalRowCount() {
		rowCount = c.totalRowCount()
	} else if rowCount < 0 {
		rowCount = 0
	}
	return rowCount, nil
}

// Index represents an index histogram.
type Index struct {
	Histogram
	*CMSketch
	ErrorRate
	statsVer int64 // statsVer is the version of the current stats, used to maintain compatibility
	Info     *model.IndexInfo
}

func (idx *Index) String() string {
	return idx.Histogram.ToString(len(idx.Info.Columns))
}

func (idx *Index) equalRowCount(sc *stmtctx.StatementContext, b []byte, modifyCount int64) float64 {
	val := types.NewBytesDatum(b)
	if idx.NDV > 0 && idx.outOfRange(val) {
		return float64(modifyCount) / (float64(idx.NDV))
	}
	if idx.CMSketch != nil {
		return float64(idx.CMSketch.QueryBytes(b))
	}
	return idx.Histogram.equalRowCount(val)
}

func (idx *Index) getRowCount(sc *stmtctx.StatementContext, indexRanges []*ranger.Range, modifyCount int64) (float64, error) {
	totalCount := float64(0)
	for _, indexRange := range indexRanges {
		lb, err := codec.EncodeKey(sc, nil, indexRange.LowVal...)
		if err != nil {
			return 0, errors.Trace(err)
		}
		rb, err := codec.EncodeKey(sc, nil, indexRange.HighVal...)
		if err != nil {
			return 0, errors.Trace(err)
		}
		fullLen := len(indexRange.LowVal) == len(indexRange.HighVal) && len(indexRange.LowVal) == len(idx.Info.Columns)
		if fullLen && bytes.Equal(lb, rb) {
			if !indexRange.LowExclude && !indexRange.HighExclude {
				totalCount += idx.equalRowCount(sc, lb, modifyCount)
			}
			continue
		}
		if indexRange.LowExclude {
			lb = kv.Key(lb).PrefixNext()
		}
		if !indexRange.HighExclude {
			rb = kv.Key(rb).PrefixNext()
		}
		l := types.NewBytesDatum(lb)
		r := types.NewBytesDatum(rb)
		totalCount += idx.betweenRowCount(l, r)
		if idx.outOfRange(l) || idx.outOfRange(r) {
			totalCount += float64(modifyCount) / outOfRangeBetweenRate
		}
	}
	if totalCount > idx.totalRowCount() {
		totalCount = idx.totalRowCount()
	}
	return totalCount, nil
}

func (idx *Index) outOfRange(val types.Datum) bool {
	if idx.Histogram.Len() == 0 {
		return true
	}
	withInLowBoundOrPrefixMatch := chunk.Compare(idx.Bounds.GetRow(0), 0, &val) <= 0 ||
		matchPrefix(idx.Bounds.GetRow(0), 0, &val)
	withInHighBound := chunk.Compare(idx.Bounds.GetRow(idx.Bounds.NumRows()-1), 0, &val) >= 0
	return !withInLowBoundOrPrefixMatch || !withInHighBound
}

// matchPrefix checks whether ad is the prefix of value
func matchPrefix(row chunk.Row, colIdx int, ad *types.Datum) bool {
	switch ad.Kind() {
	case types.KindString, types.KindBytes, types.KindBinaryLiteral, types.KindMysqlBit:
		return strings.HasPrefix(row.GetString(colIdx), ad.GetString())
	}
	return false
}
