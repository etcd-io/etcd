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
	"encoding/binary"
	"reflect"
	"unsafe"

	"github.com/cznic/mathutil"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/types"
)

// Codec is used to:
// 1. encode a Chunk to a byte slice.
// 2. decode a Chunk from a byte slice.
type Codec struct {
	// colTypes is used to check whether a column is fixed sized and what the
	// fixed size for every element.
	// NOTE: It's only used for decoding.
	colTypes []*types.FieldType
}

// NewCodec creates a new Codec object for encode or decode a Chunk.
func NewCodec(colTypes []*types.FieldType) *Codec {
	return &Codec{colTypes}
}

// Encode encodes a Chunk to a byte slice.
func (c *Codec) Encode(chk *Chunk) []byte {
	buffer := make([]byte, 0, chk.MemoryUsage())
	for _, col := range chk.columns {
		buffer = c.encodeColumn(buffer, col)
	}
	return buffer
}

func (c *Codec) encodeColumn(buffer []byte, col *column) []byte {
	var lenBuffer [4]byte
	// encode length.
	binary.LittleEndian.PutUint32(lenBuffer[:], uint32(col.length))
	buffer = append(buffer, lenBuffer[:4]...)

	// encode nullCount.
	binary.LittleEndian.PutUint32(lenBuffer[:], uint32(col.nullCount))
	buffer = append(buffer, lenBuffer[:4]...)

	// encode nullBitmap.
	if col.nullCount > 0 {
		numNullBitmapBytes := (col.length + 7) / 8
		buffer = append(buffer, col.nullBitmap[:numNullBitmapBytes]...)
	}

	// encode offsets.
	if !col.isFixed() {
		numOffsetBytes := (col.length + 1) * 4
		offsetBytes := c.i32SliceToBytes(col.offsets)
		buffer = append(buffer, offsetBytes[:numOffsetBytes]...)
	}

	// encode data.
	buffer = append(buffer, col.data...)
	return buffer
}

func (c *Codec) i32SliceToBytes(i32s []int32) (b []byte) {
	if len(i32s) == 0 {
		return nil
	}
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Len = len(i32s) * 4
	hdr.Cap = hdr.Len
	hdr.Data = uintptr(unsafe.Pointer(&i32s[0]))
	return b
}

// Decode decodes a Chunk from a byte slice, return the remained unused bytes.
func (c *Codec) Decode(buffer []byte) (*Chunk, []byte) {
	chk := &Chunk{}
	for ordinal := 0; len(buffer) > 0; ordinal++ {
		col := &column{}
		buffer = c.decodeColumn(buffer, col, ordinal)
		chk.columns = append(chk.columns, col)
	}
	return chk, buffer
}

// DecodeToChunk decodes a Chunk from a byte slice, return the remained unused bytes.
func (c *Codec) DecodeToChunk(buffer []byte, chk *Chunk) (remained []byte) {
	for i := 0; i < len(chk.columns); i++ {
		buffer = c.decodeColumn(buffer, chk.columns[i], i)
	}
	return buffer
}

func (c *Codec) decodeColumn(buffer []byte, col *column, ordinal int) (remained []byte) {
	// decode length.
	col.length = int(binary.LittleEndian.Uint32(buffer))
	buffer = buffer[4:]

	// decode nullCount.
	col.nullCount = int(binary.LittleEndian.Uint32(buffer))
	buffer = buffer[4:]

	// decode nullBitmap.
	if col.nullCount > 0 {
		numNullBitmapBytes := (col.length + 7) / 8
		col.nullBitmap = append(col.nullBitmap[:0], buffer[:numNullBitmapBytes]...)
		buffer = buffer[numNullBitmapBytes:]
	} else {
		c.setAllNotNull(col)
	}

	// decode offsets.
	numFixedBytes := getFixedLen(c.colTypes[ordinal])
	numDataBytes := numFixedBytes * col.length
	if numFixedBytes == -1 {
		numOffsetBytes := (col.length + 1) * 4
		col.offsets = append(col.offsets[:0], c.bytesToI32Slice(buffer[:numOffsetBytes])...)
		buffer = buffer[numOffsetBytes:]
		numDataBytes = int(col.offsets[col.length])
	} else if cap(col.elemBuf) < numFixedBytes {
		col.elemBuf = make([]byte, numFixedBytes)
	}

	// decode data.
	col.data = append(col.data[:0], buffer[:numDataBytes]...)
	return buffer[numDataBytes:]
}

var allNotNullBitmap [128]byte

func (c *Codec) setAllNotNull(col *column) {
	numNullBitmapBytes := (col.length + 7) / 8
	col.nullBitmap = col.nullBitmap[:0]
	for i := 0; i < numNullBitmapBytes; {
		numAppendBytes := mathutil.Min(numNullBitmapBytes-i, cap(allNotNullBitmap))
		col.nullBitmap = append(col.nullBitmap, allNotNullBitmap[:numAppendBytes]...)
		i += numAppendBytes
	}
}

func (c *Codec) bytesToI32Slice(b []byte) (i32s []int32) {
	if len(b) == 0 {
		return nil
	}
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&i32s))
	hdr.Len = len(b) / 4
	hdr.Cap = hdr.Len
	hdr.Data = uintptr(unsafe.Pointer(&b[0]))
	return i32s
}

// varElemLen indicates this column is a variable length column.
const varElemLen = -1

func getFixedLen(colType *types.FieldType) int {
	switch colType.Tp {
	case mysql.TypeFloat:
		return 4
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong,
		mysql.TypeLonglong, mysql.TypeDouble, mysql.TypeYear, mysql.TypeDuration:
		return 8
	case mysql.TypeDate, mysql.TypeDatetime, mysql.TypeTimestamp:
		return 16
	case mysql.TypeNewDecimal:
		return types.MyDecimalStructSize
	default:
		return varElemLen
	}
}

func init() {
	for i := 0; i < 128; i++ {
		allNotNullBitmap[i] = 0xFF
	}
}
