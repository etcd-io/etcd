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

package chunk

import (
	"encoding/binary"
	"reflect"
	"unsafe"

	"github.com/cznic/mathutil"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
)

// Chunk stores multiple rows of data in Apache Arrow format.
// See https://arrow.apache.org/docs/memory_layout.html
// Values are appended in compact format and can be directly accessed without decoding.
// When the chunk is done processing, we can reuse the allocated memory by resetting it.
type Chunk struct {
	columns []*column
	// numVirtualRows indicates the number of virtual rows, which have zero column.
	// It is used only when this Chunk doesn't hold any data, i.e. "len(columns)==0".
	numVirtualRows int
	// capacity indicates the max number of rows this chunk can hold.
	capacity int
}

// Capacity constants.
const (
	InitialCapacity = 32
	ZeroCapacity    = 0
)

// NewChunkWithCapacity creates a new chunk with field types and capacity.
func NewChunkWithCapacity(fields []*types.FieldType, cap int) *Chunk {
	return New(fields, cap, cap) //FIXME: in following PR.
}

// New creates a new chunk.
//  cap: the limit for the max number of rows.
//  maxChunkSize: the max limit for the number of rows.
func New(fields []*types.FieldType, cap, maxChunkSize int) *Chunk {
	chk := new(Chunk)
	chk.columns = make([]*column, 0, len(fields))
	chk.capacity = mathutil.Min(cap, maxChunkSize)
	for _, f := range fields {
		elemLen := getFixedLen(f)
		if elemLen == varElemLen {
			chk.columns = append(chk.columns, newVarLenColumn(chk.capacity, nil))
		} else {
			chk.columns = append(chk.columns, newFixedLenColumn(elemLen, chk.capacity))
		}
	}
	chk.numVirtualRows = 0
	return chk
}

// Renew creates a new Chunk based on an existing Chunk. The newly created Chunk
// has the same data schema with the old Chunk. The capacity of the new Chunk
// might be doubled based on the capacity of the old Chunk and the maxChunkSize.
//  chk: old chunk(often used in previous call).
//  maxChunkSize: the limit for the max number of rows.
func Renew(chk *Chunk, maxChunkSize int) *Chunk {
	newChk := new(Chunk)
	if chk.columns == nil {
		return newChk
	}
	newCap := reCalcCapacity(chk, maxChunkSize)
	newChk.columns = renewColumns(chk.columns, newCap)
	newChk.numVirtualRows = 0
	newChk.capacity = newCap
	return newChk
}

// renewColumns creates the columns of a Chunk. The capacity of the newly
// created columns is equal to cap.
func renewColumns(oldCol []*column, cap int) []*column {
	columns := make([]*column, 0, len(oldCol))
	for _, col := range oldCol {
		if col.isFixed() {
			columns = append(columns, newFixedLenColumn(len(col.elemBuf), cap))
		} else {
			columns = append(columns, newVarLenColumn(cap, col))
		}
	}
	return columns
}

// MemoryUsage returns the total memory usage of a Chunk in B.
// We ignore the size of column.length and column.nullCount
// since they have little effect of the total memory usage.
func (c *Chunk) MemoryUsage() (sum int64) {
	for _, col := range c.columns {
		curColMemUsage := int64(unsafe.Sizeof(*col)) + int64(cap(col.nullBitmap)) + int64(cap(col.offsets)*4) + int64(cap(col.data)) + int64(cap(col.elemBuf))
		sum += curColMemUsage
	}
	return
}

// newFixedLenColumn creates a fixed length column with elemLen and initial data capacity.
func newFixedLenColumn(elemLen, cap int) *column {
	return &column{
		elemBuf:    make([]byte, elemLen),
		data:       make([]byte, 0, cap*elemLen),
		nullBitmap: make([]byte, 0, cap>>3),
	}
}

// newVarLenColumn creates a variable length column with initial data capacity.
func newVarLenColumn(cap int, old *column) *column {
	estimatedElemLen := 8
	// For varLenColumn (e.g. varchar), the accurate length of an element is unknown.
	// Therefore, in the first executor.Next we use an experience value -- 8 (so it may make runtime.growslice)
	// but in the following Next call we estimate the length as AVG x 1.125 elemLen of the previous call.
	if old != nil && old.length != 0 {
		estimatedElemLen = (len(old.data) + len(old.data)/8) / old.length
	}
	return &column{
		offsets:    make([]int32, 1, cap+1),
		data:       make([]byte, 0, cap*estimatedElemLen),
		nullBitmap: make([]byte, 0, cap>>3),
	}
}

// MakeRef makes column in "dstColIdx" reference to column in "srcColIdx".
func (c *Chunk) MakeRef(srcColIdx, dstColIdx int) {
	c.columns[dstColIdx] = c.columns[srcColIdx]
}

// SwapColumn swaps column "c.columns[colIdx]" with column
// "other.columns[otherIdx]". If there exists columns refer to the column to be
// swapped, we need to re-build the reference.
func (c *Chunk) SwapColumn(colIdx int, other *Chunk, otherIdx int) {
	// Find the leftmost column of the reference which is the actual column to
	// be swapped.
	for i := 0; i < colIdx; i++ {
		if c.columns[i] == c.columns[colIdx] {
			colIdx = i
		}
	}
	for i := 0; i < otherIdx; i++ {
		if other.columns[i] == other.columns[otherIdx] {
			otherIdx = i
		}
	}

	// Find the columns which refer to the actual column to be swapped.
	refColsIdx := make([]int, 0, len(c.columns)-colIdx)
	for i := colIdx; i < len(c.columns); i++ {
		if c.columns[i] == c.columns[colIdx] {
			refColsIdx = append(refColsIdx, i)
		}
	}
	refColsIdx4Other := make([]int, 0, len(other.columns)-otherIdx)
	for i := otherIdx; i < len(other.columns); i++ {
		if other.columns[i] == other.columns[otherIdx] {
			refColsIdx4Other = append(refColsIdx4Other, i)
		}
	}

	// Swap columns from two chunks.
	c.columns[colIdx], other.columns[otherIdx] = other.columns[otherIdx], c.columns[colIdx]

	// Rebuild the reference.
	for _, i := range refColsIdx {
		c.MakeRef(colIdx, i)
	}
	for _, i := range refColsIdx4Other {
		other.MakeRef(otherIdx, i)
	}
}

// SwapColumns swaps columns with another Chunk.
func (c *Chunk) SwapColumns(other *Chunk) {
	c.columns, other.columns = other.columns, c.columns
	c.numVirtualRows, other.numVirtualRows = other.numVirtualRows, c.numVirtualRows
}

// SetNumVirtualRows sets the virtual row number for a Chunk.
// It should only be used when there exists no column in the Chunk.
func (c *Chunk) SetNumVirtualRows(numVirtualRows int) {
	c.numVirtualRows = numVirtualRows
}

// Reset resets the chunk, so the memory it allocated can be reused.
// Make sure all the data in the chunk is not used anymore before you reuse this chunk.
func (c *Chunk) Reset() {
	if c.columns == nil {
		return
	}
	for _, col := range c.columns {
		col.reset()
	}
	c.numVirtualRows = 0
}

// GrowAndReset resets the Chunk and doubles the capacity of the Chunk.
// The doubled capacity should not be larger than maxChunkSize.
// TODO: this method will be used in following PR.
func (c *Chunk) GrowAndReset(maxChunkSize int) {
	if c.columns == nil {
		return
	}
	newCap := reCalcCapacity(c, maxChunkSize)
	if newCap <= c.capacity {
		c.Reset()
		return
	}
	c.capacity = newCap
	c.columns = renewColumns(c.columns, newCap)
	c.numVirtualRows = 0
}

// reCalcCapacity calculates the capacity for another Chunk based on the current
// Chunk. The new capacity is doubled only when the current Chunk is full.
func reCalcCapacity(c *Chunk, maxChunkSize int) int {
	if c.NumRows() < c.capacity {
		return c.capacity
	}
	return mathutil.Min(c.capacity*2, maxChunkSize)
}

// Capacity returns the capacity of the Chunk.
func (c *Chunk) Capacity() int {
	return c.capacity
}

// NumCols returns the number of columns in the chunk.
func (c *Chunk) NumCols() int {
	return len(c.columns)
}

// NumRows returns the number of rows in the chunk.
func (c *Chunk) NumRows() int {
	if c.NumCols() == 0 {
		return c.numVirtualRows
	}
	return c.columns[0].length
}

// GetRow gets the Row in the chunk with the row index.
func (c *Chunk) GetRow(idx int) Row {
	return Row{c: c, idx: idx}
}

// AppendRow appends a row to the chunk.
func (c *Chunk) AppendRow(row Row) {
	c.AppendPartialRow(0, row)
	c.numVirtualRows++
}

// AppendPartialRow appends a row to the chunk.
func (c *Chunk) AppendPartialRow(colIdx int, row Row) {
	for i, rowCol := range row.c.columns {
		chkCol := c.columns[colIdx+i]
		chkCol.appendNullBitmap(!rowCol.isNull(row.idx))
		if rowCol.isFixed() {
			elemLen := len(rowCol.elemBuf)
			offset := row.idx * elemLen
			chkCol.data = append(chkCol.data, rowCol.data[offset:offset+elemLen]...)
		} else {
			start, end := rowCol.offsets[row.idx], rowCol.offsets[row.idx+1]
			chkCol.data = append(chkCol.data, rowCol.data[start:end]...)
			chkCol.offsets = append(chkCol.offsets, int32(len(chkCol.data)))
		}
		chkCol.length++
	}
}

// PreAlloc pre-allocates the memory space in a Chunk to store the Row.
// NOTE:
// 1. The Chunk must be empty or holds no useful data.
// 2. The schema of the Row must be the same with the Chunk.
// 3. This API is paired with the `Insert()` function, which inserts all the
//    rows data into the Chunk after the pre-allocation.
// 4. We set the null bitmap here instead of in the Insert() function because
//    when the Insert() function is called parallelly, the data race on a byte
//    can not be avoided although the manipulated bits are different inside a
//    byte.
func (c *Chunk) PreAlloc(row Row) {
	for i, srcCol := range row.c.columns {
		dstCol := c.columns[i]
		dstCol.appendNullBitmap(!srcCol.isNull(row.idx))
		elemLen := len(srcCol.elemBuf)
		if !srcCol.isFixed() {
			elemLen = int(srcCol.offsets[row.idx+1] - srcCol.offsets[row.idx])
			dstCol.offsets = append(dstCol.offsets, int32(len(dstCol.data)+elemLen))
		}
		dstCol.length++
		needCap := len(dstCol.data) + elemLen
		if needCap <= cap(dstCol.data) {
			(*reflect.SliceHeader)(unsafe.Pointer(&dstCol.data)).Len = len(dstCol.data) + elemLen
			continue
		}
		// Grow the capacity according to golang.growslice.
		newCap := cap(dstCol.data)
		doubleCap := newCap << 1
		if needCap > doubleCap {
			newCap = needCap
		} else {
			if len(dstCol.data) < 1024 {
				newCap = doubleCap
			} else {
				for 0 < newCap && newCap < needCap {
					newCap += newCap / 4
				}
				if newCap <= 0 {
					newCap = needCap
				}
			}
		}
		dstCol.data = make([]byte, len(dstCol.data)+elemLen, newCap)
	}
}

// Insert inserts `row` on the position specified by `rowIdx`.
// Note: Insert will cover the origin data, it should be called after
// PreAlloc.
func (c *Chunk) Insert(rowIdx int, row Row) {
	for i, srcCol := range row.c.columns {
		if row.IsNull(i) {
			continue
		}
		dstCol := c.columns[i]
		var srcStart, srcEnd, destStart, destEnd int
		if srcCol.isFixed() {
			srcElemLen, destElemLen := len(srcCol.elemBuf), len(dstCol.elemBuf)
			srcStart, destStart = row.idx*srcElemLen, rowIdx*destElemLen
			srcEnd, destEnd = srcStart+srcElemLen, destStart+destElemLen
		} else {
			srcStart, srcEnd = int(srcCol.offsets[row.idx]), int(srcCol.offsets[row.idx+1])
			destStart, destEnd = int(dstCol.offsets[rowIdx]), int(dstCol.offsets[rowIdx+1])
		}
		copy(dstCol.data[destStart:destEnd], srcCol.data[srcStart:srcEnd])
	}
}

// Append appends rows in [begin, end) in another Chunk to a Chunk.
func (c *Chunk) Append(other *Chunk, begin, end int) {
	for colID, src := range other.columns {
		dst := c.columns[colID]
		if src.isFixed() {
			elemLen := len(src.elemBuf)
			dst.data = append(dst.data, src.data[begin*elemLen:end*elemLen]...)
		} else {
			beginOffset, endOffset := src.offsets[begin], src.offsets[end]
			dst.data = append(dst.data, src.data[beginOffset:endOffset]...)
			for i := begin; i < end; i++ {
				dst.offsets = append(dst.offsets, dst.offsets[len(dst.offsets)-1]+src.offsets[i+1]-src.offsets[i])
			}
		}
		for i := begin; i < end; i++ {
			dst.appendNullBitmap(!src.isNull(i))
			dst.length++
		}
	}
	c.numVirtualRows += end - begin
}

// TruncateTo truncates rows from tail to head in a Chunk to "numRows" rows.
func (c *Chunk) TruncateTo(numRows int) {
	for _, col := range c.columns {
		if col.isFixed() {
			elemLen := len(col.elemBuf)
			col.data = col.data[:numRows*elemLen]
		} else {
			col.data = col.data[:col.offsets[numRows]]
			col.offsets = col.offsets[:numRows+1]
		}
		for i := numRows; i < col.length; i++ {
			if col.isNull(i) {
				col.nullCount--
			}
		}
		col.length = numRows
		bitmapLen := (col.length + 7) / 8
		col.nullBitmap = col.nullBitmap[:bitmapLen]
		if col.length%8 != 0 {
			// When we append null, we simply increment the nullCount,
			// so we need to clear the unused bits in the last bitmap byte.
			lastByte := col.nullBitmap[bitmapLen-1]
			unusedBitsLen := 8 - uint(col.length%8)
			lastByte <<= unusedBitsLen
			lastByte >>= unusedBitsLen
			col.nullBitmap[bitmapLen-1] = lastByte
		}
	}
	c.numVirtualRows = numRows
}

// AppendNull appends a null value to the chunk.
func (c *Chunk) AppendNull(colIdx int) {
	c.columns[colIdx].appendNull()
}

// AppendInt64 appends a int64 value to the chunk.
func (c *Chunk) AppendInt64(colIdx int, i int64) {
	c.columns[colIdx].appendInt64(i)
}

// AppendUint64 appends a uint64 value to the chunk.
func (c *Chunk) AppendUint64(colIdx int, u uint64) {
	c.columns[colIdx].appendUint64(u)
}

// AppendFloat32 appends a float32 value to the chunk.
func (c *Chunk) AppendFloat32(colIdx int, f float32) {
	c.columns[colIdx].appendFloat32(f)
}

// AppendFloat64 appends a float64 value to the chunk.
func (c *Chunk) AppendFloat64(colIdx int, f float64) {
	c.columns[colIdx].appendFloat64(f)
}

// AppendString appends a string value to the chunk.
func (c *Chunk) AppendString(colIdx int, str string) {
	c.columns[colIdx].appendString(str)
}

// AppendBytes appends a bytes value to the chunk.
func (c *Chunk) AppendBytes(colIdx int, b []byte) {
	c.columns[colIdx].appendBytes(b)
}

// AppendTime appends a Time value to the chunk.
// TODO: change the time structure so it can be directly written to memory.
func (c *Chunk) AppendTime(colIdx int, t types.Time) {
	c.columns[colIdx].appendTime(t)
}

// AppendDuration appends a Duration value to the chunk.
func (c *Chunk) AppendDuration(colIdx int, dur types.Duration) {
	c.columns[colIdx].appendDuration(dur)
}

// AppendMyDecimal appends a MyDecimal value to the chunk.
func (c *Chunk) AppendMyDecimal(colIdx int, dec *types.MyDecimal) {
	c.columns[colIdx].appendMyDecimal(dec)
}

// AppendEnum appends an Enum value to the chunk.
func (c *Chunk) AppendEnum(colIdx int, enum types.Enum) {
	c.columns[colIdx].appendNameValue(enum.Name, enum.Value)
}

// AppendSet appends a Set value to the chunk.
func (c *Chunk) AppendSet(colIdx int, set types.Set) {
	c.columns[colIdx].appendNameValue(set.Name, set.Value)
}

// AppendJSON appends a JSON value to the chunk.
func (c *Chunk) AppendJSON(colIdx int, j json.BinaryJSON) {
	c.columns[colIdx].appendJSON(j)
}

// AppendDatum appends a datum into the chunk.
func (c *Chunk) AppendDatum(colIdx int, d *types.Datum) {
	switch d.Kind() {
	case types.KindNull:
		c.AppendNull(colIdx)
	case types.KindInt64:
		c.AppendInt64(colIdx, d.GetInt64())
	case types.KindUint64:
		c.AppendUint64(colIdx, d.GetUint64())
	case types.KindFloat32:
		c.AppendFloat32(colIdx, d.GetFloat32())
	case types.KindFloat64:
		c.AppendFloat64(colIdx, d.GetFloat64())
	case types.KindString, types.KindBytes, types.KindBinaryLiteral, types.KindRaw, types.KindMysqlBit:
		c.AppendBytes(colIdx, d.GetBytes())
	case types.KindMysqlDecimal:
		c.AppendMyDecimal(colIdx, d.GetMysqlDecimal())
	case types.KindMysqlDuration:
		c.AppendDuration(colIdx, d.GetMysqlDuration())
	case types.KindMysqlEnum:
		c.AppendEnum(colIdx, d.GetMysqlEnum())
	case types.KindMysqlSet:
		c.AppendSet(colIdx, d.GetMysqlSet())
	case types.KindMysqlTime:
		c.AppendTime(colIdx, d.GetMysqlTime())
	case types.KindMysqlJSON:
		c.AppendJSON(colIdx, d.GetMysqlJSON())
	}
}

func writeTime(buf []byte, t types.Time) {
	binary.BigEndian.PutUint16(buf, uint16(t.Time.Year()))
	buf[2] = uint8(t.Time.Month())
	buf[3] = uint8(t.Time.Day())
	buf[4] = uint8(t.Time.Hour())
	buf[5] = uint8(t.Time.Minute())
	buf[6] = uint8(t.Time.Second())
	binary.BigEndian.PutUint32(buf[8:], uint32(t.Time.Microsecond()))
	buf[12] = t.Type
	buf[13] = uint8(t.Fsp)
}

func readTime(buf []byte) types.Time {
	year := int(binary.BigEndian.Uint16(buf))
	month := int(buf[2])
	day := int(buf[3])
	hour := int(buf[4])
	minute := int(buf[5])
	second := int(buf[6])
	microseconds := int(binary.BigEndian.Uint32(buf[8:]))
	tp := buf[12]
	fsp := int(buf[13])
	return types.Time{
		Time: types.FromDate(year, month, day, hour, minute, second, microseconds),
		Type: tp,
		Fsp:  fsp,
	}
}
