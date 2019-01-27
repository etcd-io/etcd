// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

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

package expression

import (
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/opcode"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tipb/go-tipb"
)

// baseBuiltinFunc will be contained in every struct that implement builtinFunc interface.
type baseBuiltinFunc struct {
	args   []Expression
	ctx    sessionctx.Context
	tp     *types.FieldType
	pbCode tipb.ScalarFuncSig
}

func (b *baseBuiltinFunc) PbCode() tipb.ScalarFuncSig {
	return b.pbCode
}

func (b *baseBuiltinFunc) setPbCode(c tipb.ScalarFuncSig) {
	b.pbCode = c
}

func newBaseBuiltinFunc(ctx sessionctx.Context, args []Expression) baseBuiltinFunc {
	if ctx == nil {
		panic("ctx should not be nil")
	}
	return baseBuiltinFunc{
		args: args,
		ctx:  ctx,
		tp:   types.NewFieldType(mysql.TypeUnspecified),
	}
}

// newBaseBuiltinFuncWithTp creates a built-in function signature with specified types of arguments and the return type of the function.
// argTps indicates the types of the args, retType indicates the return type of the built-in function.
// Every built-in function needs determined argTps and retType when we create it.
func newBaseBuiltinFuncWithTp(ctx sessionctx.Context, args []Expression, retType types.EvalType, argTps ...types.EvalType) (bf baseBuiltinFunc) {
	if len(args) != len(argTps) {
		panic("unexpected length of args and argTps")
	}
	if ctx == nil {
		panic("ctx should not be nil")
	}
	for i := range args {
		switch argTps[i] {
		case types.ETInt:
			args[i] = WrapWithCastAsInt(ctx, args[i])
		case types.ETReal:
			args[i] = WrapWithCastAsReal(ctx, args[i])
		case types.ETDecimal:
			args[i] = WrapWithCastAsDecimal(ctx, args[i])
		case types.ETString:
			args[i] = WrapWithCastAsString(ctx, args[i])
		case types.ETDatetime:
			args[i] = WrapWithCastAsTime(ctx, args[i], types.NewFieldType(mysql.TypeDatetime))
		case types.ETTimestamp:
			args[i] = WrapWithCastAsTime(ctx, args[i], types.NewFieldType(mysql.TypeTimestamp))
		case types.ETDuration:
			args[i] = WrapWithCastAsDuration(ctx, args[i])
		case types.ETJson:
			args[i] = WrapWithCastAsJSON(ctx, args[i])
		}
	}
	var fieldType *types.FieldType
	switch retType {
	case types.ETInt:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeLonglong,
			Flen:    mysql.MaxIntWidth,
			Decimal: 0,
			Flag:    mysql.BinaryFlag,
		}
	case types.ETReal:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeDouble,
			Flen:    mysql.MaxRealWidth,
			Decimal: types.UnspecifiedLength,
			Flag:    mysql.BinaryFlag,
		}
	case types.ETDecimal:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeNewDecimal,
			Flen:    11,
			Decimal: 0,
			Flag:    mysql.BinaryFlag,
		}
	case types.ETString:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeVarString,
			Flen:    0,
			Decimal: types.UnspecifiedLength,
		}
	case types.ETDatetime:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeDatetime,
			Flen:    mysql.MaxDatetimeWidthWithFsp,
			Decimal: types.MaxFsp,
			Flag:    mysql.BinaryFlag,
		}
	case types.ETTimestamp:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeTimestamp,
			Flen:    mysql.MaxDatetimeWidthWithFsp,
			Decimal: types.MaxFsp,
			Flag:    mysql.BinaryFlag,
		}
	case types.ETDuration:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeDuration,
			Flen:    mysql.MaxDurationWidthWithFsp,
			Decimal: types.MaxFsp,
			Flag:    mysql.BinaryFlag,
		}
	case types.ETJson:
		fieldType = &types.FieldType{
			Tp:      mysql.TypeJSON,
			Flen:    mysql.MaxBlobWidth,
			Decimal: 0,
			Charset: mysql.DefaultCharset,
			Collate: mysql.DefaultCollationName,
			Flag:    mysql.BinaryFlag,
		}
	}
	if mysql.HasBinaryFlag(fieldType.Flag) && fieldType.Tp != mysql.TypeJSON {
		fieldType.Charset, fieldType.Collate = charset.CharsetBin, charset.CollationBin
	} else {
		fieldType.Charset, fieldType.Collate = charset.GetDefaultCharsetAndCollate()
	}
	return baseBuiltinFunc{
		args: args,
		ctx:  ctx,
		tp:   fieldType,
	}
}

func (b *baseBuiltinFunc) getArgs() []Expression {
	return b.args
}

func (b *baseBuiltinFunc) evalInt(row chunk.Row) (int64, bool, error) {
	panic("baseBuiltinFunc.evalInt() should never be called.")
}

func (b *baseBuiltinFunc) evalReal(row chunk.Row) (float64, bool, error) {
	panic("baseBuiltinFunc.evalReal() should never be called.")
}

func (b *baseBuiltinFunc) evalString(row chunk.Row) (string, bool, error) {
	panic("baseBuiltinFunc.evalString() should never be called.")
}

func (b *baseBuiltinFunc) evalDecimal(row chunk.Row) (*types.MyDecimal, bool, error) {
	panic("baseBuiltinFunc.evalDecimal() should never be called.")
}

func (b *baseBuiltinFunc) evalTime(row chunk.Row) (types.Time, bool, error) {
	panic("baseBuiltinFunc.evalTime() should never be called.")
}

func (b *baseBuiltinFunc) evalDuration(row chunk.Row) (types.Duration, bool, error) {
	panic("baseBuiltinFunc.evalDuration() should never be called.")
}

func (b *baseBuiltinFunc) evalJSON(row chunk.Row) (json.BinaryJSON, bool, error) {
	panic("baseBuiltinFunc.evalJSON() should never be called.")
}

func (b *baseBuiltinFunc) getRetTp() *types.FieldType {
	switch b.tp.EvalType() {
	case types.ETString:
		if b.tp.Flen >= mysql.MaxBlobWidth {
			b.tp.Tp = mysql.TypeLongBlob
		} else if b.tp.Flen >= 65536 {
			b.tp.Tp = mysql.TypeMediumBlob
		}
		if len(b.tp.Charset) <= 0 {
			b.tp.Charset, b.tp.Collate = charset.GetDefaultCharsetAndCollate()
		}
	}
	return b.tp
}

func (b *baseBuiltinFunc) equal(fun builtinFunc) bool {
	funArgs := fun.getArgs()
	if len(funArgs) != len(b.args) {
		return false
	}
	for i := range b.args {
		if !b.args[i].Equal(b.ctx, funArgs[i]) {
			return false
		}
	}
	return true
}

func (b *baseBuiltinFunc) getCtx() sessionctx.Context {
	return b.ctx
}

func (b *baseBuiltinFunc) cloneFrom(from *baseBuiltinFunc) {
	b.args = make([]Expression, 0, len(b.args))
	for _, arg := range from.args {
		b.args = append(b.args, arg.Clone())
	}
	b.ctx = from.ctx
	b.tp = from.tp
	b.pbCode = from.pbCode
}

func (b *baseBuiltinFunc) Clone() builtinFunc {
	panic("you should not call this method.")
}

// baseBuiltinCastFunc will be contained in every struct that implement cast builtinFunc.
type baseBuiltinCastFunc struct {
	baseBuiltinFunc

	// inUnion indicates whether cast is in union context.
	inUnion bool
}

func (b *baseBuiltinCastFunc) cloneFrom(from *baseBuiltinCastFunc) {
	b.baseBuiltinFunc.cloneFrom(&from.baseBuiltinFunc)
	b.inUnion = from.inUnion
}

func newBaseBuiltinCastFunc(builtinFunc baseBuiltinFunc, inUnion bool) baseBuiltinCastFunc {
	return baseBuiltinCastFunc{
		baseBuiltinFunc: builtinFunc,
		inUnion:         inUnion,
	}
}

// builtinFunc stands for a particular function signature.
type builtinFunc interface {
	// evalInt evaluates int result of builtinFunc by given row.
	evalInt(row chunk.Row) (val int64, isNull bool, err error)
	// evalReal evaluates real representation of builtinFunc by given row.
	evalReal(row chunk.Row) (val float64, isNull bool, err error)
	// evalString evaluates string representation of builtinFunc by given row.
	evalString(row chunk.Row) (val string, isNull bool, err error)
	// evalDecimal evaluates decimal representation of builtinFunc by given row.
	evalDecimal(row chunk.Row) (val *types.MyDecimal, isNull bool, err error)
	// evalTime evaluates DATE/DATETIME/TIMESTAMP representation of builtinFunc by given row.
	evalTime(row chunk.Row) (val types.Time, isNull bool, err error)
	// evalDuration evaluates duration representation of builtinFunc by given row.
	evalDuration(row chunk.Row) (val types.Duration, isNull bool, err error)
	// evalJSON evaluates JSON representation of builtinFunc by given row.
	evalJSON(row chunk.Row) (val json.BinaryJSON, isNull bool, err error)
	// getArgs returns the arguments expressions.
	getArgs() []Expression
	// equal check if this function equals to another function.
	equal(builtinFunc) bool
	// getCtx returns this function's context.
	getCtx() sessionctx.Context
	// getRetTp returns the return type of the built-in function.
	getRetTp() *types.FieldType
	// setPbCode sets pbCode for signature.
	setPbCode(tipb.ScalarFuncSig)
	// PbCode returns PbCode of this signature.
	PbCode() tipb.ScalarFuncSig
	// Clone returns a copy of itself.
	Clone() builtinFunc
}

// baseFunctionClass will be contained in every struct that implement functionClass interface.
type baseFunctionClass struct {
	funcName string
	minArgs  int
	maxArgs  int
}

func (b *baseFunctionClass) verifyArgs(args []Expression) error {
	l := len(args)
	if l < b.minArgs || (b.maxArgs != -1 && l > b.maxArgs) {
		return ErrIncorrectParameterCount.GenWithStackByArgs(b.funcName)
	}
	return nil
}

// functionClass is the interface for a function which may contains multiple functions.
type functionClass interface {
	// getFunction gets a function signature by the types and the counts of given arguments.
	getFunction(ctx sessionctx.Context, args []Expression) (builtinFunc, error)
}

// funcs holds all registered builtin functions.
var funcs = map[string]functionClass{
	// common functions
	ast.Coalesce: &coalesceFunctionClass{baseFunctionClass{ast.Coalesce, 1, -1}},
	ast.IsNull:   &isNullFunctionClass{baseFunctionClass{ast.IsNull, 1, 1}},
	ast.Greatest: &greatestFunctionClass{baseFunctionClass{ast.Greatest, 2, -1}},
	ast.Least:    &leastFunctionClass{baseFunctionClass{ast.Least, 2, -1}},
	ast.Interval: &intervalFunctionClass{baseFunctionClass{ast.Interval, 2, -1}},

	// math functions
	ast.Abs:      &absFunctionClass{baseFunctionClass{ast.Abs, 1, 1}},
	ast.Acos:     &acosFunctionClass{baseFunctionClass{ast.Acos, 1, 1}},
	ast.Asin:     &asinFunctionClass{baseFunctionClass{ast.Asin, 1, 1}},
	ast.Atan:     &atanFunctionClass{baseFunctionClass{ast.Atan, 1, 2}},
	ast.Atan2:    &atanFunctionClass{baseFunctionClass{ast.Atan2, 2, 2}},
	ast.Ceil:     &ceilFunctionClass{baseFunctionClass{ast.Ceil, 1, 1}},
	ast.Ceiling:  &ceilFunctionClass{baseFunctionClass{ast.Ceiling, 1, 1}},
	ast.Conv:     &convFunctionClass{baseFunctionClass{ast.Conv, 3, 3}},
	ast.Cos:      &cosFunctionClass{baseFunctionClass{ast.Cos, 1, 1}},
	ast.Cot:      &cotFunctionClass{baseFunctionClass{ast.Cot, 1, 1}},
	ast.CRC32:    &crc32FunctionClass{baseFunctionClass{ast.CRC32, 1, 1}},
	ast.Degrees:  &degreesFunctionClass{baseFunctionClass{ast.Degrees, 1, 1}},
	ast.Exp:      &expFunctionClass{baseFunctionClass{ast.Exp, 1, 1}},
	ast.Floor:    &floorFunctionClass{baseFunctionClass{ast.Floor, 1, 1}},
	ast.Ln:       &logFunctionClass{baseFunctionClass{ast.Ln, 1, 1}},
	ast.Log:      &logFunctionClass{baseFunctionClass{ast.Log, 1, 2}},
	ast.Log2:     &log2FunctionClass{baseFunctionClass{ast.Log2, 1, 1}},
	ast.Log10:    &log10FunctionClass{baseFunctionClass{ast.Log10, 1, 1}},
	ast.PI:       &piFunctionClass{baseFunctionClass{ast.PI, 0, 0}},
	ast.Pow:      &powFunctionClass{baseFunctionClass{ast.Pow, 2, 2}},
	ast.Power:    &powFunctionClass{baseFunctionClass{ast.Power, 2, 2}},
	ast.Radians:  &radiansFunctionClass{baseFunctionClass{ast.Radians, 1, 1}},
	ast.Rand:     &randFunctionClass{baseFunctionClass{ast.Rand, 0, 1}},
	ast.Round:    &roundFunctionClass{baseFunctionClass{ast.Round, 1, 2}},
	ast.Sign:     &signFunctionClass{baseFunctionClass{ast.Sign, 1, 1}},
	ast.Sin:      &sinFunctionClass{baseFunctionClass{ast.Sin, 1, 1}},
	ast.Sqrt:     &sqrtFunctionClass{baseFunctionClass{ast.Sqrt, 1, 1}},
	ast.Tan:      &tanFunctionClass{baseFunctionClass{ast.Tan, 1, 1}},
	ast.Truncate: &truncateFunctionClass{baseFunctionClass{ast.Truncate, 2, 2}},

	// time functions
	ast.AddDate:          &addDateFunctionClass{baseFunctionClass{ast.AddDate, 3, 3}},
	ast.DateAdd:          &addDateFunctionClass{baseFunctionClass{ast.DateAdd, 3, 3}},
	ast.SubDate:          &subDateFunctionClass{baseFunctionClass{ast.SubDate, 3, 3}},
	ast.DateSub:          &subDateFunctionClass{baseFunctionClass{ast.DateSub, 3, 3}},
	ast.AddTime:          &addTimeFunctionClass{baseFunctionClass{ast.AddTime, 2, 2}},
	ast.ConvertTz:        &convertTzFunctionClass{baseFunctionClass{ast.ConvertTz, 3, 3}},
	ast.Curdate:          &currentDateFunctionClass{baseFunctionClass{ast.Curdate, 0, 0}},
	ast.CurrentDate:      &currentDateFunctionClass{baseFunctionClass{ast.CurrentDate, 0, 0}},
	ast.CurrentTime:      &currentTimeFunctionClass{baseFunctionClass{ast.CurrentTime, 0, 1}},
	ast.CurrentTimestamp: &nowFunctionClass{baseFunctionClass{ast.CurrentTimestamp, 0, 1}},
	ast.Curtime:          &currentTimeFunctionClass{baseFunctionClass{ast.Curtime, 0, 1}},
	ast.Date:             &dateFunctionClass{baseFunctionClass{ast.Date, 1, 1}},
	ast.DateLiteral:      &dateLiteralFunctionClass{baseFunctionClass{ast.DateLiteral, 1, 1}},
	ast.DateFormat:       &dateFormatFunctionClass{baseFunctionClass{ast.DateFormat, 2, 2}},
	ast.DateDiff:         &dateDiffFunctionClass{baseFunctionClass{ast.DateDiff, 2, 2}},
	ast.Day:              &dayOfMonthFunctionClass{baseFunctionClass{ast.Day, 1, 1}},
	ast.DayName:          &dayNameFunctionClass{baseFunctionClass{ast.DayName, 1, 1}},
	ast.DayOfMonth:       &dayOfMonthFunctionClass{baseFunctionClass{ast.DayOfMonth, 1, 1}},
	ast.DayOfWeek:        &dayOfWeekFunctionClass{baseFunctionClass{ast.DayOfWeek, 1, 1}},
	ast.DayOfYear:        &dayOfYearFunctionClass{baseFunctionClass{ast.DayOfYear, 1, 1}},
	ast.Extract:          &extractFunctionClass{baseFunctionClass{ast.Extract, 2, 2}},
	ast.FromDays:         &fromDaysFunctionClass{baseFunctionClass{ast.FromDays, 1, 1}},
	ast.FromUnixTime:     &fromUnixTimeFunctionClass{baseFunctionClass{ast.FromUnixTime, 1, 2}},
	ast.GetFormat:        &getFormatFunctionClass{baseFunctionClass{ast.GetFormat, 2, 2}},
	ast.Hour:             &hourFunctionClass{baseFunctionClass{ast.Hour, 1, 1}},
	ast.LocalTime:        &nowFunctionClass{baseFunctionClass{ast.LocalTime, 0, 1}},
	ast.LocalTimestamp:   &nowFunctionClass{baseFunctionClass{ast.LocalTimestamp, 0, 1}},
	ast.MakeDate:         &makeDateFunctionClass{baseFunctionClass{ast.MakeDate, 2, 2}},
	ast.MakeTime:         &makeTimeFunctionClass{baseFunctionClass{ast.MakeTime, 3, 3}},
	ast.MicroSecond:      &microSecondFunctionClass{baseFunctionClass{ast.MicroSecond, 1, 1}},
	ast.Minute:           &minuteFunctionClass{baseFunctionClass{ast.Minute, 1, 1}},
	ast.Month:            &monthFunctionClass{baseFunctionClass{ast.Month, 1, 1}},
	ast.MonthName:        &monthNameFunctionClass{baseFunctionClass{ast.MonthName, 1, 1}},
	ast.Now:              &nowFunctionClass{baseFunctionClass{ast.Now, 0, 1}},
	ast.PeriodAdd:        &periodAddFunctionClass{baseFunctionClass{ast.PeriodAdd, 2, 2}},
	ast.PeriodDiff:       &periodDiffFunctionClass{baseFunctionClass{ast.PeriodDiff, 2, 2}},
	ast.Quarter:          &quarterFunctionClass{baseFunctionClass{ast.Quarter, 1, 1}},
	ast.SecToTime:        &secToTimeFunctionClass{baseFunctionClass{ast.SecToTime, 1, 1}},
	ast.Second:           &secondFunctionClass{baseFunctionClass{ast.Second, 1, 1}},
	ast.StrToDate:        &strToDateFunctionClass{baseFunctionClass{ast.StrToDate, 2, 2}},
	ast.SubTime:          &subTimeFunctionClass{baseFunctionClass{ast.SubTime, 2, 2}},
	ast.Sysdate:          &sysDateFunctionClass{baseFunctionClass{ast.Sysdate, 0, 1}},
	ast.Time:             &timeFunctionClass{baseFunctionClass{ast.Time, 1, 1}},
	ast.TimeLiteral:      &timeLiteralFunctionClass{baseFunctionClass{ast.TimeLiteral, 1, 1}},
	ast.TimeFormat:       &timeFormatFunctionClass{baseFunctionClass{ast.TimeFormat, 2, 2}},
	ast.TimeToSec:        &timeToSecFunctionClass{baseFunctionClass{ast.TimeToSec, 1, 1}},
	ast.TimeDiff:         &timeDiffFunctionClass{baseFunctionClass{ast.TimeDiff, 2, 2}},
	ast.Timestamp:        &timestampFunctionClass{baseFunctionClass{ast.Timestamp, 1, 2}},
	ast.TimestampLiteral: &timestampLiteralFunctionClass{baseFunctionClass{ast.TimestampLiteral, 1, 2}},
	ast.TimestampAdd:     &timestampAddFunctionClass{baseFunctionClass{ast.TimestampAdd, 3, 3}},
	ast.TimestampDiff:    &timestampDiffFunctionClass{baseFunctionClass{ast.TimestampDiff, 3, 3}},
	ast.ToDays:           &toDaysFunctionClass{baseFunctionClass{ast.ToDays, 1, 1}},
	ast.ToSeconds:        &toSecondsFunctionClass{baseFunctionClass{ast.ToSeconds, 1, 1}},
	ast.UnixTimestamp:    &unixTimestampFunctionClass{baseFunctionClass{ast.UnixTimestamp, 0, 1}},
	ast.UTCDate:          &utcDateFunctionClass{baseFunctionClass{ast.UTCDate, 0, 0}},
	ast.UTCTime:          &utcTimeFunctionClass{baseFunctionClass{ast.UTCTime, 0, 1}},
	ast.UTCTimestamp:     &utcTimestampFunctionClass{baseFunctionClass{ast.UTCTimestamp, 0, 1}},
	ast.Week:             &weekFunctionClass{baseFunctionClass{ast.Week, 1, 2}},
	ast.Weekday:          &weekDayFunctionClass{baseFunctionClass{ast.Weekday, 1, 1}},
	ast.WeekOfYear:       &weekOfYearFunctionClass{baseFunctionClass{ast.WeekOfYear, 1, 1}},
	ast.Year:             &yearFunctionClass{baseFunctionClass{ast.Year, 1, 1}},
	ast.YearWeek:         &yearWeekFunctionClass{baseFunctionClass{ast.YearWeek, 1, 2}},
	ast.LastDay:          &lastDayFunctionClass{baseFunctionClass{ast.LastDay, 1, 1}},

	// string functions
	ast.ASCII:           &asciiFunctionClass{baseFunctionClass{ast.ASCII, 1, 1}},
	ast.Bin:             &binFunctionClass{baseFunctionClass{ast.Bin, 1, 1}},
	ast.Concat:          &concatFunctionClass{baseFunctionClass{ast.Concat, 1, -1}},
	ast.ConcatWS:        &concatWSFunctionClass{baseFunctionClass{ast.ConcatWS, 2, -1}},
	ast.Convert:         &convertFunctionClass{baseFunctionClass{ast.Convert, 2, 2}},
	ast.Elt:             &eltFunctionClass{baseFunctionClass{ast.Elt, 2, -1}},
	ast.ExportSet:       &exportSetFunctionClass{baseFunctionClass{ast.ExportSet, 3, 5}},
	ast.Field:           &fieldFunctionClass{baseFunctionClass{ast.Field, 2, -1}},
	ast.Format:          &formatFunctionClass{baseFunctionClass{ast.Format, 2, 3}},
	ast.FromBase64:      &fromBase64FunctionClass{baseFunctionClass{ast.FromBase64, 1, 1}},
	ast.InsertFunc:      &insertFunctionClass{baseFunctionClass{ast.InsertFunc, 4, 4}},
	ast.Instr:           &instrFunctionClass{baseFunctionClass{ast.Instr, 2, 2}},
	ast.Lcase:           &lowerFunctionClass{baseFunctionClass{ast.Lcase, 1, 1}},
	ast.Left:            &leftFunctionClass{baseFunctionClass{ast.Left, 2, 2}},
	ast.Right:           &rightFunctionClass{baseFunctionClass{ast.Right, 2, 2}},
	ast.Length:          &lengthFunctionClass{baseFunctionClass{ast.Length, 1, 1}},
	ast.LoadFile:        &loadFileFunctionClass{baseFunctionClass{ast.LoadFile, 1, 1}},
	ast.Locate:          &locateFunctionClass{baseFunctionClass{ast.Locate, 2, 3}},
	ast.Lower:           &lowerFunctionClass{baseFunctionClass{ast.Lower, 1, 1}},
	ast.Lpad:            &lpadFunctionClass{baseFunctionClass{ast.Lpad, 3, 3}},
	ast.LTrim:           &lTrimFunctionClass{baseFunctionClass{ast.LTrim, 1, 1}},
	ast.Mid:             &substringFunctionClass{baseFunctionClass{ast.Mid, 3, 3}},
	ast.MakeSet:         &makeSetFunctionClass{baseFunctionClass{ast.MakeSet, 2, -1}},
	ast.Oct:             &octFunctionClass{baseFunctionClass{ast.Oct, 1, 1}},
	ast.Ord:             &ordFunctionClass{baseFunctionClass{ast.Ord, 1, 1}},
	ast.Position:        &locateFunctionClass{baseFunctionClass{ast.Position, 2, 2}},
	ast.Quote:           &quoteFunctionClass{baseFunctionClass{ast.Quote, 1, 1}},
	ast.Repeat:          &repeatFunctionClass{baseFunctionClass{ast.Repeat, 2, 2}},
	ast.Replace:         &replaceFunctionClass{baseFunctionClass{ast.Replace, 3, 3}},
	ast.Reverse:         &reverseFunctionClass{baseFunctionClass{ast.Reverse, 1, 1}},
	ast.RTrim:           &rTrimFunctionClass{baseFunctionClass{ast.RTrim, 1, 1}},
	ast.Space:           &spaceFunctionClass{baseFunctionClass{ast.Space, 1, 1}},
	ast.Strcmp:          &strcmpFunctionClass{baseFunctionClass{ast.Strcmp, 2, 2}},
	ast.Substring:       &substringFunctionClass{baseFunctionClass{ast.Substring, 2, 3}},
	ast.Substr:          &substringFunctionClass{baseFunctionClass{ast.Substr, 2, 3}},
	ast.SubstringIndex:  &substringIndexFunctionClass{baseFunctionClass{ast.SubstringIndex, 3, 3}},
	ast.ToBase64:        &toBase64FunctionClass{baseFunctionClass{ast.ToBase64, 1, 1}},
	ast.Trim:            &trimFunctionClass{baseFunctionClass{ast.Trim, 1, 3}},
	ast.Upper:           &upperFunctionClass{baseFunctionClass{ast.Upper, 1, 1}},
	ast.Ucase:           &upperFunctionClass{baseFunctionClass{ast.Ucase, 1, 1}},
	ast.Hex:             &hexFunctionClass{baseFunctionClass{ast.Hex, 1, 1}},
	ast.Unhex:           &unhexFunctionClass{baseFunctionClass{ast.Unhex, 1, 1}},
	ast.Rpad:            &rpadFunctionClass{baseFunctionClass{ast.Rpad, 3, 3}},
	ast.BitLength:       &bitLengthFunctionClass{baseFunctionClass{ast.BitLength, 1, 1}},
	ast.CharFunc:        &charFunctionClass{baseFunctionClass{ast.CharFunc, 2, -1}},
	ast.CharLength:      &charLengthFunctionClass{baseFunctionClass{ast.CharLength, 1, 1}},
	ast.CharacterLength: &charLengthFunctionClass{baseFunctionClass{ast.CharacterLength, 1, 1}},
	ast.FindInSet:       &findInSetFunctionClass{baseFunctionClass{ast.FindInSet, 2, 2}},

	// information functions
	ast.ConnectionID: &connectionIDFunctionClass{baseFunctionClass{ast.ConnectionID, 0, 0}},
	ast.CurrentUser:  &currentUserFunctionClass{baseFunctionClass{ast.CurrentUser, 0, 0}},
	ast.Database:     &databaseFunctionClass{baseFunctionClass{ast.Database, 0, 0}},
	// This function is a synonym for DATABASE().
	// See http://dev.mysql.com/doc/refman/5.7/en/information-functions.html#function_schema
	ast.Schema:       &databaseFunctionClass{baseFunctionClass{ast.Schema, 0, 0}},
	ast.FoundRows:    &foundRowsFunctionClass{baseFunctionClass{ast.FoundRows, 0, 0}},
	ast.LastInsertId: &lastInsertIDFunctionClass{baseFunctionClass{ast.LastInsertId, 0, 1}},
	ast.User:         &userFunctionClass{baseFunctionClass{ast.User, 0, 0}},
	ast.Version:      &versionFunctionClass{baseFunctionClass{ast.Version, 0, 0}},
	ast.Benchmark:    &benchmarkFunctionClass{baseFunctionClass{ast.Benchmark, 2, 2}},
	ast.Charset:      &charsetFunctionClass{baseFunctionClass{ast.Charset, 1, 1}},
	ast.Coercibility: &coercibilityFunctionClass{baseFunctionClass{ast.Coercibility, 1, 1}},
	ast.Collation:    &collationFunctionClass{baseFunctionClass{ast.Collation, 1, 1}},
	ast.RowCount:     &rowCountFunctionClass{baseFunctionClass{ast.RowCount, 0, 0}},
	ast.SessionUser:  &userFunctionClass{baseFunctionClass{ast.SessionUser, 0, 0}},
	ast.SystemUser:   &userFunctionClass{baseFunctionClass{ast.SystemUser, 0, 0}},
	// This function is used to show tidb-server version info.
	ast.TiDBVersion:    &tidbVersionFunctionClass{baseFunctionClass{ast.TiDBVersion, 0, 0}},
	ast.TiDBIsDDLOwner: &tidbIsDDLOwnerFunctionClass{baseFunctionClass{ast.TiDBIsDDLOwner, 0, 0}},

	// control functions
	ast.If:     &ifFunctionClass{baseFunctionClass{ast.If, 3, 3}},
	ast.Ifnull: &ifNullFunctionClass{baseFunctionClass{ast.Ifnull, 2, 2}},

	// miscellaneous functions
	ast.Sleep:           &sleepFunctionClass{baseFunctionClass{ast.Sleep, 1, 1}},
	ast.AnyValue:        &anyValueFunctionClass{baseFunctionClass{ast.AnyValue, 1, 1}},
	ast.DefaultFunc:     &defaultFunctionClass{baseFunctionClass{ast.DefaultFunc, 1, 1}},
	ast.InetAton:        &inetAtonFunctionClass{baseFunctionClass{ast.InetAton, 1, 1}},
	ast.InetNtoa:        &inetNtoaFunctionClass{baseFunctionClass{ast.InetNtoa, 1, 1}},
	ast.Inet6Aton:       &inet6AtonFunctionClass{baseFunctionClass{ast.Inet6Aton, 1, 1}},
	ast.Inet6Ntoa:       &inet6NtoaFunctionClass{baseFunctionClass{ast.Inet6Ntoa, 1, 1}},
	ast.IsFreeLock:      &isFreeLockFunctionClass{baseFunctionClass{ast.IsFreeLock, 1, 1}},
	ast.IsIPv4:          &isIPv4FunctionClass{baseFunctionClass{ast.IsIPv4, 1, 1}},
	ast.IsIPv4Compat:    &isIPv4CompatFunctionClass{baseFunctionClass{ast.IsIPv4Compat, 1, 1}},
	ast.IsIPv4Mapped:    &isIPv4MappedFunctionClass{baseFunctionClass{ast.IsIPv4Mapped, 1, 1}},
	ast.IsIPv6:          &isIPv6FunctionClass{baseFunctionClass{ast.IsIPv6, 1, 1}},
	ast.IsUsedLock:      &isUsedLockFunctionClass{baseFunctionClass{ast.IsUsedLock, 1, 1}},
	ast.MasterPosWait:   &masterPosWaitFunctionClass{baseFunctionClass{ast.MasterPosWait, 2, 4}},
	ast.NameConst:       &nameConstFunctionClass{baseFunctionClass{ast.NameConst, 2, 2}},
	ast.ReleaseAllLocks: &releaseAllLocksFunctionClass{baseFunctionClass{ast.ReleaseAllLocks, 0, 0}},
	ast.UUID:            &uuidFunctionClass{baseFunctionClass{ast.UUID, 0, 0}},
	ast.UUIDShort:       &uuidShortFunctionClass{baseFunctionClass{ast.UUIDShort, 0, 0}},

	// get_lock() and release_lock() are parsed but do nothing.
	// It is used for preventing error in Ruby's activerecord migrations.
	ast.GetLock:     &lockFunctionClass{baseFunctionClass{ast.GetLock, 2, 2}},
	ast.ReleaseLock: &releaseLockFunctionClass{baseFunctionClass{ast.ReleaseLock, 1, 1}},

	ast.LogicAnd:   &logicAndFunctionClass{baseFunctionClass{ast.LogicAnd, 2, 2}},
	ast.LogicOr:    &logicOrFunctionClass{baseFunctionClass{ast.LogicOr, 2, 2}},
	ast.LogicXor:   &logicXorFunctionClass{baseFunctionClass{ast.LogicXor, 2, 2}},
	ast.GE:         &compareFunctionClass{baseFunctionClass{ast.GE, 2, 2}, opcode.GE},
	ast.LE:         &compareFunctionClass{baseFunctionClass{ast.LE, 2, 2}, opcode.LE},
	ast.EQ:         &compareFunctionClass{baseFunctionClass{ast.EQ, 2, 2}, opcode.EQ},
	ast.NE:         &compareFunctionClass{baseFunctionClass{ast.NE, 2, 2}, opcode.NE},
	ast.LT:         &compareFunctionClass{baseFunctionClass{ast.LT, 2, 2}, opcode.LT},
	ast.GT:         &compareFunctionClass{baseFunctionClass{ast.GT, 2, 2}, opcode.GT},
	ast.NullEQ:     &compareFunctionClass{baseFunctionClass{ast.NullEQ, 2, 2}, opcode.NullEQ},
	ast.Plus:       &arithmeticPlusFunctionClass{baseFunctionClass{ast.Plus, 2, 2}},
	ast.Minus:      &arithmeticMinusFunctionClass{baseFunctionClass{ast.Minus, 2, 2}},
	ast.Mod:        &arithmeticModFunctionClass{baseFunctionClass{ast.Mod, 2, 2}},
	ast.Div:        &arithmeticDivideFunctionClass{baseFunctionClass{ast.Div, 2, 2}},
	ast.Mul:        &arithmeticMultiplyFunctionClass{baseFunctionClass{ast.Mul, 2, 2}},
	ast.IntDiv:     &arithmeticIntDivideFunctionClass{baseFunctionClass{ast.IntDiv, 2, 2}},
	ast.BitNeg:     &bitNegFunctionClass{baseFunctionClass{ast.BitNeg, 1, 1}},
	ast.And:        &bitAndFunctionClass{baseFunctionClass{ast.And, 2, 2}},
	ast.LeftShift:  &leftShiftFunctionClass{baseFunctionClass{ast.LeftShift, 2, 2}},
	ast.RightShift: &rightShiftFunctionClass{baseFunctionClass{ast.RightShift, 2, 2}},
	ast.UnaryNot:   &unaryNotFunctionClass{baseFunctionClass{ast.UnaryNot, 1, 1}},
	ast.Or:         &bitOrFunctionClass{baseFunctionClass{ast.Or, 2, 2}},
	ast.Xor:        &bitXorFunctionClass{baseFunctionClass{ast.Xor, 2, 2}},
	ast.UnaryMinus: &unaryMinusFunctionClass{baseFunctionClass{ast.UnaryMinus, 1, 1}},
	ast.In:         &inFunctionClass{baseFunctionClass{ast.In, 2, -1}},
	ast.IsTruth:    &isTrueOrFalseFunctionClass{baseFunctionClass{ast.IsTruth, 1, 1}, opcode.IsTruth},
	ast.IsFalsity:  &isTrueOrFalseFunctionClass{baseFunctionClass{ast.IsFalsity, 1, 1}, opcode.IsFalsity},
	ast.Like:       &likeFunctionClass{baseFunctionClass{ast.Like, 3, 3}},
	ast.Regexp:     &regexpFunctionClass{baseFunctionClass{ast.Regexp, 2, 2}},
	ast.Case:       &caseWhenFunctionClass{baseFunctionClass{ast.Case, 1, -1}},
	ast.RowFunc:    &rowFunctionClass{baseFunctionClass{ast.RowFunc, 2, -1}},
	ast.SetVar:     &setVarFunctionClass{baseFunctionClass{ast.SetVar, 2, 2}},
	ast.GetVar:     &getVarFunctionClass{baseFunctionClass{ast.GetVar, 1, 1}},
	ast.BitCount:   &bitCountFunctionClass{baseFunctionClass{ast.BitCount, 1, 1}},
	ast.GetParam:   &getParamFunctionClass{baseFunctionClass{ast.GetParam, 1, 1}},

	// encryption and compression functions
	ast.AesDecrypt:               &aesDecryptFunctionClass{baseFunctionClass{ast.AesDecrypt, 2, 3}},
	ast.AesEncrypt:               &aesEncryptFunctionClass{baseFunctionClass{ast.AesEncrypt, 2, 3}},
	ast.Compress:                 &compressFunctionClass{baseFunctionClass{ast.Compress, 1, 1}},
	ast.Decode:                   &decodeFunctionClass{baseFunctionClass{ast.Decode, 2, 2}},
	ast.DesDecrypt:               &desDecryptFunctionClass{baseFunctionClass{ast.DesDecrypt, 1, 2}},
	ast.DesEncrypt:               &desEncryptFunctionClass{baseFunctionClass{ast.DesEncrypt, 1, 2}},
	ast.Encode:                   &encodeFunctionClass{baseFunctionClass{ast.Encode, 2, 2}},
	ast.Encrypt:                  &encryptFunctionClass{baseFunctionClass{ast.Encrypt, 1, 2}},
	ast.MD5:                      &md5FunctionClass{baseFunctionClass{ast.MD5, 1, 1}},
	ast.OldPassword:              &oldPasswordFunctionClass{baseFunctionClass{ast.OldPassword, 1, 1}},
	ast.PasswordFunc:             &passwordFunctionClass{baseFunctionClass{ast.PasswordFunc, 1, 1}},
	ast.RandomBytes:              &randomBytesFunctionClass{baseFunctionClass{ast.RandomBytes, 1, 1}},
	ast.SHA1:                     &sha1FunctionClass{baseFunctionClass{ast.SHA1, 1, 1}},
	ast.SHA:                      &sha1FunctionClass{baseFunctionClass{ast.SHA, 1, 1}},
	ast.SHA2:                     &sha2FunctionClass{baseFunctionClass{ast.SHA2, 2, 2}},
	ast.Uncompress:               &uncompressFunctionClass{baseFunctionClass{ast.Uncompress, 1, 1}},
	ast.UncompressedLength:       &uncompressedLengthFunctionClass{baseFunctionClass{ast.UncompressedLength, 1, 1}},
	ast.ValidatePasswordStrength: &validatePasswordStrengthFunctionClass{baseFunctionClass{ast.ValidatePasswordStrength, 1, 1}},

	// json functions
	ast.JSONType:          &jsonTypeFunctionClass{baseFunctionClass{ast.JSONType, 1, 1}},
	ast.JSONExtract:       &jsonExtractFunctionClass{baseFunctionClass{ast.JSONExtract, 2, -1}},
	ast.JSONUnquote:       &jsonUnquoteFunctionClass{baseFunctionClass{ast.JSONUnquote, 1, 1}},
	ast.JSONSet:           &jsonSetFunctionClass{baseFunctionClass{ast.JSONSet, 3, -1}},
	ast.JSONInsert:        &jsonInsertFunctionClass{baseFunctionClass{ast.JSONInsert, 3, -1}},
	ast.JSONReplace:       &jsonReplaceFunctionClass{baseFunctionClass{ast.JSONReplace, 3, -1}},
	ast.JSONRemove:        &jsonRemoveFunctionClass{baseFunctionClass{ast.JSONRemove, 2, -1}},
	ast.JSONMerge:         &jsonMergeFunctionClass{baseFunctionClass{ast.JSONMerge, 2, -1}},
	ast.JSONObject:        &jsonObjectFunctionClass{baseFunctionClass{ast.JSONObject, 0, -1}},
	ast.JSONArray:         &jsonArrayFunctionClass{baseFunctionClass{ast.JSONArray, 0, -1}},
	ast.JSONContains:      &jsonContainsFunctionClass{baseFunctionClass{ast.JSONContains, 2, 3}},
	ast.JSONContainsPath:  &jsonContainsPathFunctionClass{baseFunctionClass{ast.JSONContainsPath, 3, -1}},
	ast.JSONValid:         &jsonValidFunctionClass{baseFunctionClass{ast.JSONValid, 1, 1}},
	ast.JSONArrayAppend:   &jsonArrayAppendFunctionClass{baseFunctionClass{ast.JSONArrayAppend, 3, -1}},
	ast.JSONArrayInsert:   &jsonArrayInsertFunctionClass{baseFunctionClass{ast.JSONArrayInsert, 3, -1}},
	ast.JSONMergePatch:    &jsonMergePatchFunctionClass{baseFunctionClass{ast.JSONMergePatch, 2, -1}},
	ast.JSONMergePreserve: &jsonMergePreserveFunctionClass{baseFunctionClass{ast.JSONMergePreserve, 2, -1}},
	ast.JSONPretty:        &jsonPrettyFunctionClass{baseFunctionClass{ast.JSONPretty, 1, 1}},
	ast.JSONQuote:         &jsonQuoteFunctionClass{baseFunctionClass{ast.JSONQuote, 1, 1}},
	ast.JSONSearch:        &jsonSearchFunctionClass{baseFunctionClass{ast.JSONSearch, 3, -1}},
	ast.JSONStorageSize:   &jsonStorageSizeFunctionClass{baseFunctionClass{ast.JSONStorageSize, 1, 1}},
	ast.JSONDepth:         &jsonDepthFunctionClass{baseFunctionClass{ast.JSONDepth, 1, 1}},
	ast.JSONKeys:          &jsonKeysFunctionClass{baseFunctionClass{ast.JSONKeys, 1, 2}},
	ast.JSONLength:        &jsonLengthFunctionClass{baseFunctionClass{ast.JSONLength, 1, 2}},
}
