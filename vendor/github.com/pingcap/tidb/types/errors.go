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

package types

import (
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	parser_types "github.com/pingcap/parser/types"
)

var (
	// ErrDataTooLong is returned when converts a string value that is longer than field type length.
	ErrDataTooLong = terror.ClassTypes.New(codeDataTooLong, "Data Too Long")
	// ErrIllegalValueForType is returned when value of type is illegal.
	ErrIllegalValueForType = terror.ClassTypes.New(codeIllegalValueForType, mysql.MySQLErrName[mysql.ErrIllegalValueForType])
	// ErrTruncated is returned when data has been truncated during conversion.
	ErrTruncated = terror.ClassTypes.New(codeTruncated, "Data Truncated")
	// ErrTruncatedWrongVal is returned when data has been truncated during conversion.
	ErrTruncatedWrongVal = terror.ClassTypes.New(codeTruncatedWrongValue, msgTruncatedWrongVal)
	// ErrOverflow is returned when data is out of range for a field type.
	ErrOverflow = terror.ClassTypes.New(codeOverflow, msgOverflow)
	// ErrDivByZero is return when do division by 0.
	ErrDivByZero = terror.ClassTypes.New(codeDivByZero, "Division by 0")
	// ErrTooBigDisplayWidth is return when display width out of range for column.
	ErrTooBigDisplayWidth = terror.ClassTypes.New(codeTooBigDisplayWidth, "Too Big Display width")
	// ErrTooBigFieldLength is return when column length too big for column.
	ErrTooBigFieldLength = terror.ClassTypes.New(codeTooBigFieldLength, "Too Big Field length")
	// ErrTooBigSet is returned when too many strings for column.
	ErrTooBigSet = terror.ClassTypes.New(codeTooBigSet, "Too Big Set")
	// ErrTooBigScale is returned when type DECIMAL/NUMERIC scale is bigger than mysql.MaxDecimalScale.
	ErrTooBigScale = terror.ClassTypes.New(codeTooBigScale, mysql.MySQLErrName[mysql.ErrTooBigScale])
	// ErrTooBigPrecision is returned when type DECIMAL/NUMERIC precision is bigger than mysql.MaxDecimalWidth
	ErrTooBigPrecision = terror.ClassTypes.New(codeTooBigPrecision, mysql.MySQLErrName[mysql.ErrTooBigPrecision])
	// ErrWrongFieldSpec is return when incorrect column specifier for column.
	ErrWrongFieldSpec = terror.ClassTypes.New(codeWrongFieldSpec, "Wrong Field Spec")
	// ErrBadNumber is return when parsing an invalid binary decimal number.
	ErrBadNumber = terror.ClassTypes.New(codeBadNumber, "Bad Number")
	// ErrInvalidDefault is returned when meet a invalid default value.
	ErrInvalidDefault = parser_types.ErrInvalidDefault
	// ErrCastAsSignedOverflow is returned when positive out-of-range integer, and convert to it's negative complement.
	ErrCastAsSignedOverflow = terror.ClassTypes.New(codeUnknown, msgCastAsSignedOverflow)
	// ErrCastNegIntAsUnsigned is returned when a negative integer be casted to an unsigned int.
	ErrCastNegIntAsUnsigned = terror.ClassTypes.New(codeUnknown, msgCastNegIntAsUnsigned)
	// ErrMBiggerThanD is returned when precision less than the scale.
	ErrMBiggerThanD = terror.ClassTypes.New(codeMBiggerThanD, mysql.MySQLErrName[mysql.ErrMBiggerThanD])
	// ErrWarnDataOutOfRange is returned when the value in a numeric column that is outside the permissible range of the column data type.
	// See https://dev.mysql.com/doc/refman/5.5/en/out-of-range-and-overflow.html for details
	ErrWarnDataOutOfRange = terror.ClassTypes.New(codeDataOutOfRange, mysql.MySQLErrName[mysql.ErrWarnDataOutOfRange])
)

const (
	codeBadNumber terror.ErrCode = 1

	codeDataTooLong         = terror.ErrCode(mysql.ErrDataTooLong)
	codeIllegalValueForType = terror.ErrCode(mysql.ErrIllegalValueForType)
	codeTruncated           = terror.ErrCode(mysql.WarnDataTruncated)
	codeOverflow            = terror.ErrCode(mysql.ErrDataOutOfRange)
	codeDivByZero           = terror.ErrCode(mysql.ErrDivisionByZero)
	codeTooBigDisplayWidth  = terror.ErrCode(mysql.ErrTooBigDisplaywidth)
	codeTooBigFieldLength   = terror.ErrCode(mysql.ErrTooBigFieldlength)
	codeTooBigSet           = terror.ErrCode(mysql.ErrTooBigSet)
	codeTooBigScale         = terror.ErrCode(mysql.ErrTooBigScale)
	codeTooBigPrecision     = terror.ErrCode(mysql.ErrTooBigPrecision)
	codeWrongFieldSpec      = terror.ErrCode(mysql.ErrWrongFieldSpec)
	codeTruncatedWrongValue = terror.ErrCode(mysql.ErrTruncatedWrongValue)
	codeUnknown             = terror.ErrCode(mysql.ErrUnknown)
	codeInvalidDefault      = terror.ErrCode(mysql.ErrInvalidDefault)
	codeMBiggerThanD        = terror.ErrCode(mysql.ErrMBiggerThanD)
	codeDataOutOfRange      = terror.ErrCode(mysql.ErrWarnDataOutOfRange)
)

var (
	msgOverflow             = mysql.MySQLErrName[mysql.ErrDataOutOfRange]
	msgTruncatedWrongVal    = mysql.MySQLErrName[mysql.ErrTruncatedWrongValue]
	msgCastAsSignedOverflow = "Cast to signed converted positive out-of-range integer to it's negative complement"
	msgCastNegIntAsUnsigned = "Cast to unsigned converted negative integer to it's positive complement"
)

func init() {
	typesMySQLErrCodes := map[terror.ErrCode]uint16{
		codeDataTooLong:         mysql.ErrDataTooLong,
		codeIllegalValueForType: mysql.ErrIllegalValueForType,
		codeTruncated:           mysql.WarnDataTruncated,
		codeOverflow:            mysql.ErrDataOutOfRange,
		codeDivByZero:           mysql.ErrDivisionByZero,
		codeTooBigDisplayWidth:  mysql.ErrTooBigDisplaywidth,
		codeTooBigFieldLength:   mysql.ErrTooBigFieldlength,
		codeTooBigSet:           mysql.ErrTooBigSet,
		codeTooBigScale:         mysql.ErrTooBigScale,
		codeTooBigPrecision:     mysql.ErrTooBigPrecision,
		codeWrongFieldSpec:      mysql.ErrWrongFieldSpec,
		codeTruncatedWrongValue: mysql.ErrTruncatedWrongValue,
		codeUnknown:             mysql.ErrUnknown,
		codeInvalidDefault:      mysql.ErrInvalidDefault,
		codeMBiggerThanD:        mysql.ErrMBiggerThanD,
		codeDataOutOfRange:      mysql.ErrWarnDataOutOfRange,
	}
	terror.ErrClassToMySQLCodes[terror.ClassTypes] = typesMySQLErrCodes
}
