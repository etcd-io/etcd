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
	"math"
	"unsafe"

	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/hack"
)

// MutRow represents a mutable Row.
// The underlying columns only contains one row and not exposed to the user.
type MutRow Row

// ToRow converts the MutRow to Row, so it can be used to read data.
func (mr MutRow) ToRow() Row {
	return Row(mr)
}

// Len returns the number of columns.
func (mr MutRow) Len() int {
	return len(mr.c.columns)
}

// MutRowFromValues creates a MutRow from a interface slice.
func MutRowFromValues(vals ...interface{}) MutRow {
	c := &Chunk{columns: make([]*column, 0, len(vals))}
	for _, val := range vals {
		col := makeMutRowColumn(val)
		c.columns = append(c.columns, col)
	}
	return MutRow{c: c}
}

// MutRowFromDatums creates a MutRow from a datum slice.
func MutRowFromDatums(datums []types.Datum) MutRow {
	c := &Chunk{columns: make([]*column, 0, len(datums))}
	for _, d := range datums {
		col := makeMutRowColumn(d.GetValue())
		c.columns = append(c.columns, col)
	}
	return MutRow{c: c, idx: 0}
}

// MutRowFromTypes creates a MutRow from a FieldType slice, each column is initialized to zero value.
func MutRowFromTypes(types []*types.FieldType) MutRow {
	c := &Chunk{columns: make([]*column, 0, len(types))}
	for _, tp := range types {
		col := makeMutRowColumn(zeroValForType(tp))
		c.columns = append(c.columns, col)
	}
	return MutRow{c: c, idx: 0}
}

func zeroValForType(tp *types.FieldType) interface{} {
	switch tp.Tp {
	case mysql.TypeFloat:
		return float32(0)
	case mysql.TypeDouble:
		return float64(0)
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong, mysql.TypeYear:
		if mysql.HasUnsignedFlag(tp.Flag) {
			return uint64(0)
		}
		return int64(0)
	case mysql.TypeString, mysql.TypeVarString, mysql.TypeVarchar:
		return ""
	case mysql.TypeBlob, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeLongBlob:
		return []byte{}
	case mysql.TypeDuration:
		return types.ZeroDuration
	case mysql.TypeNewDecimal:
		return types.NewDecFromInt(0)
	case mysql.TypeDate:
		return types.ZeroDate
	case mysql.TypeDatetime:
		return types.ZeroDatetime
	case mysql.TypeTimestamp:
		return types.ZeroTimestamp
	case mysql.TypeBit:
		return types.BinaryLiteral{}
	case mysql.TypeSet:
		return types.Set{}
	case mysql.TypeEnum:
		return types.Enum{}
	case mysql.TypeJSON:
		return json.CreateBinary(nil)
	default:
		return nil
	}
}

func makeMutRowColumn(in interface{}) *column {
	switch x := in.(type) {
	case nil:
		col := makeMutRowUint64Column(uint64(0))
		col.nullBitmap[0] = 0
		return col
	case int:
		return makeMutRowUint64Column(uint64(x))
	case int64:
		return makeMutRowUint64Column(uint64(x))
	case uint64:
		return makeMutRowUint64Column(x)
	case float64:
		return makeMutRowUint64Column(math.Float64bits(x))
	case float32:
		col := newMutRowFixedLenColumn(4)
		*(*uint32)(unsafe.Pointer(&col.data[0])) = math.Float32bits(x)
		return col
	case string:
		return makeMutRowBytesColumn(hack.Slice(x))
	case []byte:
		return makeMutRowBytesColumn(x)
	case types.BinaryLiteral:
		return makeMutRowBytesColumn(x)
	case *types.MyDecimal:
		col := newMutRowFixedLenColumn(types.MyDecimalStructSize)
		*(*types.MyDecimal)(unsafe.Pointer(&col.data[0])) = *x
		return col
	case types.Time:
		col := newMutRowFixedLenColumn(16)
		writeTime(col.data, x)
		return col
	case json.BinaryJSON:
		col := newMutRowVarLenColumn(len(x.Value) + 1)
		col.data[0] = x.TypeCode
		copy(col.data[1:], x.Value)
		return col
	case types.Duration:
		col := newMutRowFixedLenColumn(16)
		*(*types.Duration)(unsafe.Pointer(&col.data[0])) = x
		return col
	case types.Enum:
		col := newMutRowVarLenColumn(len(x.Name) + 8)
		*(*uint64)(unsafe.Pointer(&col.data[0])) = x.Value
		copy(col.data[8:], x.Name)
		return col
	case types.Set:
		col := newMutRowVarLenColumn(len(x.Name) + 8)
		*(*uint64)(unsafe.Pointer(&col.data[0])) = x.Value
		copy(col.data[8:], x.Name)
		return col
	default:
		return nil
	}
}

func newMutRowFixedLenColumn(elemSize int) *column {
	buf := make([]byte, elemSize+1)
	col := &column{
		length:     1,
		elemBuf:    buf[:elemSize],
		data:       buf[:elemSize],
		nullBitmap: buf[elemSize:],
	}
	col.nullBitmap[0] = 1
	return col
}

func newMutRowVarLenColumn(valSize int) *column {
	buf := make([]byte, valSize+1)
	col := &column{
		length:     1,
		offsets:    []int32{0, int32(valSize)},
		data:       buf[:valSize],
		nullBitmap: buf[valSize:],
	}
	col.nullBitmap[0] = 1
	return col
}

func makeMutRowUint64Column(val uint64) *column {
	col := newMutRowFixedLenColumn(8)
	*(*uint64)(unsafe.Pointer(&col.data[0])) = val
	return col
}

func makeMutRowBytesColumn(bin []byte) *column {
	col := newMutRowVarLenColumn(len(bin))
	copy(col.data, bin)
	col.nullBitmap[0] = 1
	return col
}

// SetRow sets the MutRow with Row.
func (mr MutRow) SetRow(row Row) {
	for colIdx, rCol := range row.c.columns {
		mrCol := mr.c.columns[colIdx]
		if rCol.isNull(row.idx) {
			mrCol.nullBitmap[0] = 0
			continue
		}
		elemLen := len(rCol.elemBuf)
		if elemLen > 0 {
			copy(mrCol.data, rCol.data[row.idx*elemLen:(row.idx+1)*elemLen])
		} else {
			setMutRowBytes(mrCol, rCol.data[rCol.offsets[row.idx]:rCol.offsets[row.idx+1]])
		}
		mrCol.nullBitmap[0] = 1
	}
}

// SetValues sets the MutRow with values.
func (mr MutRow) SetValues(vals ...interface{}) {
	for i, v := range vals {
		mr.SetValue(i, v)
	}
}

// SetValue sets the MutRow with colIdx and value.
func (mr MutRow) SetValue(colIdx int, val interface{}) {
	col := mr.c.columns[colIdx]
	if val == nil {
		col.nullBitmap[0] = 0
		return
	}
	switch x := val.(type) {
	case int:
		binary.LittleEndian.PutUint64(col.data, uint64(x))
	case int64:
		binary.LittleEndian.PutUint64(col.data, uint64(x))
	case uint64:
		binary.LittleEndian.PutUint64(col.data, x)
	case float64:
		binary.LittleEndian.PutUint64(col.data, math.Float64bits(x))
	case float32:
		binary.LittleEndian.PutUint32(col.data, math.Float32bits(x))
	case string:
		setMutRowBytes(col, hack.Slice(x))
	case []byte:
		setMutRowBytes(col, x)
	case types.BinaryLiteral:
		setMutRowBytes(col, x)
	case types.Duration:
		*(*types.Duration)(unsafe.Pointer(&col.data[0])) = x
	case *types.MyDecimal:
		*(*types.MyDecimal)(unsafe.Pointer(&col.data[0])) = *x
	case types.Time:
		writeTime(col.data, x)
	case types.Enum:
		setMutRowNameValue(col, x.Name, x.Value)
	case types.Set:
		setMutRowNameValue(col, x.Name, x.Value)
	case json.BinaryJSON:
		setMutRowJSON(col, x)
	}
	col.nullBitmap[0] = 1
}

// SetDatums sets the MutRow with datum slice.
func (mr MutRow) SetDatums(datums ...types.Datum) {
	for i, d := range datums {
		mr.SetDatum(i, d)
	}
}

// SetDatum sets the MutRow with colIdx and datum.
func (mr MutRow) SetDatum(colIdx int, d types.Datum) {
	col := mr.c.columns[colIdx]
	if d.IsNull() {
		col.nullBitmap[0] = 0
		return
	}
	switch d.Kind() {
	case types.KindInt64, types.KindUint64, types.KindFloat64:
		binary.LittleEndian.PutUint64(mr.c.columns[colIdx].data, d.GetUint64())
	case types.KindFloat32:
		binary.LittleEndian.PutUint32(mr.c.columns[colIdx].data, math.Float32bits(d.GetFloat32()))
	case types.KindString, types.KindBytes, types.KindBinaryLiteral:
		setMutRowBytes(col, d.GetBytes())
	case types.KindMysqlTime:
		writeTime(col.data, d.GetMysqlTime())
	case types.KindMysqlDuration:
		*(*types.Duration)(unsafe.Pointer(&col.data[0])) = d.GetMysqlDuration()
	case types.KindMysqlDecimal:
		*(*types.MyDecimal)(unsafe.Pointer(&col.data[0])) = *d.GetMysqlDecimal()
	case types.KindMysqlJSON:
		setMutRowJSON(col, d.GetMysqlJSON())
	case types.KindMysqlEnum:
		e := d.GetMysqlEnum()
		setMutRowNameValue(col, e.Name, e.Value)
	case types.KindMysqlSet:
		s := d.GetMysqlSet()
		setMutRowNameValue(col, s.Name, s.Value)
	default:
		mr.c.columns[colIdx] = makeMutRowColumn(d.GetValue())
	}
	col.nullBitmap[0] = 1
}

func setMutRowBytes(col *column, bin []byte) {
	if len(col.data) >= len(bin) {
		col.data = col.data[:len(bin)]
	} else {
		buf := make([]byte, len(bin)+1)
		col.data = buf[:len(bin)]
		col.nullBitmap = buf[len(bin):]
	}
	copy(col.data, bin)
	col.offsets[1] = int32(len(bin))
}

func setMutRowNameValue(col *column, name string, val uint64) {
	dataLen := len(name) + 8
	if len(col.data) >= dataLen {
		col.data = col.data[:dataLen]
	} else {
		buf := make([]byte, dataLen+1)
		col.data = buf[:dataLen]
		col.nullBitmap = buf[dataLen:]
	}
	binary.LittleEndian.PutUint64(col.data, val)
	copy(col.data[8:], name)
	col.offsets[1] = int32(dataLen)
}

func setMutRowJSON(col *column, j json.BinaryJSON) {
	dataLen := len(j.Value) + 1
	if len(col.data) >= dataLen {
		col.data = col.data[:dataLen]
	} else {
		// In MutRow, there always exists 1 data in every column,
		// we should allocate one more byte for null bitmap.
		buf := make([]byte, dataLen+1)
		col.data = buf[:dataLen]
		col.nullBitmap = buf[dataLen:]
	}
	col.data[0] = j.TypeCode
	copy(col.data[1:], j.Value)
	col.offsets[1] = int32(dataLen)
}

// ShallowCopyPartialRow shallow copies the data of `row` to MutRow.
func (mr MutRow) ShallowCopyPartialRow(colIdx int, row Row) {
	for i, srcCol := range row.c.columns {
		dstCol := mr.c.columns[colIdx+i]
		if !srcCol.isNull(row.idx) {
			// MutRow only contains one row, so we can directly set the whole byte.
			dstCol.nullBitmap[0] = 1
		} else {
			dstCol.nullBitmap[0] = 0
		}

		if srcCol.isFixed() {
			elemLen := len(srcCol.elemBuf)
			offset := row.idx * elemLen
			dstCol.data = srcCol.data[offset : offset+elemLen]
		} else {
			start, end := srcCol.offsets[row.idx], srcCol.offsets[row.idx+1]
			dstCol.data = srcCol.data[start:end]
			dstCol.offsets[1] = int32(len(dstCol.data))
		}
	}
}
