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
	"bytes"
	"fmt"

	"github.com/cznic/mathutil"
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
)

type concatFunction struct {
	aggFunction
	separator string
	sepInited bool
	maxLen    uint64
	// truncated according to MySQL, a 'group_concat' function generates exactly one 'truncated' warning during its life time, no matter
	// how many group actually truncated. 'truncated' acts as a sentinel to indicate whether this warning has already been
	// generated.
	truncated bool
}

func (cf *concatFunction) writeValue(evalCtx *AggEvaluateContext, val types.Datum) {
	if val.Kind() == types.KindBytes {
		evalCtx.Buffer.Write(val.GetBytes())
	} else {
		evalCtx.Buffer.WriteString(fmt.Sprintf("%v", val.GetValue()))
	}
}

func (cf *concatFunction) initSeparator(sc *stmtctx.StatementContext, row chunk.Row) error {
	sepArg := cf.Args[len(cf.Args)-1]
	sepDatum, err := sepArg.Eval(row)
	if err != nil {
		return errors.Trace(err)
	}
	if sepDatum.IsNull() {
		return errors.Errorf("Invalid separator argument.")
	}
	cf.separator, err = sepDatum.ToString()
	return errors.Trace(err)
}

// Update implements Aggregation interface.
func (cf *concatFunction) Update(evalCtx *AggEvaluateContext, sc *stmtctx.StatementContext, row chunk.Row) error {
	datumBuf := make([]types.Datum, 0, len(cf.Args))
	if !cf.sepInited {
		err := cf.initSeparator(sc, row)
		if err != nil {
			return errors.Trace(err)
		}
		cf.sepInited = true
	}

	// The last parameter is the concat separator, we only concat the first "len(cf.Args)-1" parameters.
	for i, length := 0, len(cf.Args)-1; i < length; i++ {
		value, err := cf.Args[i].Eval(row)
		if err != nil {
			return errors.Trace(err)
		}
		if value.IsNull() {
			return nil
		}
		datumBuf = append(datumBuf, value)
	}
	if cf.HasDistinct {
		d, err := evalCtx.DistinctChecker.Check(datumBuf)
		if err != nil {
			return errors.Trace(err)
		}
		if !d {
			return nil
		}
	}
	if evalCtx.Buffer == nil {
		evalCtx.Buffer = &bytes.Buffer{}
	} else {
		evalCtx.Buffer.WriteString(cf.separator)
	}
	for _, val := range datumBuf {
		cf.writeValue(evalCtx, val)
	}
	if cf.maxLen > 0 && uint64(evalCtx.Buffer.Len()) > cf.maxLen {
		i := mathutil.MaxInt
		if uint64(i) > cf.maxLen {
			i = int(cf.maxLen)
		}
		evalCtx.Buffer.Truncate(i)
		if !cf.truncated {
			sc.AppendWarning(expression.ErrCutValueGroupConcat.GenWithStackByArgs(cf.Args[0].String()))
		}
		cf.truncated = true
	}
	return nil
}

// GetResult implements Aggregation interface.
func (cf *concatFunction) GetResult(evalCtx *AggEvaluateContext) (d types.Datum) {
	if evalCtx.Buffer != nil {
		d.SetString(evalCtx.Buffer.String())
	} else {
		d.SetNull()
	}
	return d
}

func (cf *concatFunction) ResetContext(sc *stmtctx.StatementContext, evalCtx *AggEvaluateContext) {
	if cf.HasDistinct {
		evalCtx.DistinctChecker = createDistinctChecker(sc)
	}
	evalCtx.Buffer = nil
}

// GetPartialResult implements Aggregation interface.
func (cf *concatFunction) GetPartialResult(evalCtx *AggEvaluateContext) []types.Datum {
	return []types.Datum{cf.GetResult(evalCtx)}
}
