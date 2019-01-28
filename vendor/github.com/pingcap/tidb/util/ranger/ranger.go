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

package ranger

import (
	"bytes"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
)

func validInterval(sc *stmtctx.StatementContext, low, high point) (bool, error) {
	l, err := codec.EncodeKey(sc, nil, low.value)
	if err != nil {
		return false, errors.Trace(err)
	}
	if low.excl {
		l = []byte(kv.Key(l).PrefixNext())
	}
	r, err := codec.EncodeKey(sc, nil, high.value)
	if err != nil {
		return false, errors.Trace(err)
	}
	if !high.excl {
		r = []byte(kv.Key(r).PrefixNext())
	}
	return bytes.Compare(l, r) < 0, nil
}

// points2Ranges build index ranges from range points.
// Only one column is built there. If there're multiple columns, use appendPoints2Ranges.
func points2Ranges(sc *stmtctx.StatementContext, rangePoints []point, tp *types.FieldType) ([]*Range, error) {
	ranges := make([]*Range, 0, len(rangePoints)/2)
	for i := 0; i < len(rangePoints); i += 2 {
		startPoint, err := convertPoint(sc, rangePoints[i], tp)
		if err != nil {
			return nil, errors.Trace(err)
		}
		endPoint, err := convertPoint(sc, rangePoints[i+1], tp)
		if err != nil {
			return nil, errors.Trace(err)
		}
		less, err := validInterval(sc, startPoint, endPoint)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !less {
			continue
		}
		// If column has not null flag, [null, null] should be removed.
		if mysql.HasNotNullFlag(tp.Flag) && endPoint.value.Kind() == types.KindNull {
			continue
		}

		ran := &Range{
			LowVal:      []types.Datum{startPoint.value},
			LowExclude:  startPoint.excl,
			HighVal:     []types.Datum{endPoint.value},
			HighExclude: endPoint.excl,
		}
		ranges = append(ranges, ran)
	}
	return ranges, nil
}

func convertPoint(sc *stmtctx.StatementContext, point point, tp *types.FieldType) (point, error) {
	switch point.value.Kind() {
	case types.KindMaxValue, types.KindMinNotNull:
		return point, nil
	}
	casted, err := point.value.ConvertTo(sc, tp)
	if err != nil {
		return point, errors.Trace(err)
	}
	valCmpCasted, err := point.value.CompareDatum(sc, &casted)
	if err != nil {
		return point, errors.Trace(err)
	}
	point.value = casted
	if valCmpCasted == 0 {
		return point, nil
	}
	if point.start {
		if point.excl {
			if valCmpCasted < 0 {
				// e.g. "a > 1.9" convert to "a >= 2".
				point.excl = false
			}
		} else {
			if valCmpCasted > 0 {
				// e.g. "a >= 1.1 convert to "a > 1"
				point.excl = true
			}
		}
	} else {
		if point.excl {
			if valCmpCasted > 0 {
				// e.g. "a < 1.1" convert to "a <= 1"
				point.excl = false
			}
		} else {
			if valCmpCasted < 0 {
				// e.g. "a <= 1.9" convert to "a < 2"
				point.excl = true
			}
		}
	}
	return point, nil
}

// appendPoints2Ranges appends additional column ranges for multi-column index.
// The additional column ranges can only be appended to point ranges.
// for example we have an index (a, b), if the condition is (a > 1 and b = 2)
// then we can not build a conjunctive ranges for this index.
func appendPoints2Ranges(sc *stmtctx.StatementContext, origin []*Range, rangePoints []point,
	ft *types.FieldType) ([]*Range, error) {
	var newIndexRanges []*Range
	for i := 0; i < len(origin); i++ {
		oRange := origin[i]
		if !oRange.IsPoint(sc) {
			newIndexRanges = append(newIndexRanges, oRange)
		} else {
			newRanges, err := appendPoints2IndexRange(sc, oRange, rangePoints, ft)
			if err != nil {
				return nil, errors.Trace(err)
			}
			newIndexRanges = append(newIndexRanges, newRanges...)
		}
	}
	return newIndexRanges, nil
}

func appendPoints2IndexRange(sc *stmtctx.StatementContext, origin *Range, rangePoints []point,
	ft *types.FieldType) ([]*Range, error) {
	newRanges := make([]*Range, 0, len(rangePoints)/2)
	for i := 0; i < len(rangePoints); i += 2 {
		startPoint, err := convertPoint(sc, rangePoints[i], ft)
		if err != nil {
			return nil, errors.Trace(err)
		}
		endPoint, err := convertPoint(sc, rangePoints[i+1], ft)
		if err != nil {
			return nil, errors.Trace(err)
		}
		less, err := validInterval(sc, startPoint, endPoint)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !less {
			continue
		}

		lowVal := make([]types.Datum, len(origin.LowVal)+1)
		copy(lowVal, origin.LowVal)
		lowVal[len(origin.LowVal)] = startPoint.value

		highVal := make([]types.Datum, len(origin.HighVal)+1)
		copy(highVal, origin.HighVal)
		highVal[len(origin.HighVal)] = endPoint.value

		ir := &Range{
			LowVal:      lowVal,
			LowExclude:  startPoint.excl,
			HighVal:     highVal,
			HighExclude: endPoint.excl,
		}
		newRanges = append(newRanges, ir)
	}
	return newRanges, nil
}

// points2TableRanges build ranges for table scan from range points.
// It will remove the nil and convert MinNotNull and MaxValue to MinInt64 or MinUint64 and MaxInt64 or MaxUint64.
func points2TableRanges(sc *stmtctx.StatementContext, rangePoints []point, tp *types.FieldType) ([]*Range, error) {
	ranges := make([]*Range, 0, len(rangePoints)/2)
	var minValueDatum, maxValueDatum types.Datum
	// Currently, table's kv range cannot accept encoded value of MaxValueDatum. we need to convert it.
	if mysql.HasUnsignedFlag(tp.Flag) {
		minValueDatum.SetUint64(0)
		maxValueDatum.SetUint64(math.MaxUint64)
	} else {
		minValueDatum.SetInt64(math.MinInt64)
		maxValueDatum.SetInt64(math.MaxInt64)
	}
	for i := 0; i < len(rangePoints); i += 2 {
		startPoint, err := convertPoint(sc, rangePoints[i], tp)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if startPoint.value.Kind() == types.KindNull {
			startPoint.value = minValueDatum
			startPoint.excl = false
		} else if startPoint.value.Kind() == types.KindMinNotNull {
			startPoint.value = minValueDatum
		}
		endPoint, err := convertPoint(sc, rangePoints[i+1], tp)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if endPoint.value.Kind() == types.KindMaxValue {
			endPoint.value = maxValueDatum
		} else if endPoint.value.Kind() == types.KindNull {
			continue
		}
		less, err := validInterval(sc, startPoint, endPoint)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !less {
			continue
		}
		ran := &Range{
			LowVal:      []types.Datum{startPoint.value},
			LowExclude:  startPoint.excl,
			HighVal:     []types.Datum{endPoint.value},
			HighExclude: endPoint.excl,
		}
		ranges = append(ranges, ran)
	}
	return ranges, nil
}

// BuildTableRange will build range of pk for PhysicalTableScan
func BuildTableRange(accessConditions []expression.Expression, sc *stmtctx.StatementContext, tp *types.FieldType) ([]*Range, error) {
	rb := builder{sc: sc}
	rangePoints := fullRange
	for _, cond := range accessConditions {
		rangePoints = rb.intersection(rangePoints, rb.build(cond))
		if rb.err != nil {
			return nil, errors.Trace(rb.err)
		}
	}
	newTp := newFieldType(tp)
	ranges, err := points2TableRanges(sc, rangePoints, newTp)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ranges, nil
}

// BuildColumnRange builds the range for sampling histogram to calculate the row count.
func BuildColumnRange(conds []expression.Expression, sc *stmtctx.StatementContext, tp *types.FieldType) ([]*Range, error) {
	if len(conds) == 0 {
		return []*Range{{LowVal: []types.Datum{{}}, HighVal: []types.Datum{types.MaxValueDatum()}}}, nil
	}

	rb := builder{sc: sc}
	rangePoints := fullRange
	for _, cond := range conds {
		rangePoints = rb.intersection(rangePoints, rb.build(cond))
		if rb.err != nil {
			return nil, errors.Trace(rb.err)
		}
	}
	newTp := newFieldType(tp)
	ranges, err := points2Ranges(sc, rangePoints, newTp)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ranges, nil
}

// buildCNFIndexRange builds the range for index where the top layer is CNF.
func buildCNFIndexRange(sc *stmtctx.StatementContext, cols []*expression.Column, newTp []*types.FieldType, lengths []int,
	eqAndInCount int, accessCondition []expression.Expression) ([]*Range, error) {
	rb := builder{sc: sc}
	var (
		ranges []*Range
		err    error
	)
	for _, col := range cols {
		newTp = append(newTp, newFieldType(col.RetType))
	}
	for i := 0; i < eqAndInCount; i++ {
		if sf, ok := accessCondition[i].(*expression.ScalarFunction); !ok || (sf.FuncName.L != ast.EQ && sf.FuncName.L != ast.In) {
			break
		}
		// Build ranges for equal or in access conditions.
		point := rb.build(accessCondition[i])
		if rb.err != nil {
			return nil, errors.Trace(rb.err)
		}
		if i == 0 {
			ranges, err = points2Ranges(sc, point, newTp[i])
		} else {
			ranges, err = appendPoints2Ranges(sc, ranges, point, newTp[i])
		}
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	rangePoints := fullRange
	// Build rangePoints for non-equal access conditions.
	for i := eqAndInCount; i < len(accessCondition); i++ {
		rangePoints = rb.intersection(rangePoints, rb.build(accessCondition[i]))
		if rb.err != nil {
			return nil, errors.Trace(rb.err)
		}
	}
	if eqAndInCount == 0 {
		ranges, err = points2Ranges(sc, rangePoints, newTp[0])
	} else if eqAndInCount < len(accessCondition) {
		ranges, err = appendPoints2Ranges(sc, ranges, rangePoints, newTp[eqAndInCount])
	}
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Take prefix index into consideration.
	if hasPrefix(lengths) {
		fixPrefixColRange(ranges, lengths, newTp)
	}

	if len(ranges) > 0 && len(ranges[0].LowVal) < len(cols) {
		for _, ran := range ranges {
			if ran.HighExclude || ran.LowExclude {
				if ran.HighExclude {
					ran.HighVal = append(ran.HighVal, types.NewDatum(nil))
				} else {
					ran.HighVal = append(ran.HighVal, types.MaxValueDatum())
				}
				if ran.LowExclude {
					ran.LowVal = append(ran.LowVal, types.MaxValueDatum())
				} else {
					ran.LowVal = append(ran.LowVal, types.NewDatum(nil))
				}
			}
		}
	}
	return ranges, nil
}

type sortRange struct {
	originalValue *Range
	encodedStart  []byte
	encodedEnd    []byte
}

func unionRanges(sc *stmtctx.StatementContext, ranges []*Range) ([]*Range, error) {
	if len(ranges) == 0 {
		return nil, nil
	}
	objects := make([]*sortRange, 0, len(ranges))
	for _, ran := range ranges {
		left, err := codec.EncodeKey(sc, nil, ran.LowVal...)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if ran.LowExclude {
			left = kv.Key(left).PrefixNext()
		}
		right, err := codec.EncodeKey(sc, nil, ran.HighVal...)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !ran.HighExclude {
			right = kv.Key(right).PrefixNext()
		}
		objects = append(objects, &sortRange{originalValue: ran, encodedStart: left, encodedEnd: right})
	}
	sort.Slice(objects, func(i, j int) bool {
		return bytes.Compare(objects[i].encodedStart, objects[j].encodedStart) < 0
	})
	ranges = ranges[:0]
	lastRange := objects[0]
	for i := 1; i < len(objects); i++ {
		// For two intervals [a, b], [c, d], we have guaranteed that a >= c. If b >= c. Then two intervals are overlapped.
		// And this two can be merged as [a, max(b, d)].
		// Otherwise they aren't overlapped.
		if bytes.Compare(lastRange.encodedEnd, objects[i].encodedStart) >= 0 {
			if bytes.Compare(lastRange.encodedEnd, objects[i].encodedEnd) < 0 {
				lastRange.encodedEnd = objects[i].encodedEnd
				lastRange.originalValue.HighVal = objects[i].originalValue.HighVal
				lastRange.originalValue.HighExclude = objects[i].originalValue.HighExclude
			}
		} else {
			ranges = append(ranges, lastRange.originalValue)
			lastRange = objects[i]
		}
	}
	ranges = append(ranges, lastRange.originalValue)
	return ranges, nil
}

func hasPrefix(lengths []int) bool {
	for _, l := range lengths {
		if l != types.UnspecifiedLength {
			return true
		}
	}
	return false
}

func fixPrefixColRange(ranges []*Range, lengths []int, tp []*types.FieldType) {
	for _, ran := range ranges {
		for i := 0; i < len(ran.LowVal); i++ {
			fixRangeDatum(&ran.LowVal[i], lengths[i], tp[i])
		}
		ran.LowExclude = false
		for i := 0; i < len(ran.HighVal); i++ {
			fixRangeDatum(&ran.HighVal[i], lengths[i], tp[i])
		}
		ran.HighExclude = false
	}
}

func fixRangeDatum(v *types.Datum, length int, tp *types.FieldType) {
	// If this column is prefix and the prefix length is smaller than the range, cut it.
	// In case of UTF8, prefix should be cut by characters rather than bytes
	if v.Kind() == types.KindString || v.Kind() == types.KindBytes {
		colCharset := tp.Charset
		colValue := v.GetBytes()
		isUTF8Charset := colCharset == charset.CharsetUTF8 || colCharset == charset.CharsetUTF8MB4
		if isUTF8Charset {
			if length != types.UnspecifiedLength && utf8.RuneCount(colValue) > length {
				rs := bytes.Runes(colValue)
				truncateStr := string(rs[:length])
				// truncate value and limit its length
				v.SetString(truncateStr)
			}
		} else if length != types.UnspecifiedLength && len(colValue) > length {
			// truncate value and limit its length
			v.SetBytes(colValue[:length])
		}
	}
}

// We cannot use the FieldType of column directly. e.g. the column a is int32 and we have a > 1111111111111111111.
// Obviously the constant is bigger than MaxInt32, so we will get overflow error if we use the FieldType of column a.
func newFieldType(tp *types.FieldType) *types.FieldType {
	switch tp.Tp {
	// To avoid overflow error.
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong:
		newTp := types.NewFieldType(mysql.TypeLonglong)
		newTp.Flag = tp.Flag
		newTp.Charset = tp.Charset
		return newTp
	// To avoid data truncate error.
	case mysql.TypeFloat, mysql.TypeDouble, mysql.TypeBlob, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeLongBlob,
		mysql.TypeString, mysql.TypeVarchar, mysql.TypeVarString:
		newTp := types.NewFieldType(tp.Tp)
		newTp.Charset = tp.Charset
		return newTp
	default:
		return tp
	}
}

// points2EqOrInCond constructs a 'EQUAL' or 'IN' scalar function based on the
// 'points'. The target column is extracted from the 'expr'.
// NOTE:
// 1. 'expr' must be either 'EQUAL' or 'IN' function.
// 2. 'points' should not be empty.
func points2EqOrInCond(ctx sessionctx.Context, points []point, expr expression.Expression) expression.Expression {
	// len(points) cannot be 0 here, since we impose early termination in extractEqAndInCondition
	sf, _ := expr.(*expression.ScalarFunction)
	// Constant and Column args should have same RetType, simply get from first arg
	retType := sf.GetArgs()[0].GetType()
	args := make([]expression.Expression, 0, len(points)/2)
	if sf.FuncName.L == ast.EQ {
		if c, ok := sf.GetArgs()[0].(*expression.Column); ok {
			args = append(args, c)
		} else if c, ok := sf.GetArgs()[1].(*expression.Column); ok {
			args = append(args, c)
		}
	} else {
		args = append(args, sf.GetArgs()[0])
	}
	for i := 0; i < len(points); i = i + 2 {
		value := &expression.Constant{
			Value:   points[i].value,
			RetType: retType,
		}
		args = append(args, value)
	}
	funcName := ast.EQ
	if len(args) > 2 {
		funcName = ast.In
	}
	f := expression.NewFunctionInternal(ctx, funcName, sf.GetType(), args...)
	return f
}
