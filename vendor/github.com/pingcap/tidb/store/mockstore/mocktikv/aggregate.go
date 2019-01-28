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

package mocktikv

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/expression/aggregation"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/codec"
	"golang.org/x/net/context"
)

type aggCtxsMapper map[string][]*aggregation.AggEvaluateContext

var (
	_ executor = &hashAggExec{}
	_ executor = &streamAggExec{}
)

type hashAggExec struct {
	evalCtx           *evalContext
	aggExprs          []aggregation.Aggregation
	aggCtxsMap        aggCtxsMapper
	groupByExprs      []expression.Expression
	relatedColOffsets []int
	row               []types.Datum
	groups            map[string]struct{}
	groupKeys         [][]byte
	groupKeyRows      [][][]byte
	executed          bool
	currGroupIdx      int
	count             int64

	src executor
}

func (e *hashAggExec) SetSrcExec(exec executor) {
	e.src = exec
}

func (e *hashAggExec) GetSrcExec() executor {
	return e.src
}

func (e *hashAggExec) ResetCounts() {
	e.src.ResetCounts()
}

func (e *hashAggExec) Counts() []int64 {
	return e.src.Counts()
}

func (e *hashAggExec) innerNext(ctx context.Context) (bool, error) {
	values, err := e.src.Next(ctx)
	if err != nil {
		return false, errors.Trace(err)
	}
	if values == nil {
		return false, nil
	}
	err = e.aggregate(values)
	if err != nil {
		return false, errors.Trace(err)
	}
	return true, nil
}

func (e *hashAggExec) Cursor() ([]byte, bool) {
	panic("don't not use coprocessor streaming API for hash aggregation!")
}

func (e *hashAggExec) Next(ctx context.Context) (value [][]byte, err error) {
	e.count++
	if e.aggCtxsMap == nil {
		e.aggCtxsMap = make(aggCtxsMapper, 0)
	}
	if !e.executed {
		for {
			hasMore, err := e.innerNext(ctx)
			if err != nil {
				return nil, errors.Trace(err)
			}
			if !hasMore {
				break
			}
		}
		e.executed = true
	}

	if e.currGroupIdx >= len(e.groups) {
		return nil, nil
	}
	gk := e.groupKeys[e.currGroupIdx]
	value = make([][]byte, 0, len(e.groupByExprs)+2*len(e.aggExprs))
	aggCtxs := e.getContexts(gk)
	for i, agg := range e.aggExprs {
		partialResults := agg.GetPartialResult(aggCtxs[i])
		for _, result := range partialResults {
			data, err := codec.EncodeValue(e.evalCtx.sc, nil, result)
			if err != nil {
				return nil, errors.Trace(err)
			}
			value = append(value, data)
		}
	}
	value = append(value, e.groupKeyRows[e.currGroupIdx]...)
	e.currGroupIdx++

	return value, nil
}

func (e *hashAggExec) getGroupKey() ([]byte, [][]byte, error) {
	length := len(e.groupByExprs)
	if length == 0 {
		return nil, nil, nil
	}
	bufLen := 0
	row := make([][]byte, 0, length)
	for _, item := range e.groupByExprs {
		v, err := item.Eval(chunk.MutRowFromDatums(e.row).ToRow())
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		b, err := codec.EncodeValue(e.evalCtx.sc, nil, v)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		bufLen += len(b)
		row = append(row, b)
	}
	buf := make([]byte, 0, bufLen)
	for _, col := range row {
		buf = append(buf, col...)
	}
	return buf, row, nil
}

// aggregate updates aggregate functions with row.
func (e *hashAggExec) aggregate(value [][]byte) error {
	err := e.evalCtx.decodeRelatedColumnVals(e.relatedColOffsets, value, e.row)
	if err != nil {
		return errors.Trace(err)
	}
	// Get group key.
	gk, gbyKeyRow, err := e.getGroupKey()
	if err != nil {
		return errors.Trace(err)
	}
	if _, ok := e.groups[string(gk)]; !ok {
		e.groups[string(gk)] = struct{}{}
		e.groupKeys = append(e.groupKeys, gk)
		e.groupKeyRows = append(e.groupKeyRows, gbyKeyRow)
	}
	// Update aggregate expressions.
	aggCtxs := e.getContexts(gk)
	for i, agg := range e.aggExprs {
		err = agg.Update(aggCtxs[i], e.evalCtx.sc, chunk.MutRowFromDatums(e.row).ToRow())
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (e *hashAggExec) getContexts(groupKey []byte) []*aggregation.AggEvaluateContext {
	groupKeyString := string(groupKey)
	aggCtxs, ok := e.aggCtxsMap[groupKeyString]
	if !ok {
		aggCtxs = make([]*aggregation.AggEvaluateContext, 0, len(e.aggExprs))
		for _, agg := range e.aggExprs {
			aggCtxs = append(aggCtxs, agg.CreateContext(e.evalCtx.sc))
		}
		e.aggCtxsMap[groupKeyString] = aggCtxs
	}
	return aggCtxs
}

type streamAggExec struct {
	evalCtx           *evalContext
	aggExprs          []aggregation.Aggregation
	aggCtxs           []*aggregation.AggEvaluateContext
	groupByExprs      []expression.Expression
	relatedColOffsets []int
	row               []types.Datum
	tmpGroupByRow     []types.Datum
	currGroupByRow    []types.Datum
	nextGroupByRow    []types.Datum
	currGroupByValues [][]byte
	executed          bool
	hasData           bool
	count             int64

	src executor
}

func (e *streamAggExec) SetSrcExec(exec executor) {
	e.src = exec
}

func (e *streamAggExec) GetSrcExec() executor {
	return e.src
}

func (e *streamAggExec) ResetCounts() {
	e.src.ResetCounts()
}

func (e *streamAggExec) Counts() []int64 {
	return e.src.Counts()
}

func (e *streamAggExec) getPartialResult() ([][]byte, error) {
	value := make([][]byte, 0, len(e.groupByExprs)+2*len(e.aggExprs))
	for i, agg := range e.aggExprs {
		partialResults := agg.GetPartialResult(e.aggCtxs[i])
		for _, result := range partialResults {
			data, err := codec.EncodeValue(e.evalCtx.sc, nil, result)
			if err != nil {
				return nil, errors.Trace(err)
			}
			value = append(value, data)
		}
		// Clear the aggregate context.
		e.aggCtxs[i] = agg.CreateContext(e.evalCtx.sc)
	}
	e.currGroupByValues = e.currGroupByValues[:0]
	for _, d := range e.currGroupByRow {
		buf, err := codec.EncodeValue(e.evalCtx.sc, nil, d)
		if err != nil {
			return nil, errors.Trace(err)
		}
		e.currGroupByValues = append(e.currGroupByValues, buf)
	}
	e.currGroupByRow = types.CopyRow(e.nextGroupByRow)
	return append(value, e.currGroupByValues...), nil
}

func (e *streamAggExec) meetNewGroup(row [][]byte) (bool, error) {
	if len(e.groupByExprs) == 0 {
		return false, nil
	}

	e.tmpGroupByRow = e.tmpGroupByRow[:0]
	matched, firstGroup := true, false
	if e.nextGroupByRow == nil {
		matched, firstGroup = false, true
	}
	for i, item := range e.groupByExprs {
		d, err := item.Eval(chunk.MutRowFromDatums(e.row).ToRow())
		if err != nil {
			return false, errors.Trace(err)
		}
		if matched {
			c, err := d.CompareDatum(e.evalCtx.sc, &e.nextGroupByRow[i])
			if err != nil {
				return false, errors.Trace(err)
			}
			matched = c == 0
		}
		e.tmpGroupByRow = append(e.tmpGroupByRow, d)
	}
	if firstGroup {
		e.currGroupByRow = types.CopyRow(e.tmpGroupByRow)
	}
	if matched {
		return false, nil
	}
	e.nextGroupByRow = e.tmpGroupByRow
	return !firstGroup, nil
}

func (e *streamAggExec) Cursor() ([]byte, bool) {
	panic("don't not use coprocessor streaming API for stream aggregation!")
}

func (e *streamAggExec) Next(ctx context.Context) (retRow [][]byte, err error) {
	e.count++
	if e.executed {
		return nil, nil
	}

	for {
		values, err := e.src.Next(ctx)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if values == nil {
			e.executed = true
			if !e.hasData && len(e.groupByExprs) > 0 {
				return nil, nil
			}
			return e.getPartialResult()
		}

		e.hasData = true
		err = e.evalCtx.decodeRelatedColumnVals(e.relatedColOffsets, values, e.row)
		if err != nil {
			return nil, errors.Trace(err)
		}
		newGroup, err := e.meetNewGroup(values)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if newGroup {
			retRow, err = e.getPartialResult()
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		for i, agg := range e.aggExprs {
			err = agg.Update(e.aggCtxs[i], e.evalCtx.sc, chunk.MutRowFromDatums(e.row).ToRow())
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		if newGroup {
			return retRow, nil
		}
	}
}
