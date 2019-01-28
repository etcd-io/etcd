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

package aggregation

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
)

type maxMinFunction struct {
	aggFunction
	isMax bool
}

// GetResult implements Aggregation interface.
func (mmf *maxMinFunction) GetResult(evalCtx *AggEvaluateContext) (d types.Datum) {
	return evalCtx.Value
}

// GetPartialResult implements Aggregation interface.
func (mmf *maxMinFunction) GetPartialResult(evalCtx *AggEvaluateContext) []types.Datum {
	return []types.Datum{mmf.GetResult(evalCtx)}
}

// Update implements Aggregation interface.
func (mmf *maxMinFunction) Update(evalCtx *AggEvaluateContext, sc *stmtctx.StatementContext, row chunk.Row) error {
	a := mmf.Args[0]
	value, err := a.Eval(row)
	if err != nil {
		return errors.Trace(err)
	}
	if evalCtx.Value.IsNull() {
		evalCtx.Value = *(&value).Copy()
	}
	if value.IsNull() {
		return nil
	}
	var c int
	c, err = evalCtx.Value.CompareDatum(sc, &value)
	if err != nil {
		return errors.Trace(err)
	}
	if (mmf.isMax && c == -1) || (!mmf.isMax && c == 1) {
		evalCtx.Value = *(&value).Copy()
	}
	return nil
}
