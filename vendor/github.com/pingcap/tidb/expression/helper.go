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

package expression

import (
	"math"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/parser_driver"
)

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

// IsCurrentTimestampExpr returns whether e is CurrentTimestamp expression.
func IsCurrentTimestampExpr(e ast.ExprNode) bool {
	if fn, ok := e.(*ast.FuncCallExpr); ok && fn.FnName.L == ast.CurrentTimestamp {
		return true
	}
	return false
}

// GetTimeValue gets the time value with type tp.
func GetTimeValue(ctx sessionctx.Context, v interface{}, tp byte, fsp int) (d types.Datum, err error) {
	value := types.Time{
		Type: tp,
		Fsp:  fsp,
	}

	defaultTime, err := getSystemTimestamp(ctx)
	if err != nil {
		return d, errors.Trace(err)
	}
	sc := ctx.GetSessionVars().StmtCtx
	switch x := v.(type) {
	case string:
		upperX := strings.ToUpper(x)
		if upperX == strings.ToUpper(ast.CurrentTimestamp) {
			value.Time = types.FromGoTime(defaultTime.Truncate(time.Duration(math.Pow10(9-fsp)) * time.Nanosecond))
			if tp == mysql.TypeTimestamp || tp == mysql.TypeDatetime {
				err = value.ConvertTimeZone(time.Local, ctx.GetSessionVars().Location())
				if err != nil {
					return d, errors.Trace(err)
				}
			}
		} else if upperX == types.ZeroDatetimeStr {
			value, err = types.ParseTimeFromNum(sc, 0, tp, fsp)
			terror.Log(errors.Trace(err))
		} else {
			value, err = types.ParseTime(sc, x, tp, fsp)
			if err != nil {
				return d, errors.Trace(err)
			}
		}
	case *driver.ValueExpr:
		switch x.Kind() {
		case types.KindString:
			value, err = types.ParseTime(sc, x.GetString(), tp, fsp)
			if err != nil {
				return d, errors.Trace(err)
			}
		case types.KindInt64:
			value, err = types.ParseTimeFromNum(sc, x.GetInt64(), tp, fsp)
			if err != nil {
				return d, errors.Trace(err)
			}
		case types.KindNull:
			return d, nil
		default:
			return d, errors.Trace(errDefaultValue)
		}
	case *ast.FuncCallExpr:
		if x.FnName.L == ast.CurrentTimestamp {
			d.SetString(strings.ToUpper(ast.CurrentTimestamp))
			return d, nil
		}
		return d, errors.Trace(errDefaultValue)
	case *ast.UnaryOperationExpr:
		// support some expression, like `-1`
		v, err := EvalAstExpr(ctx, x)
		if err != nil {
			return d, errors.Trace(err)
		}
		ft := types.NewFieldType(mysql.TypeLonglong)
		xval, err := v.ConvertTo(ctx.GetSessionVars().StmtCtx, ft)
		if err != nil {
			return d, errors.Trace(err)
		}

		value, err = types.ParseTimeFromNum(sc, xval.GetInt64(), tp, fsp)
		if err != nil {
			return d, errors.Trace(err)
		}
	default:
		return d, nil
	}
	d.SetMysqlTime(value)
	return d, nil
}

func getSystemTimestamp(ctx sessionctx.Context) (time.Time, error) {
	now := time.Now()

	if ctx == nil {
		return now, nil
	}

	sessionVars := ctx.GetSessionVars()
	timestampStr, err := variable.GetSessionSystemVar(sessionVars, "timestamp")
	if err != nil {
		return now, errors.Trace(err)
	}

	if timestampStr == "" {
		return now, nil
	}
	timestamp, err := types.StrToInt(sessionVars.StmtCtx, timestampStr)
	if err != nil {
		return time.Time{}, errors.Trace(err)
	}
	if timestamp <= 0 {
		return now, nil
	}
	return time.Unix(timestamp, 0), nil
}
