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

package expression

import (
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
)

// Error instances.
var (
	// All the exported errors are defined here:
	ErrIncorrectParameterCount = terror.ClassExpression.New(mysql.ErrWrongParamcountToNativeFct, mysql.MySQLErrName[mysql.ErrWrongParamcountToNativeFct])
	ErrDivisionByZero          = terror.ClassExpression.New(mysql.ErrDivisionByZero, mysql.MySQLErrName[mysql.ErrDivisionByZero])
	ErrRegexp                  = terror.ClassExpression.New(mysql.ErrRegexp, mysql.MySQLErrName[mysql.ErrRegexp])
	ErrOperandColumns          = terror.ClassExpression.New(mysql.ErrOperandColumns, mysql.MySQLErrName[mysql.ErrOperandColumns])
	ErrCutValueGroupConcat     = terror.ClassExpression.New(mysql.ErrCutValueGroupConcat, mysql.MySQLErrName[mysql.ErrCutValueGroupConcat])

	// All the un-exported errors are defined here:
	errFunctionNotExists             = terror.ClassExpression.New(mysql.ErrSpDoesNotExist, mysql.MySQLErrName[mysql.ErrSpDoesNotExist])
	errZlibZData                     = terror.ClassExpression.New(mysql.ErrZlibZData, mysql.MySQLErrName[mysql.ErrZlibZData])
	errZlibZBuf                      = terror.ClassExpression.New(mysql.ErrZlibZBuf, mysql.MySQLErrName[mysql.ErrZlibZBuf])
	errIncorrectArgs                 = terror.ClassExpression.New(mysql.ErrWrongArguments, mysql.MySQLErrName[mysql.ErrWrongArguments])
	errUnknownCharacterSet           = terror.ClassExpression.New(mysql.ErrUnknownCharacterSet, mysql.MySQLErrName[mysql.ErrUnknownCharacterSet])
	errDefaultValue                  = terror.ClassExpression.New(mysql.ErrInvalidDefault, "invalid default value")
	errDeprecatedSyntaxNoReplacement = terror.ClassExpression.New(mysql.ErrWarnDeprecatedSyntaxNoReplacement, mysql.MySQLErrName[mysql.ErrWarnDeprecatedSyntaxNoReplacement])
	errBadField                      = terror.ClassExpression.New(mysql.ErrBadField, mysql.MySQLErrName[mysql.ErrBadField])
	errWarnAllowedPacketOverflowed   = terror.ClassExpression.New(mysql.ErrWarnAllowedPacketOverflowed, mysql.MySQLErrName[mysql.ErrWarnAllowedPacketOverflowed])
	errWarnOptionIgnored             = terror.ClassExpression.New(mysql.WarnOptionIgnored, mysql.MySQLErrName[mysql.WarnOptionIgnored])
	errTruncatedWrongValue           = terror.ClassExpression.New(mysql.ErrTruncatedWrongValue, mysql.MySQLErrName[mysql.ErrTruncatedWrongValue])
)

func init() {
	expressionMySQLErrCodes := map[terror.ErrCode]uint16{
		mysql.ErrWrongParamcountToNativeFct:        mysql.ErrWrongParamcountToNativeFct,
		mysql.ErrDivisionByZero:                    mysql.ErrDivisionByZero,
		mysql.ErrSpDoesNotExist:                    mysql.ErrSpDoesNotExist,
		mysql.ErrZlibZData:                         mysql.ErrZlibZData,
		mysql.ErrZlibZBuf:                          mysql.ErrZlibZBuf,
		mysql.ErrWrongArguments:                    mysql.ErrWrongArguments,
		mysql.ErrUnknownCharacterSet:               mysql.ErrUnknownCharacterSet,
		mysql.ErrInvalidDefault:                    mysql.ErrInvalidDefault,
		mysql.ErrWarnDeprecatedSyntaxNoReplacement: mysql.ErrWarnDeprecatedSyntaxNoReplacement,
		mysql.ErrOperandColumns:                    mysql.ErrOperandColumns,
		mysql.ErrCutValueGroupConcat:               mysql.ErrCutValueGroupConcat,
		mysql.ErrRegexp:                            mysql.ErrRegexp,
		mysql.ErrWarnAllowedPacketOverflowed:       mysql.ErrWarnAllowedPacketOverflowed,
		mysql.WarnOptionIgnored:                    mysql.WarnOptionIgnored,
		mysql.ErrTruncatedWrongValue:               mysql.ErrTruncatedWrongValue,
	}
	terror.ErrClassToMySQLCodes[terror.ClassExpression] = expressionMySQLErrCodes
}

// handleInvalidTimeError reports error or warning depend on the context.
func handleInvalidTimeError(ctx sessionctx.Context, err error) error {
	if err == nil || !(terror.ErrorEqual(err, types.ErrInvalidTimeFormat) || types.ErrIncorrectDatetimeValue.Equal(err) || types.ErrTruncatedWrongValue.Equal(err)) {
		return err
	}
	sc := ctx.GetSessionVars().StmtCtx
	if ctx.GetSessionVars().StrictSQLMode && (sc.InInsertStmt || sc.InUpdateOrDeleteStmt) {
		return err
	}
	sc.AppendWarning(err)
	return nil
}

// handleDivisionByZeroError reports error or warning depend on the context.
func handleDivisionByZeroError(ctx sessionctx.Context) error {
	sc := ctx.GetSessionVars().StmtCtx
	if sc.InInsertStmt || sc.InUpdateOrDeleteStmt {
		if !ctx.GetSessionVars().SQLMode.HasErrorForDivisionByZeroMode() {
			return nil
		}
		if ctx.GetSessionVars().StrictSQLMode && !sc.DividedByZeroAsWarning {
			return ErrDivisionByZero
		}
	}
	sc.AppendWarning(ErrDivisionByZero)
	return nil
}
