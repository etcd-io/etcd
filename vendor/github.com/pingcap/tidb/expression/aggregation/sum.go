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
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
)

type sumFunction struct {
	aggFunction
}

// Update implements Aggregation interface.
func (sf *sumFunction) Update(evalCtx *AggEvaluateContext, sc *stmtctx.StatementContext, row chunk.Row) error {
	return sf.updateSum(sc, evalCtx, row)
}

// GetResult implements Aggregation interface.
func (sf *sumFunction) GetResult(evalCtx *AggEvaluateContext) (d types.Datum) {
	return evalCtx.Value
}

// GetPartialResult implements Aggregation interface.
func (sf *sumFunction) GetPartialResult(evalCtx *AggEvaluateContext) []types.Datum {
	return []types.Datum{sf.GetResult(evalCtx)}
}
