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
	"github.com/cznic/mathutil"
	"github.com/pingcap/errors"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
)

type avgFunction struct {
	aggFunction
}

func (af *avgFunction) updateAvg(sc *stmtctx.StatementContext, evalCtx *AggEvaluateContext, row chunk.Row) error {
	a := af.Args[1]
	value, err := a.Eval(row)
	if err != nil {
		return errors.Trace(err)
	}
	if value.IsNull() {
		return nil
	}
	evalCtx.Value, err = calculateSum(sc, evalCtx.Value, value)
	if err != nil {
		return errors.Trace(err)
	}
	count, err := af.Args[0].Eval(row)
	if err != nil {
		return errors.Trace(err)
	}
	evalCtx.Count += count.GetInt64()
	return nil
}

func (af *avgFunction) ResetContext(sc *stmtctx.StatementContext, evalCtx *AggEvaluateContext) {
	if af.HasDistinct {
		evalCtx.DistinctChecker = createDistinctChecker(sc)
	}
	evalCtx.Value.SetNull()
	evalCtx.Count = 0
}

// Update implements Aggregation interface.
func (af *avgFunction) Update(evalCtx *AggEvaluateContext, sc *stmtctx.StatementContext, row chunk.Row) (err error) {
	switch af.Mode {
	case Partial1Mode, CompleteMode:
		err = af.updateSum(sc, evalCtx, row)
	case Partial2Mode, FinalMode:
		err = af.updateAvg(sc, evalCtx, row)
	case DedupMode:
		panic("DedupMode is not supported now.")
	}
	return errors.Trace(err)
}

// GetResult implements Aggregation interface.
func (af *avgFunction) GetResult(evalCtx *AggEvaluateContext) (d types.Datum) {
	switch evalCtx.Value.Kind() {
	case types.KindFloat64:
		sum := evalCtx.Value.GetFloat64()
		d.SetFloat64(sum / float64(evalCtx.Count))
		return
	case types.KindMysqlDecimal:
		x := evalCtx.Value.GetMysqlDecimal()
		y := types.NewDecFromInt(evalCtx.Count)
		to := new(types.MyDecimal)
		err := types.DecimalDiv(x, y, to, types.DivFracIncr)
		terror.Log(errors.Trace(err))
		frac := af.RetTp.Decimal
		if frac == -1 {
			frac = mysql.MaxDecimalScale
		}
		err = to.Round(to, mathutil.Min(frac, mysql.MaxDecimalScale), types.ModeHalfEven)
		terror.Log(errors.Trace(err))
		d.SetMysqlDecimal(to)
	}
	return
}

// GetPartialResult implements Aggregation interface.
func (af *avgFunction) GetPartialResult(evalCtx *AggEvaluateContext) []types.Datum {
	return []types.Datum{types.NewIntDatum(evalCtx.Count), evalCtx.Value}
}
