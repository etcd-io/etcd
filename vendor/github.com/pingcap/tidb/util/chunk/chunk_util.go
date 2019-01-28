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

// CopySelectedJoinRows copies the selected joined rows from the source Chunk
// to the destination Chunk.
// Return true if at least one joined row was selected.
//
// NOTE: All the outer rows in the source Chunk should be the same.
func CopySelectedJoinRows(src *Chunk, innerColOffset, outerColOffset int, selected []bool, dst *Chunk) bool {
	if src.NumRows() == 0 {
		return false
	}

	numSelected := copySelectedInnerRows(innerColOffset, outerColOffset, src, selected, dst)
	copyOuterRows(innerColOffset, outerColOffset, src, numSelected, dst)
	dst.numVirtualRows += numSelected
	return numSelected > 0
}

// copySelectedInnerRows copies the selected inner rows from the source Chunk
// to the destination Chunk.
// return the number of rows which is selected.
func copySelectedInnerRows(innerColOffset, outerColOffset int, src *Chunk, selected []bool, dst *Chunk) int {
	oldLen := dst.columns[innerColOffset].length
	var srcCols []*column
	if innerColOffset == 0 {
		srcCols = src.columns[:outerColOffset]
	} else {
		srcCols = src.columns[innerColOffset:]
	}
	for j, srcCol := range srcCols {
		dstCol := dst.columns[innerColOffset+j]
		if srcCol.isFixed() {
			for i := 0; i < len(selected); i++ {
				if !selected[i] {
					continue
				}
				dstCol.appendNullBitmap(!srcCol.isNull(i))
				dstCol.length++

				elemLen := len(srcCol.elemBuf)
				offset := i * elemLen
				dstCol.data = append(dstCol.data, srcCol.data[offset:offset+elemLen]...)
			}
		} else {
			for i := 0; i < len(selected); i++ {
				if !selected[i] {
					continue
				}
				dstCol.appendNullBitmap(!srcCol.isNull(i))
				dstCol.length++

				start, end := srcCol.offsets[i], srcCol.offsets[i+1]
				dstCol.data = append(dstCol.data, srcCol.data[start:end]...)
				dstCol.offsets = append(dstCol.offsets, int32(len(dstCol.data)))
			}
		}
	}
	return dst.columns[innerColOffset].length - oldLen
}

// copyOuterRows copies the continuous 'numRows' outer rows in the source Chunk
// to the destination Chunk.
func copyOuterRows(innerColOffset, outerColOffset int, src *Chunk, numRows int, dst *Chunk) {
	if numRows <= 0 {
		return
	}
	row := src.GetRow(0)
	var srcCols []*column
	if innerColOffset == 0 {
		srcCols = src.columns[outerColOffset:]
	} else {
		srcCols = src.columns[:innerColOffset]
	}
	for i, srcCol := range srcCols {
		dstCol := dst.columns[outerColOffset+i]
		dstCol.appendMultiSameNullBitmap(!srcCol.isNull(row.idx), numRows)
		dstCol.length += numRows
		if srcCol.isFixed() {
			elemLen := len(srcCol.elemBuf)
			start := row.idx * elemLen
			end := start + numRows*elemLen
			dstCol.data = append(dstCol.data, srcCol.data[start:end]...)
		} else {
			start, end := srcCol.offsets[row.idx], srcCol.offsets[row.idx+numRows]
			dstCol.data = append(dstCol.data, srcCol.data[start:end]...)
			offsets := dstCol.offsets
			elemLen := srcCol.offsets[row.idx+1] - srcCol.offsets[row.idx]
			for j := 0; j < numRows; j++ {
				offsets = append(offsets, int32(offsets[len(offsets)-1]+elemLen))
			}
			dstCol.offsets = offsets
		}
	}
}
