// Copyright 2016 PingCAP, Inc.
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
	"bytes"
	"fmt"
	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tipb/go-tipb"
)

// Aggregation stands for aggregate functions.
type Aggregation interface {
	// Update during executing.
	Update(evalCtx *AggEvaluateContext, sc *stmtctx.StatementContext, row chunk.Row) error

	// GetPartialResult will called by coprocessor to get partial results. For avg function, partial results will return
	// sum and count values at the same time.
	GetPartialResult(evalCtx *AggEvaluateContext) []types.Datum

	// GetResult will be called when all data have been processed.
	GetResult(evalCtx *AggEvaluateContext) types.Datum

	// CreateContext creates a new AggEvaluateContext for the aggregation function.
	CreateContext(sc *stmtctx.StatementContext) *AggEvaluateContext

	// ResetContext resets the content of the evaluate context.
	ResetContext(sc *stmtctx.StatementContext, evalCtx *AggEvaluateContext)

	// GetFinalAggFunc constructs the final agg functions, only used in parallel execution.
	GetFinalAggFunc(ctx sessionctx.Context, idx int) (int, Aggregation)

	// GetArgs gets the args of the aggregate function.
	GetArgs() []expression.Expression

	// Clone deep copy the Aggregation.
	Clone(ctx sessionctx.Context) Aggregation
}

// NewDistAggFunc creates new Aggregate function for mock tikv.
func NewDistAggFunc(expr *tipb.Expr, fieldTps []*types.FieldType, sc *stmtctx.StatementContext) (Aggregation, error) {
	args := make([]expression.Expression, 0, len(expr.Children))
	for _, child := range expr.Children {
		arg, err := expression.PBToExpr(child, fieldTps, sc)
		if err != nil {
			return nil, errors.Trace(err)
		}
		args = append(args, arg)
	}
	switch expr.Tp {
	case tipb.ExprType_Sum:
		return &sumFunction{aggFunction: newAggFunc(ast.AggFuncSum, args, false)}, nil
	case tipb.ExprType_Count:
		return &countFunction{aggFunction: newAggFunc(ast.AggFuncCount, args, false)}, nil
	case tipb.ExprType_Avg:
		return &avgFunction{aggFunction: newAggFunc(ast.AggFuncAvg, args, false)}, nil
	case tipb.ExprType_GroupConcat:
		return &concatFunction{aggFunction: newAggFunc(ast.AggFuncGroupConcat, args, false)}, nil
	case tipb.ExprType_Max:
		return &maxMinFunction{aggFunction: newAggFunc(ast.AggFuncMax, args, false), isMax: true}, nil
	case tipb.ExprType_Min:
		return &maxMinFunction{aggFunction: newAggFunc(ast.AggFuncMin, args, false)}, nil
	case tipb.ExprType_First:
		return &firstRowFunction{aggFunction: newAggFunc(ast.AggFuncFirstRow, args, false)}, nil
	case tipb.ExprType_Agg_BitOr:
		return &bitOrFunction{aggFunction: newAggFunc(ast.AggFuncBitOr, args, false)}, nil
	case tipb.ExprType_Agg_BitXor:
		return &bitXorFunction{aggFunction: newAggFunc(ast.AggFuncBitXor, args, false)}, nil
	case tipb.ExprType_Agg_BitAnd:
		return &bitAndFunction{aggFunction: newAggFunc(ast.AggFuncBitAnd, args, false)}, nil
	}
	return nil, errors.Errorf("Unknown aggregate function type %v", expr.Tp)
}

// AggEvaluateContext is used to store intermediate result when calculating aggregate functions.
type AggEvaluateContext struct {
	DistinctChecker *distinctChecker
	Count           int64
	Value           types.Datum
	Buffer          *bytes.Buffer // Buffer is used for group_concat.
	GotFirstRow     bool          // It will check if the agg has met the first row key.
}

// AggFunctionMode stands for the aggregation function's mode.
type AggFunctionMode int

// |-----------------|--------------|--------------|
// | AggFunctionMode | input        | output       |
// |-----------------|--------------|--------------|
// | CompleteMode    | origin data  | final result |
// | FinalMode       | partial data | final result |
// | Partial1Mode    | origin data  | partial data |
// | Partial2Mode    | partial data | partial data |
// | DedupMode       | origin data  | origin data  |
// |-----------------|--------------|--------------|
const (
	CompleteMode AggFunctionMode = iota
	FinalMode
	Partial1Mode
	Partial2Mode
	DedupMode
)

type aggFunction struct {
	*AggFuncDesc
}

func newAggFunc(funcName string, args []expression.Expression, hasDistinct bool) aggFunction {
	return aggFunction{AggFuncDesc: &AggFuncDesc{
		Name:        funcName,
		Args:        args,
		HasDistinct: hasDistinct,
	}}
}

// CreateContext implements Aggregation interface.
func (af *aggFunction) CreateContext(sc *stmtctx.StatementContext) *AggEvaluateContext {
	evalCtx := &AggEvaluateContext{}
	if af.HasDistinct {
		evalCtx.DistinctChecker = createDistinctChecker(sc)
	}
	return evalCtx
}

func (af *aggFunction) ResetContext(sc *stmtctx.StatementContext, evalCtx *AggEvaluateContext) {
	if af.HasDistinct {
		evalCtx.DistinctChecker = createDistinctChecker(sc)
	}
	evalCtx.Value.SetNull()
}

func (af *aggFunction) updateSum(sc *stmtctx.StatementContext, evalCtx *AggEvaluateContext, row chunk.Row) error {
	a := af.Args[0]
	value, err := a.Eval(row)
	if err != nil {
		return errors.Trace(err)
	}
	if value.IsNull() {
		return nil
	}
	if af.HasDistinct {
		d, err1 := evalCtx.DistinctChecker.Check([]types.Datum{value})
		if err1 != nil {
			return errors.Trace(err1)
		}
		if !d {
			return nil
		}
	}
	evalCtx.Value, err = calculateSum(sc, evalCtx.Value, value)
	if err != nil {
		return errors.Trace(err)
	}
	evalCtx.Count++
	return nil
}

func (af *aggFunction) GetFinalAggFunc(ctx sessionctx.Context, idx int) (_ int, newAggFunc Aggregation) {
	switch af.Mode {
	case DedupMode:
		panic("DedupMode is not supported now.")
	case Partial1Mode:
		args := make([]expression.Expression, 0, 2)
		if NeedCount(af.Name) {
			args = append(args, &expression.Column{
				ColName: model.NewCIStr(fmt.Sprintf("col_%d", idx)),
				Index:   idx,
				RetType: &types.FieldType{Tp: mysql.TypeLonglong, Flen: 21, Charset: charset.CharsetBin, Collate: charset.CollationBin},
			})
			idx++
		}
		if NeedValue(af.Name) {
			args = append(args, &expression.Column{
				ColName: model.NewCIStr(fmt.Sprintf("col_%d", idx)),
				Index:   idx,
				RetType: af.RetTp,
			})
			idx++
			if af.Name == ast.AggFuncGroupConcat {
				separator := af.Args[len(af.Args)-1]
				args = append(args, separator.Clone())
			}
		}
		desc := af.AggFuncDesc.Clone()
		desc.Mode = FinalMode
		desc.Args = args
		newAggFunc = desc.GetAggFunc(ctx)
	case Partial2Mode:
		desc := af.AggFuncDesc.Clone()
		desc.Mode = FinalMode
		idx += len(desc.Args)
		newAggFunc = desc.GetAggFunc(ctx)
	case FinalMode, CompleteMode:
		panic("GetFinalAggFunc should not be called when aggMode is FinalMode/CompleteMode.")
	}
	return idx, newAggFunc
}

func (af *aggFunction) GetArgs() []expression.Expression {
	return af.Args
}

func (af *aggFunction) Clone(ctx sessionctx.Context) Aggregation {
	desc := af.AggFuncDesc.Clone()
	return desc.GetAggFunc(ctx)
}

// NeedCount indicates whether the aggregate function should record count.
func NeedCount(name string) bool {
	return name == ast.AggFuncCount || name == ast.AggFuncAvg
}

// NeedValue indicates whether the aggregate function should record value.
func NeedValue(name string) bool {
	switch name {
	case ast.AggFuncSum, ast.AggFuncAvg, ast.AggFuncFirstRow, ast.AggFuncMax, ast.AggFuncMin,
		ast.AggFuncGroupConcat, ast.AggFuncBitOr, ast.AggFuncBitAnd, ast.AggFuncBitXor:
		return true
	default:
		return false
	}
}

// IsAllFirstRow checks whether functions in `aggFuncs` are all FirstRow.
func IsAllFirstRow(aggFuncs []*AggFuncDesc) bool {
	for _, fun := range aggFuncs {
		if fun.Name != ast.AggFuncFirstRow {
			return false
		}
	}
	return true
}
