// Copyright 2015 PingCAP, Inc.
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
	"fmt"
	"math"
	"sort"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
)

// Error instances.
var (
	ErrUnsupportedType = terror.ClassOptimizer.New(CodeUnsupportedType, "Unsupported type")
)

// Error codes.
const (
	CodeUnsupportedType terror.ErrCode = 1
)

// RangeType is alias for int.
type RangeType int

// RangeType constants.
const (
	IntRangeType RangeType = iota
	ColumnRangeType
	IndexRangeType
)

// Point is the end point of range interval.
type point struct {
	value types.Datum
	excl  bool // exclude
	start bool
}

func (rp point) String() string {
	val := rp.value.GetValue()
	if rp.value.Kind() == types.KindMinNotNull {
		val = "-inf"
	} else if rp.value.Kind() == types.KindMaxValue {
		val = "+inf"
	}
	if rp.start {
		symbol := "["
		if rp.excl {
			symbol = "("
		}
		return fmt.Sprintf("%s%v", symbol, val)
	}
	symbol := "]"
	if rp.excl {
		symbol = ")"
	}
	return fmt.Sprintf("%v%s", val, symbol)
}

type pointSorter struct {
	points []point
	err    error
	sc     *stmtctx.StatementContext
}

func (r *pointSorter) Len() int {
	return len(r.points)
}

func (r *pointSorter) Less(i, j int) bool {
	a := r.points[i]
	b := r.points[j]
	less, err := rangePointLess(r.sc, a, b)
	if err != nil {
		r.err = err
	}
	return less
}

func rangePointLess(sc *stmtctx.StatementContext, a, b point) (bool, error) {
	cmp, err := a.value.CompareDatum(sc, &b.value)
	if cmp != 0 {
		return cmp < 0, nil
	}
	return rangePointEqualValueLess(a, b), errors.Trace(err)
}

func rangePointEqualValueLess(a, b point) bool {
	if a.start && b.start {
		return !a.excl && b.excl
	} else if a.start {
		return !a.excl && !b.excl
	} else if b.start {
		return a.excl || b.excl
	}
	return a.excl && !b.excl
}

func (r *pointSorter) Swap(i, j int) {
	r.points[i], r.points[j] = r.points[j], r.points[i]
}

// fullRange is (-∞, +∞).
var fullRange = []point{
	{start: true},
	{value: types.MaxValueDatum()},
}

// FullIntRange is used for table range. Since table range cannot accept MaxValueDatum as the max value.
// So we need to set it to MaxInt64.
func FullIntRange(isUnsigned bool) []*Range {
	if isUnsigned {
		return []*Range{{LowVal: []types.Datum{types.NewUintDatum(0)}, HighVal: []types.Datum{types.NewUintDatum(math.MaxUint64)}}}
	}
	return []*Range{{LowVal: []types.Datum{types.NewIntDatum(math.MinInt64)}, HighVal: []types.Datum{types.NewIntDatum(math.MaxInt64)}}}
}

// FullRange is (-∞, +∞) for Range.
func FullRange() []*Range {
	return []*Range{{LowVal: []types.Datum{{}}, HighVal: []types.Datum{types.MaxValueDatum()}}}
}

// builder is the range builder struct.
type builder struct {
	err error
	sc  *stmtctx.StatementContext
}

func (r *builder) build(expr expression.Expression) []point {
	switch x := expr.(type) {
	case *expression.Column:
		return r.buildFromColumn(x)
	case *expression.ScalarFunction:
		return r.buildFromScalarFunc(x)
	case *expression.Constant:
		return r.buildFromConstant(x)
	}

	return fullRange
}

func (r *builder) buildFromConstant(expr *expression.Constant) []point {
	dt, err := expr.Eval(chunk.Row{})
	if err != nil {
		r.err = err
		return nil
	}
	if dt.IsNull() {
		return nil
	}

	val, err := dt.ToBool(r.sc)
	if err != nil {
		r.err = err
		return nil
	}

	if val == 0 {
		return nil
	}
	return fullRange
}

func (r *builder) buildFromColumn(expr *expression.Column) []point {
	// column name expression is equivalent to column name is true.
	startPoint1 := point{value: types.MinNotNullDatum(), start: true}
	endPoint1 := point{excl: true}
	endPoint1.value.SetInt64(0)
	startPoint2 := point{excl: true, start: true}
	startPoint2.value.SetInt64(0)
	endPoint2 := point{value: types.MaxValueDatum()}
	return []point{startPoint1, endPoint1, startPoint2, endPoint2}
}

func (r *builder) buildFormBinOp(expr *expression.ScalarFunction) []point {
	// This has been checked that the binary operation is comparison operation, and one of
	// the operand is column name expression.
	var (
		op    string
		value types.Datum
		err   error
	)
	if _, ok := expr.GetArgs()[0].(*expression.Column); ok {
		value, err = expr.GetArgs()[1].Eval(chunk.Row{})
		op = expr.FuncName.L
	} else {
		value, err = expr.GetArgs()[0].Eval(chunk.Row{})
		switch expr.FuncName.L {
		case ast.GE:
			op = ast.LE
		case ast.GT:
			op = ast.LT
		case ast.LT:
			op = ast.GT
		case ast.LE:
			op = ast.GE
		default:
			op = expr.FuncName.L
		}
	}
	if err != nil {
		return nil
	}
	if value.IsNull() {
		return nil
	}

	switch op {
	case ast.EQ:
		startPoint := point{value: value, start: true}
		endPoint := point{value: value}
		return []point{startPoint, endPoint}
	case ast.NE:
		startPoint1 := point{value: types.MinNotNullDatum(), start: true}
		endPoint1 := point{value: value, excl: true}
		startPoint2 := point{value: value, start: true, excl: true}
		endPoint2 := point{value: types.MaxValueDatum()}
		return []point{startPoint1, endPoint1, startPoint2, endPoint2}
	case ast.LT:
		startPoint := point{value: types.MinNotNullDatum(), start: true}
		endPoint := point{value: value, excl: true}
		return []point{startPoint, endPoint}
	case ast.LE:
		startPoint := point{value: types.MinNotNullDatum(), start: true}
		endPoint := point{value: value}
		return []point{startPoint, endPoint}
	case ast.GT:
		startPoint := point{value: value, start: true, excl: true}
		endPoint := point{value: types.MaxValueDatum()}
		return []point{startPoint, endPoint}
	case ast.GE:
		startPoint := point{value: value, start: true}
		endPoint := point{value: types.MaxValueDatum()}
		return []point{startPoint, endPoint}
	}
	return nil
}

func (r *builder) buildFromIsTrue(expr *expression.ScalarFunction, isNot int) []point {
	if isNot == 1 {
		// NOT TRUE range is {[null null] [0, 0]}
		startPoint1 := point{start: true}
		endPoint1 := point{}
		startPoint2 := point{start: true}
		startPoint2.value.SetInt64(0)
		endPoint2 := point{}
		endPoint2.value.SetInt64(0)
		return []point{startPoint1, endPoint1, startPoint2, endPoint2}
	}
	// TRUE range is {[-inf 0) (0 +inf]}
	startPoint1 := point{value: types.MinNotNullDatum(), start: true}
	endPoint1 := point{excl: true}
	endPoint1.value.SetInt64(0)
	startPoint2 := point{excl: true, start: true}
	startPoint2.value.SetInt64(0)
	endPoint2 := point{value: types.MaxValueDatum()}
	return []point{startPoint1, endPoint1, startPoint2, endPoint2}
}

func (r *builder) buildFromIsFalse(expr *expression.ScalarFunction, isNot int) []point {
	if isNot == 1 {
		// NOT FALSE range is {[-inf, 0), (0, +inf], [null, null]}
		startPoint1 := point{start: true}
		endPoint1 := point{excl: true}
		endPoint1.value.SetInt64(0)
		startPoint2 := point{start: true, excl: true}
		startPoint2.value.SetInt64(0)
		endPoint2 := point{value: types.MaxValueDatum()}
		return []point{startPoint1, endPoint1, startPoint2, endPoint2}
	}
	// FALSE range is {[0, 0]}
	startPoint := point{start: true}
	startPoint.value.SetInt64(0)
	endPoint := point{}
	endPoint.value.SetInt64(0)
	return []point{startPoint, endPoint}
}

func (r *builder) buildFromIn(expr *expression.ScalarFunction) ([]point, bool) {
	list := expr.GetArgs()[1:]
	rangePoints := make([]point, 0, len(list)*2)
	hasNull := false
	for _, e := range list {
		v, ok := e.(*expression.Constant)
		if !ok {
			r.err = ErrUnsupportedType.GenWithStack("expr:%v is not constant", e)
			return fullRange, hasNull
		}
		dt, err := v.Eval(chunk.Row{})
		if err != nil {
			r.err = ErrUnsupportedType.GenWithStack("expr:%v is not evaluated", e)
			return fullRange, hasNull
		}
		if dt.IsNull() {
			hasNull = true
			continue
		}
		startPoint := point{value: types.NewDatum(dt.GetValue()), start: true}
		endPoint := point{value: types.NewDatum(dt.GetValue())}
		rangePoints = append(rangePoints, startPoint, endPoint)
	}
	sorter := pointSorter{points: rangePoints, sc: r.sc}
	sort.Sort(&sorter)
	if sorter.err != nil {
		r.err = sorter.err
	}
	// check and remove duplicates
	curPos, frontPos := 0, 0
	for frontPos < len(rangePoints) {
		if rangePoints[curPos].start == rangePoints[frontPos].start {
			frontPos++
		} else {
			curPos++
			rangePoints[curPos] = rangePoints[frontPos]
			frontPos++
		}
	}
	if curPos > 0 {
		curPos++
	}
	return rangePoints[:curPos], hasNull
}

func (r *builder) newBuildFromPatternLike(expr *expression.ScalarFunction) []point {
	pdt, err := expr.GetArgs()[1].(*expression.Constant).Eval(chunk.Row{})
	if err != nil {
		r.err = errors.Trace(err)
		return fullRange
	}
	pattern, err := pdt.ToString()
	if err != nil {
		r.err = errors.Trace(err)
		return fullRange
	}
	if pattern == "" {
		startPoint := point{value: types.NewStringDatum(""), start: true}
		endPoint := point{value: types.NewStringDatum("")}
		return []point{startPoint, endPoint}
	}
	lowValue := make([]byte, 0, len(pattern))
	edt, err := expr.GetArgs()[2].(*expression.Constant).Eval(chunk.Row{})
	if err != nil {
		r.err = errors.Trace(err)
		return fullRange
	}
	escape := byte(edt.GetInt64())
	var exclude bool
	isExactMatch := true
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == escape {
			i++
			if i < len(pattern) {
				lowValue = append(lowValue, pattern[i])
			} else {
				lowValue = append(lowValue, escape)
			}
			continue
		}
		if pattern[i] == '%' {
			// Get the prefix.
			isExactMatch = false
			break
		} else if pattern[i] == '_' {
			// Get the prefix, but exclude the prefix.
			// e.g., "abc_x", the start point exclude "abc",
			// because the string length is more than 3.
			exclude = true
			isExactMatch = false
			break
		}
		lowValue = append(lowValue, pattern[i])
	}
	if len(lowValue) == 0 {
		return []point{{value: types.MinNotNullDatum(), start: true}, {value: types.MaxValueDatum()}}
	}
	if isExactMatch {
		val := types.NewStringDatum(string(lowValue))
		return []point{{value: val, start: true}, {value: val}}
	}
	startPoint := point{start: true, excl: exclude}
	startPoint.value.SetBytesAsString(lowValue)
	highValue := make([]byte, len(lowValue))
	copy(highValue, lowValue)
	endPoint := point{excl: true}
	for i := len(highValue) - 1; i >= 0; i-- {
		// Make the end point value more than the start point value,
		// and the length of the end point value is the same as the length of the start point value.
		// e.g., the start point value is "abc", so the end point value is "abd".
		highValue[i]++
		if highValue[i] != 0 {
			endPoint.value.SetBytesAsString(highValue)
			break
		}
		// If highValue[i] is 255 and highValue[i]++ is 0, then the end point value is max value.
		if i == 0 {
			endPoint.value = types.MaxValueDatum()
		}
	}
	return []point{startPoint, endPoint}
}

func (r *builder) buildFromNot(expr *expression.ScalarFunction) []point {
	switch n := expr.FuncName.L; n {
	case ast.IsTruth:
		return r.buildFromIsTrue(expr, 1)
	case ast.IsFalsity:
		return r.buildFromIsFalse(expr, 1)
	case ast.In:
		var (
			isUnsignedIntCol bool
			nonNegativePos   int
		)
		rangePoints, hasNull := r.buildFromIn(expr)
		if hasNull {
			return nil
		}
		if x, ok := expr.GetArgs()[0].(*expression.Column); ok {
			isUnsignedIntCol = mysql.HasUnsignedFlag(x.RetType.Flag) && mysql.IsIntegerType(x.RetType.Tp)
		}
		// negative ranges can be directly ignored for unsigned int columns.
		if isUnsignedIntCol {
			for nonNegativePos = 0; nonNegativePos < len(rangePoints); nonNegativePos += 2 {
				if rangePoints[nonNegativePos].value.Kind() == types.KindUint64 || rangePoints[nonNegativePos].value.GetInt64() >= 0 {
					break
				}
			}
			rangePoints = rangePoints[nonNegativePos:]
		}
		retRangePoints := make([]point, 0, 2+len(rangePoints))
		previousValue := types.Datum{}
		for i := 0; i < len(rangePoints); i += 2 {
			retRangePoints = append(retRangePoints, point{value: previousValue, start: true, excl: true})
			retRangePoints = append(retRangePoints, point{value: rangePoints[i].value, excl: true})
			previousValue = rangePoints[i].value
		}
		// Append the interval (last element, max value].
		retRangePoints = append(retRangePoints, point{value: previousValue, start: true, excl: true})
		retRangePoints = append(retRangePoints, point{value: types.MaxValueDatum()})
		return retRangePoints
	case ast.Like:
		// Pattern not like is not supported.
		r.err = ErrUnsupportedType.GenWithStack("NOT LIKE is not supported.")
		return fullRange
	case ast.IsNull:
		startPoint := point{value: types.MinNotNullDatum(), start: true}
		endPoint := point{value: types.MaxValueDatum()}
		return []point{startPoint, endPoint}
	}
	return nil
}

func (r *builder) buildFromScalarFunc(expr *expression.ScalarFunction) []point {
	switch op := expr.FuncName.L; op {
	case ast.GE, ast.GT, ast.LT, ast.LE, ast.EQ, ast.NE:
		return r.buildFormBinOp(expr)
	case ast.LogicAnd:
		return r.intersection(r.build(expr.GetArgs()[0]), r.build(expr.GetArgs()[1]))
	case ast.LogicOr:
		return r.union(r.build(expr.GetArgs()[0]), r.build(expr.GetArgs()[1]))
	case ast.IsTruth:
		return r.buildFromIsTrue(expr, 0)
	case ast.IsFalsity:
		return r.buildFromIsFalse(expr, 0)
	case ast.In:
		retPoints, _ := r.buildFromIn(expr)
		return retPoints
	case ast.Like:
		return r.newBuildFromPatternLike(expr)
	case ast.IsNull:
		startPoint := point{start: true}
		endPoint := point{}
		return []point{startPoint, endPoint}
	case ast.UnaryNot:
		return r.buildFromNot(expr.GetArgs()[0].(*expression.ScalarFunction))
	}

	return nil
}

func (r *builder) intersection(a, b []point) []point {
	return r.merge(a, b, false)
}

func (r *builder) union(a, b []point) []point {
	return r.merge(a, b, true)
}

func (r *builder) merge(a, b []point, union bool) []point {
	sorter := pointSorter{points: append(a, b...), sc: r.sc}
	sort.Sort(&sorter)
	if sorter.err != nil {
		r.err = sorter.err
		return nil
	}
	var (
		inRangeCount         int
		requiredInRangeCount int
	)
	if union {
		requiredInRangeCount = 1
	} else {
		requiredInRangeCount = 2
	}
	merged := make([]point, 0, len(sorter.points))
	for _, val := range sorter.points {
		if val.start {
			inRangeCount++
			if inRangeCount == requiredInRangeCount {
				// just reached the required in range count, a new range started.
				merged = append(merged, val)
			}
		} else {
			if inRangeCount == requiredInRangeCount {
				// just about to leave the required in range count, the range is ended.
				merged = append(merged, val)
			}
			inRangeCount--
		}
	}
	return merged
}
