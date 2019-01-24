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

package ast

import (
	"fmt"
	"io"
	"strings"

	"github.com/pingcap/errors"
	. "github.com/pingcap/parser/format"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/types"
)

var (
	_ FuncNode = &AggregateFuncExpr{}
	_ FuncNode = &FuncCallExpr{}
	_ FuncNode = &FuncCastExpr{}
	_ FuncNode = &WindowFuncExpr{}
)

// List scalar function names.
const (
	LogicAnd   = "and"
	Cast       = "cast"
	LeftShift  = "leftshift"
	RightShift = "rightshift"
	LogicOr    = "or"
	GE         = "ge"
	LE         = "le"
	EQ         = "eq"
	NE         = "ne"
	LT         = "lt"
	GT         = "gt"
	Plus       = "plus"
	Minus      = "minus"
	And        = "bitand"
	Or         = "bitor"
	Mod        = "mod"
	Xor        = "bitxor"
	Div        = "div"
	Mul        = "mul"
	UnaryNot   = "not" // Avoid name conflict with Not in github/pingcap/check.
	BitNeg     = "bitneg"
	IntDiv     = "intdiv"
	LogicXor   = "xor"
	NullEQ     = "nulleq"
	UnaryPlus  = "unaryplus"
	UnaryMinus = "unaryminus"
	In         = "in"
	Like       = "like"
	Case       = "case"
	Regexp     = "regexp"
	IsNull     = "isnull"
	IsTruth    = "istrue"  // Avoid name conflict with IsTrue in github/pingcap/check.
	IsFalsity  = "isfalse" // Avoid name conflict with IsFalse in github/pingcap/check.
	RowFunc    = "row"
	SetVar     = "setvar"
	GetVar     = "getvar"
	Values     = "values"
	BitCount   = "bit_count"
	GetParam   = "getparam"

	// common functions
	Coalesce = "coalesce"
	Greatest = "greatest"
	Least    = "least"
	Interval = "interval"

	// math functions
	Abs      = "abs"
	Acos     = "acos"
	Asin     = "asin"
	Atan     = "atan"
	Atan2    = "atan2"
	Ceil     = "ceil"
	Ceiling  = "ceiling"
	Conv     = "conv"
	Cos      = "cos"
	Cot      = "cot"
	CRC32    = "crc32"
	Degrees  = "degrees"
	Exp      = "exp"
	Floor    = "floor"
	Ln       = "ln"
	Log      = "log"
	Log2     = "log2"
	Log10    = "log10"
	PI       = "pi"
	Pow      = "pow"
	Power    = "power"
	Radians  = "radians"
	Rand     = "rand"
	Round    = "round"
	Sign     = "sign"
	Sin      = "sin"
	Sqrt     = "sqrt"
	Tan      = "tan"
	Truncate = "truncate"

	// time functions
	AddDate          = "adddate"
	AddTime          = "addtime"
	ConvertTz        = "convert_tz"
	Curdate          = "curdate"
	CurrentDate      = "current_date"
	CurrentTime      = "current_time"
	CurrentTimestamp = "current_timestamp"
	Curtime          = "curtime"
	Date             = "date"
	DateLiteral      = "dateliteral"
	DateAdd          = "date_add"
	DateFormat       = "date_format"
	DateSub          = "date_sub"
	DateDiff         = "datediff"
	Day              = "day"
	DayName          = "dayname"
	DayOfMonth       = "dayofmonth"
	DayOfWeek        = "dayofweek"
	DayOfYear        = "dayofyear"
	Extract          = "extract"
	FromDays         = "from_days"
	FromUnixTime     = "from_unixtime"
	GetFormat        = "get_format"
	Hour             = "hour"
	LocalTime        = "localtime"
	LocalTimestamp   = "localtimestamp"
	MakeDate         = "makedate"
	MakeTime         = "maketime"
	MicroSecond      = "microsecond"
	Minute           = "minute"
	Month            = "month"
	MonthName        = "monthname"
	Now              = "now"
	PeriodAdd        = "period_add"
	PeriodDiff       = "period_diff"
	Quarter          = "quarter"
	SecToTime        = "sec_to_time"
	Second           = "second"
	StrToDate        = "str_to_date"
	SubDate          = "subdate"
	SubTime          = "subtime"
	Sysdate          = "sysdate"
	Time             = "time"
	TimeLiteral      = "timeliteral"
	TimeFormat       = "time_format"
	TimeToSec        = "time_to_sec"
	TimeDiff         = "timediff"
	Timestamp        = "timestamp"
	TimestampLiteral = "timestampliteral"
	TimestampAdd     = "timestampadd"
	TimestampDiff    = "timestampdiff"
	ToDays           = "to_days"
	ToSeconds        = "to_seconds"
	UnixTimestamp    = "unix_timestamp"
	UTCDate          = "utc_date"
	UTCTime          = "utc_time"
	UTCTimestamp     = "utc_timestamp"
	Week             = "week"
	Weekday          = "weekday"
	WeekOfYear       = "weekofyear"
	Year             = "year"
	YearWeek         = "yearweek"
	LastDay          = "last_day"
	TiDBParseTso     = "tidb_parse_tso"

	// string functions
	ASCII           = "ascii"
	Bin             = "bin"
	Concat          = "concat"
	ConcatWS        = "concat_ws"
	Convert         = "convert"
	Elt             = "elt"
	ExportSet       = "export_set"
	Field           = "field"
	Format          = "format"
	FromBase64      = "from_base64"
	InsertFunc      = "insert_func"
	Instr           = "instr"
	Lcase           = "lcase"
	Left            = "left"
	Length          = "length"
	LoadFile        = "load_file"
	Locate          = "locate"
	Lower           = "lower"
	Lpad            = "lpad"
	LTrim           = "ltrim"
	MakeSet         = "make_set"
	Mid             = "mid"
	Oct             = "oct"
	Ord             = "ord"
	Position        = "position"
	Quote           = "quote"
	Repeat          = "repeat"
	Replace         = "replace"
	Reverse         = "reverse"
	Right           = "right"
	RTrim           = "rtrim"
	Space           = "space"
	Strcmp          = "strcmp"
	Substring       = "substring"
	Substr          = "substr"
	SubstringIndex  = "substring_index"
	ToBase64        = "to_base64"
	Trim            = "trim"
	Upper           = "upper"
	Ucase           = "ucase"
	Hex             = "hex"
	Unhex           = "unhex"
	Rpad            = "rpad"
	BitLength       = "bit_length"
	CharFunc        = "char_func"
	CharLength      = "char_length"
	CharacterLength = "character_length"
	FindInSet       = "find_in_set"

	// information functions
	Benchmark      = "benchmark"
	Charset        = "charset"
	Coercibility   = "coercibility"
	Collation      = "collation"
	ConnectionID   = "connection_id"
	CurrentUser    = "current_user"
	Database       = "database"
	FoundRows      = "found_rows"
	LastInsertId   = "last_insert_id"
	RowCount       = "row_count"
	Schema         = "schema"
	SessionUser    = "session_user"
	SystemUser     = "system_user"
	User           = "user"
	Version        = "version"
	TiDBVersion    = "tidb_version"
	TiDBIsDDLOwner = "tidb_is_ddl_owner"

	// control functions
	If     = "if"
	Ifnull = "ifnull"
	Nullif = "nullif"

	// miscellaneous functions
	AnyValue        = "any_value"
	DefaultFunc     = "default_func"
	InetAton        = "inet_aton"
	InetNtoa        = "inet_ntoa"
	Inet6Aton       = "inet6_aton"
	Inet6Ntoa       = "inet6_ntoa"
	IsFreeLock      = "is_free_lock"
	IsIPv4          = "is_ipv4"
	IsIPv4Compat    = "is_ipv4_compat"
	IsIPv4Mapped    = "is_ipv4_mapped"
	IsIPv6          = "is_ipv6"
	IsUsedLock      = "is_used_lock"
	MasterPosWait   = "master_pos_wait"
	NameConst       = "name_const"
	ReleaseAllLocks = "release_all_locks"
	Sleep           = "sleep"
	UUID            = "uuid"
	UUIDShort       = "uuid_short"
	// get_lock() and release_lock() is parsed but do nothing.
	// It is used for preventing error in Ruby's activerecord migrations.
	GetLock     = "get_lock"
	ReleaseLock = "release_lock"

	// encryption and compression functions
	AesDecrypt               = "aes_decrypt"
	AesEncrypt               = "aes_encrypt"
	Compress                 = "compress"
	Decode                   = "decode"
	DesDecrypt               = "des_decrypt"
	DesEncrypt               = "des_encrypt"
	Encode                   = "encode"
	Encrypt                  = "encrypt"
	MD5                      = "md5"
	OldPassword              = "old_password"
	PasswordFunc             = "password_func"
	RandomBytes              = "random_bytes"
	SHA1                     = "sha1"
	SHA                      = "sha"
	SHA2                     = "sha2"
	Uncompress               = "uncompress"
	UncompressedLength       = "uncompressed_length"
	ValidatePasswordStrength = "validate_password_strength"

	// json functions
	JSONType          = "json_type"
	JSONExtract       = "json_extract"
	JSONUnquote       = "json_unquote"
	JSONArray         = "json_array"
	JSONObject        = "json_object"
	JSONMerge         = "json_merge"
	JSONSet           = "json_set"
	JSONInsert        = "json_insert"
	JSONReplace       = "json_replace"
	JSONRemove        = "json_remove"
	JSONContains      = "json_contains"
	JSONContainsPath  = "json_contains_path"
	JSONValid         = "json_valid"
	JSONArrayAppend   = "json_array_append"
	JSONArrayInsert   = "json_array_insert"
	JSONMergePatch    = "json_merge_patch"
	JSONMergePreserve = "json_merge_preserve"
	JSONPretty        = "json_pretty"
	JSONQuote         = "json_quote"
	JSONSearch        = "json_search"
	JSONStorageSize   = "json_storage_size"
	JSONDepth         = "json_depth"
	JSONKeys          = "json_keys"
	JSONLength        = "json_length"
)

// FuncCallExpr is for function expression.
type FuncCallExpr struct {
	funcNode
	// FnName is the function name.
	FnName model.CIStr
	// Args is the function args.
	Args []ExprNode
}

// Restore implements Node interface.
func (n *FuncCallExpr) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord(n.FnName.O)
	ctx.WritePlain("(")
	switch n.FnName.L {
	case "convert":
		if err := n.Args[0].Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCastExpr.Expr")
		}
		ctx.WriteKeyWord(" USING ")
		ctx.WriteKeyWord(n.Args[1].GetType().Charset)
	case "adddate", "subdate", "date_add", "date_sub":
		if err := n.Args[0].Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
		}
		ctx.WritePlain(", ")
		ctx.WriteKeyWord("INTERVAL ")
		if err := n.Args[1].Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
		}
		ctx.WritePlain(" ")
		ctx.WriteKeyWord(n.Args[2].(ValueExpr).GetString())
	case "extract":
		ctx.WriteKeyWord(n.Args[0].(ValueExpr).GetString())
		ctx.WriteKeyWord(" FROM ")
		if err := n.Args[1].Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
		}
	case "get_format":
		ctx.WriteKeyWord(n.Args[0].(ValueExpr).GetString())
		ctx.WritePlain(", ")
		if err := n.Args[1].Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
		}
	case "position":
		if err := n.Args[0].Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
		}
		ctx.WriteKeyWord(" IN ")
		if err := n.Args[1].Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
		}
	case "trim":
		switch len(n.Args) {
		case 1:
			if err := n.Args[0].Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
			}
		case 2:
			if err := n.Args[1].Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
			}
			ctx.WriteKeyWord(" FROM ")
			if err := n.Args[0].Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
			}
		case 3:
			switch fmt.Sprint(n.Args[2].(ValueExpr).GetValue()) {
			case "3":
				ctx.WriteKeyWord("TRAILING ")
			case "2":
				ctx.WriteKeyWord("LEADING ")
			case "0", "1":
				ctx.WriteKeyWord("BOTH ")
			}
			if n.Args[1].(ValueExpr).GetValue() != nil {
				if err := n.Args[1].Restore(ctx); err != nil {
					return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
				}
				ctx.WritePlain(" ")
			}
			ctx.WriteKeyWord("FROM ")
			if err := n.Args[0].Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
			}
		}
	case "timestampdiff", "timestampadd":
		ctx.WriteKeyWord(n.Args[0].(ValueExpr).GetString())
		for i := 1; i < len(n.Args); {
			ctx.WritePlain(", ")
			if err := n.Args[i].Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore FuncCallExpr")
			}
			i++
		}
	default:
		for i, argv := range n.Args {
			if i != 0 {
				ctx.WritePlain(", ")
			}
			if err := argv.Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore FuncCallExpr.Args %d", i)
			}
		}
	}
	ctx.WritePlain(")")
	return nil
}

// Format the ExprNode into a Writer.
func (n *FuncCallExpr) Format(w io.Writer) {
	fmt.Fprintf(w, "%s(", n.FnName.L)
	if !n.specialFormatArgs(w) {
		for i, arg := range n.Args {
			arg.Format(w)
			if i != len(n.Args)-1 {
				fmt.Fprint(w, ", ")
			}
		}
	}
	fmt.Fprint(w, ")")
}

// specialFormatArgs formats argument list for some special functions.
func (n *FuncCallExpr) specialFormatArgs(w io.Writer) bool {
	switch n.FnName.L {
	case DateAdd, DateSub, AddDate, SubDate:
		n.Args[0].Format(w)
		fmt.Fprint(w, ", INTERVAL ")
		n.Args[1].Format(w)
		fmt.Fprintf(w, " %s", n.Args[2].(ValueExpr).GetDatumString())
		return true
	case TimestampAdd, TimestampDiff:
		fmt.Fprintf(w, "%s, ", n.Args[0].(ValueExpr).GetDatumString())
		n.Args[1].Format(w)
		fmt.Fprint(w, ", ")
		n.Args[2].Format(w)
		return true
	}
	return false
}

// Accept implements Node interface.
func (n *FuncCallExpr) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*FuncCallExpr)
	for i, val := range n.Args {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Args[i] = node.(ExprNode)
	}
	return v.Leave(n)
}

// CastFunctionType is the type for cast function.
type CastFunctionType int

// CastFunction types
const (
	CastFunction CastFunctionType = iota + 1
	CastConvertFunction
	CastBinaryOperator
)

// FuncCastExpr is the cast function converting value to another type, e.g, cast(expr AS signed).
// See https://dev.mysql.com/doc/refman/5.7/en/cast-functions.html
type FuncCastExpr struct {
	funcNode
	// Expr is the expression to be converted.
	Expr ExprNode
	// Tp is the conversion type.
	Tp *types.FieldType
	// FunctionType is either Cast, Convert or Binary.
	FunctionType CastFunctionType
}

// Restore implements Node interface.
func (n *FuncCastExpr) Restore(ctx *RestoreCtx) error {
	switch n.FunctionType {
	case CastFunction:
		ctx.WriteKeyWord("CAST")
		ctx.WritePlain("(")
		if err := n.Expr.Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCastExpr.Expr")
		}
		ctx.WriteKeyWord(" AS ")
		n.Tp.FormatAsCastType(ctx.In)
		ctx.WritePlain(")")
	case CastConvertFunction:
		ctx.WriteKeyWord("CONVERT")
		ctx.WritePlain("(")
		if err := n.Expr.Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCastExpr.Expr")
		}
		ctx.WritePlain(", ")
		n.Tp.FormatAsCastType(ctx.In)
		ctx.WritePlain(")")
	case CastBinaryOperator:
		ctx.WriteKeyWord("BINARY ")
		if err := n.Expr.Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore FuncCastExpr.Expr")
		}
	}
	return nil
}

// Format the ExprNode into a Writer.
func (n *FuncCastExpr) Format(w io.Writer) {
	switch n.FunctionType {
	case CastFunction:
		fmt.Fprint(w, "CAST(")
		n.Expr.Format(w)
		fmt.Fprint(w, " AS ")
		n.Tp.FormatAsCastType(w)
		fmt.Fprint(w, ")")
	case CastConvertFunction:
		fmt.Fprint(w, "CONVERT(")
		n.Expr.Format(w)
		fmt.Fprint(w, ", ")
		n.Tp.FormatAsCastType(w)
		fmt.Fprint(w, ")")
	case CastBinaryOperator:
		fmt.Fprint(w, "BINARY ")
		n.Expr.Format(w)
	}
}

// Accept implements Node Accept interface.
func (n *FuncCastExpr) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*FuncCastExpr)
	node, ok := n.Expr.Accept(v)
	if !ok {
		return n, false
	}
	n.Expr = node.(ExprNode)
	return v.Leave(n)
}

// TrimDirectionType is the type for trim direction.
type TrimDirectionType int

const (
	// TrimBothDefault trims from both direction by default.
	TrimBothDefault TrimDirectionType = iota
	// TrimBoth trims from both direction with explicit notation.
	TrimBoth
	// TrimLeading trims from left.
	TrimLeading
	// TrimTrailing trims from right.
	TrimTrailing
)

// DateArithType is type for DateArith type.
type DateArithType byte

const (
	// DateArithAdd is to run adddate or date_add function option.
	// See https://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_adddate
	// See https://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_date-add
	DateArithAdd DateArithType = iota + 1
	// DateArithSub is to run subdate or date_sub function option.
	// See https://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_subdate
	// See https://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_date-sub
	DateArithSub
)

const (
	// AggFuncCount is the name of Count function.
	AggFuncCount = "count"
	// AggFuncSum is the name of Sum function.
	AggFuncSum = "sum"
	// AggFuncAvg is the name of Avg function.
	AggFuncAvg = "avg"
	// AggFuncFirstRow is the name of FirstRowColumn function.
	AggFuncFirstRow = "firstrow"
	// AggFuncMax is the name of max function.
	AggFuncMax = "max"
	// AggFuncMin is the name of min function.
	AggFuncMin = "min"
	// AggFuncGroupConcat is the name of group_concat function.
	AggFuncGroupConcat = "group_concat"
	// AggFuncBitOr is the name of bit_or function.
	AggFuncBitOr = "bit_or"
	// AggFuncBitXor is the name of bit_xor function.
	AggFuncBitXor = "bit_xor"
	// AggFuncBitAnd is the name of bit_and function.
	AggFuncBitAnd = "bit_and"
	// AggFuncVarPop is the name of var_pop function
	AggFuncVarPop = "var_pop"
	// AggFuncVarSamp is the name of var_samp function
	AggFuncVarSamp = "var_samp"
	// AggFuncStddevPop is the name of stddev_pop function
	AggFuncStddevPop = "stddev_pop"
	// AggFuncStddevSamp is the name of stddev_samp function
	AggFuncStddevSamp = "stddev_samp"
)

// AggregateFuncExpr represents aggregate function expression.
type AggregateFuncExpr struct {
	funcNode
	// F is the function name.
	F string
	// Args is the function args.
	Args []ExprNode
	// Distinct is true, function hence only aggregate distinct values.
	// For example, column c1 values are "1", "2", "2",  "sum(c1)" is "5",
	// but "sum(distinct c1)" is "3".
	Distinct bool
}

// Restore implements Node interface.
func (n *AggregateFuncExpr) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord(n.F)
	ctx.WritePlain("(")
	if n.Distinct {
		ctx.WriteKeyWord("DISTINCT ")
	}
	switch strings.ToLower(n.F) {
	case "group_concat":
		for i := 0; i < len(n.Args)-1; i++ {
			if i != 0 {
				ctx.WritePlain(", ")
			}
			if err := n.Args[i].Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore AggregateFuncExpr.Args[%d]", i)
			}
		}
		ctx.WriteKeyWord(" SEPARATOR ")
		if err := n.Args[len(n.Args)-1].Restore(ctx); err != nil {
			return errors.Annotate(err, "An error occurred while restore AggregateFuncExpr.Args SEPARATOR")
		}
	default:
		for i, argv := range n.Args {
			if i != 0 {
				ctx.WritePlain(", ")
			}
			if err := argv.Restore(ctx); err != nil {
				return errors.Annotatef(err, "An error occurred while restore AggregateFuncExpr.Args[%d]", i)
			}
		}
	}
	ctx.WritePlain(")")
	return nil
}

// Format the ExprNode into a Writer.
func (n *AggregateFuncExpr) Format(w io.Writer) {
	panic("Not implemented")
}

// Accept implements Node Accept interface.
func (n *AggregateFuncExpr) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*AggregateFuncExpr)
	for i, val := range n.Args {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Args[i] = node.(ExprNode)
	}
	return v.Leave(n)
}

const (
	// WindowFuncRowNumber is the name of row_number function.
	WindowFuncRowNumber = "row_number"
	// WindowFuncRank is the name of rank function.
	WindowFuncRank = "rank"
	// WindowFuncDenseRank is the name of dense_rank function.
	WindowFuncDenseRank = "dense_rank"
	// WindowFuncCumeDist is the name of cume_dist function.
	WindowFuncCumeDist = "cume_dist"
	// WindowFuncPercentRank is the name of percent_rank function.
	WindowFuncPercentRank = "percent_rank"
	// WindowFuncNtile is the name of ntile function.
	WindowFuncNtile = "ntile"
	// WindowFuncLead is the name of lead function.
	WindowFuncLead = "lead"
	// WindowFuncLag is the name of lag function.
	WindowFuncLag = "lag"
	// WindowFuncFirstValue is the name of first_value function.
	WindowFuncFirstValue = "first_value"
	// WindowFuncLastValue is the name of last_value function.
	WindowFuncLastValue = "last_value"
	// WindowFuncNthValue is the name of nth_value function.
	WindowFuncNthValue = "nth_value"
)

// WindowFuncExpr represents window function expression.
type WindowFuncExpr struct {
	funcNode

	// F is the function name.
	F string
	// Args is the function args.
	Args []ExprNode
	// Distinct cannot be true for most window functions, except `max` and `min`.
	// We need to raise error if it is not allowed to be true.
	Distinct bool
	// IgnoreNull indicates how to handle null value.
	// MySQL only supports `RESPECT NULLS`, so we need to raise error if it is true.
	IgnoreNull bool
	// FromLast indicates the calculation direction of this window function.
	// MySQL only supports calculation from first, so we need to raise error if it is true.
	FromLast bool
	// Spec is the specification of this window.
	Spec WindowSpec
}

// Restore implements Node interface.
func (n *WindowFuncExpr) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord(n.F)
	ctx.WritePlain("(")
	for i, v := range n.Args {
		if i != 0 {
			ctx.WritePlain(", ")
		} else if n.Distinct {
			ctx.WriteKeyWord("DISTINCT ")
		}
		if err := v.Restore(ctx); err != nil {
			return errors.Annotatef(err, "An error occurred while restore WindowFuncExpr.Args[%d]", i)
		}
	}
	ctx.WritePlain(")")
	if n.FromLast {
		ctx.WriteKeyWord(" FROM LAST")
	}
	if n.IgnoreNull {
		ctx.WriteKeyWord(" IGNORE NULLS")
	}
	ctx.WriteKeyWord(" OVER ")
	if err := n.Spec.Restore(ctx); err != nil {
		return errors.Annotate(err, "An error occurred while restore WindowFuncExpr.Spec")
	}

	return nil
}

// Format formats the window function expression into a Writer.
func (n *WindowFuncExpr) Format(w io.Writer) {
	panic("Not implemented")
}

// Accept implements Node Accept interface.
func (n *WindowFuncExpr) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*WindowFuncExpr)
	for i, val := range n.Args {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Args[i] = node.(ExprNode)
	}
	node, ok := n.Spec.Accept(v)
	if !ok {
		return n, false
	}
	n.Spec = *node.(*WindowSpec)
	return v.Leave(n)
}
