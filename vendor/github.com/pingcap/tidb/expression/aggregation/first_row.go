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

type firstRowFunction struct {
	aggFunction
}

// Update implements Aggregation interface.
func (ff *firstRowFunction) Update(evalCtx *AggEvaluateContext, sc *stmtctx.StatementContext, row chunk.Row) error {
	if evalCtx.GotFirstRow {
		return nil
	}
	if len(ff.Args) != 1 {
		return errors.New("Wrong number of args for AggFuncFirstRow")
	}
	value, err := ff.Args[0].Eval(row)
	if err != nil {
		return errors.Trace(err)
	}
	evalCtx.Value = types.CopyDatum(value)
	evalCtx.GotFirstRow = true
	return nil
}

// GetResult implements Aggregation interface.
func (ff *firstRowFunction) GetResult(evalCtx *AggEvaluateContext) types.Datum {
	return evalCtx.Value
}

func (ff *firstRowFunction) ResetContext(_ *stmtctx.StatementContext, evalCtx *AggEvaluateContext) {
	evalCtx.GotFirstRow = false
}

// GetPartialResult implements Aggregation interface.
func (ff *firstRowFunction) GetPartialResult(evalCtx *AggEvaluateContext) []types.Datum {
	return []types.Datum{ff.GetResult(evalCtx)}
}
