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

package types

import (
	"fmt"
	"io"
	"strings"

	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/format"
	"github.com/pingcap/parser/mysql"
)

// UnspecifiedLength is unspecified length.
const (
	UnspecifiedLength = -1
)

// FieldType records field type information.
type FieldType struct {
	Tp      byte
	Flag    uint
	Flen    int
	Decimal int
	Charset string
	Collate string
	// Elems is the element list for enum and set type.
	Elems []string
}

// NewFieldType returns a FieldType,
// with a type and other information about field type.
func NewFieldType(tp byte) *FieldType {
	return &FieldType{
		Tp:      tp,
		Flen:    UnspecifiedLength,
		Decimal: UnspecifiedLength,
	}
}

// Clone returns a copy of itself.
func (ft *FieldType) Clone() *FieldType {
	ret := *ft
	return &ret
}

// Equal checks whether two FieldType objects are equal.
func (ft *FieldType) Equal(other *FieldType) bool {
	// We do not need to compare whole `ft.Flag == other.Flag` when wrapping cast upon an Expression.
	// but need compare unsigned_flag of ft.Flag.
	partialEqual := ft.Tp == other.Tp &&
		ft.Flen == other.Flen &&
		ft.Decimal == other.Decimal &&
		ft.Charset == other.Charset &&
		ft.Collate == other.Collate &&
		mysql.HasUnsignedFlag(ft.Flag) == mysql.HasUnsignedFlag(other.Flag)
	if !partialEqual || len(ft.Elems) != len(other.Elems) {
		return false
	}
	for i := range ft.Elems {
		if ft.Elems[i] != other.Elems[i] {
			return false
		}
	}
	return true
}

// EvalType gets the type in evaluation.
func (ft *FieldType) EvalType() EvalType {
	switch ft.Tp {
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong,
		mysql.TypeBit, mysql.TypeYear:
		return ETInt
	case mysql.TypeFloat, mysql.TypeDouble:
		return ETReal
	case mysql.TypeNewDecimal:
		return ETDecimal
	case mysql.TypeDate, mysql.TypeDatetime:
		return ETDatetime
	case mysql.TypeTimestamp:
		return ETTimestamp
	case mysql.TypeDuration:
		return ETDuration
	case mysql.TypeJSON:
		return ETJson
	}
	return ETString
}

// Hybrid checks whether a type is a hybrid type, which can represent different types of value in specific context.
func (ft *FieldType) Hybrid() bool {
	return ft.Tp == mysql.TypeEnum || ft.Tp == mysql.TypeBit || ft.Tp == mysql.TypeSet
}

// Init initializes the FieldType data.
func (ft *FieldType) Init(tp byte) {
	ft.Tp = tp
	ft.Flen = UnspecifiedLength
	ft.Decimal = UnspecifiedLength
}

// CompactStr only considers Tp/CharsetBin/Flen/Deimal.
// This is used for showing column type in infoschema.
func (ft *FieldType) CompactStr() string {
	ts := TypeToStr(ft.Tp, ft.Charset)
	suffix := ""

	defaultFlen, defaultDecimal := mysql.GetDefaultFieldLengthAndDecimal(ft.Tp)
	isDecimalNotDefault := ft.Decimal != defaultDecimal && ft.Decimal != 0 && ft.Decimal != UnspecifiedLength

	// displayFlen and displayDecimal are flen and decimal values with `-1` substituted with default value.
	displayFlen, displayDecimal := ft.Flen, ft.Decimal
	if displayFlen == 0 || displayFlen == UnspecifiedLength {
		displayFlen = defaultFlen
	}
	if displayDecimal == 0 || displayDecimal == UnspecifiedLength {
		displayDecimal = defaultDecimal
	}

	switch ft.Tp {
	case mysql.TypeEnum, mysql.TypeSet:
		// Format is ENUM ('e1', 'e2') or SET ('e1', 'e2')
		es := make([]string, 0, len(ft.Elems))
		for _, e := range ft.Elems {
			e = format.OutputFormat(e)
			es = append(es, e)
		}
		suffix = fmt.Sprintf("('%s')", strings.Join(es, "','"))
	case mysql.TypeTimestamp, mysql.TypeDatetime, mysql.TypeDuration:
		if isDecimalNotDefault {
			suffix = fmt.Sprintf("(%d)", displayDecimal)
		}
	case mysql.TypeDouble, mysql.TypeFloat:
		// 1. Flen Not Default, Decimal Not Default -> Valid
		// 2. Flen Not Default, Decimal Default (-1) -> Invalid
		// 3. Flen Default, Decimal Not Default -> Valid
		// 4. Flen Default, Decimal Default -> Valid (hide)
		if isDecimalNotDefault {
			suffix = fmt.Sprintf("(%d,%d)", displayFlen, displayDecimal)
		}
	case mysql.TypeNewDecimal:
		suffix = fmt.Sprintf("(%d,%d)", displayFlen, displayDecimal)
	case mysql.TypeBit, mysql.TypeShort, mysql.TypeTiny, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong, mysql.TypeVarchar, mysql.TypeString, mysql.TypeVarString:
		// Flen is always shown.
		suffix = fmt.Sprintf("(%d)", displayFlen)
	}
	return ts + suffix
}

// InfoSchemaStr joins the CompactStr with unsigned flag and
// returns a string.
func (ft *FieldType) InfoSchemaStr() string {
	suffix := ""
	if mysql.HasUnsignedFlag(ft.Flag) {
		suffix = " unsigned"
	}
	return ft.CompactStr() + suffix
}

// String joins the information of FieldType and returns a string.
// Note: when flen or decimal is unspecified, this function will use the default value instead of -1.
func (ft *FieldType) String() string {
	strs := []string{ft.CompactStr()}
	if mysql.HasUnsignedFlag(ft.Flag) {
		strs = append(strs, "UNSIGNED")
	}
	if mysql.HasZerofillFlag(ft.Flag) {
		strs = append(strs, "ZEROFILL")
	}
	if mysql.HasBinaryFlag(ft.Flag) && ft.Tp != mysql.TypeString {
		strs = append(strs, "BINARY")
	}

	if IsTypeChar(ft.Tp) || IsTypeBlob(ft.Tp) {
		if ft.Charset != "" && ft.Charset != charset.CharsetBin {
			strs = append(strs, fmt.Sprintf("CHARACTER SET %s", ft.Charset))
		}
		if ft.Collate != "" && ft.Collate != charset.CharsetBin {
			strs = append(strs, fmt.Sprintf("COLLATE %s", ft.Collate))
		}
	}

	return strings.Join(strs, " ")
}

// Restore implements Node interface.
func (ft *FieldType) Restore(ctx *format.RestoreCtx) error {
	ctx.WriteKeyWord(TypeToStr(ft.Tp, ft.Charset))

	switch ft.Tp {
	case mysql.TypeEnum, mysql.TypeSet:
		ctx.WritePlain("(")
		for i, e := range ft.Elems {
			if i != 0 {
				ctx.WritePlain(",")
			}
			ctx.WriteString(e)
		}
		ctx.WritePlain(")")
	case mysql.TypeTimestamp, mysql.TypeDatetime, mysql.TypeDuration:
		if ft.Flen > 0 && ft.Decimal > 0 {
			ctx.WritePlainf("(%d)", ft.Decimal)
		}
	case mysql.TypeDouble, mysql.TypeFloat:
		if ft.Flen > 0 && ft.Decimal > 0 {
			ctx.WritePlainf("(%d,%d)", ft.Flen, ft.Decimal)
		}
	case mysql.TypeNewDecimal:
		if ft.Flen > 0 && ft.Decimal > 0 {
			ctx.WritePlainf("(%d,%d)", ft.Flen, ft.Decimal)
		}
	case mysql.TypeBit, mysql.TypeShort, mysql.TypeTiny, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong, mysql.TypeVarchar, mysql.TypeString, mysql.TypeVarString:
		if ft.Flen > 0 {
			ctx.WritePlainf("(%d)", ft.Flen)
		}
	}

	if mysql.HasUnsignedFlag(ft.Flag) {
		ctx.WriteKeyWord(" UNSIGNED")
	}
	if mysql.HasZerofillFlag(ft.Flag) {
		ctx.WriteKeyWord(" ZEROFILL")
	}
	if mysql.HasBinaryFlag(ft.Flag) && ft.Charset != charset.CharsetBin {
		ctx.WriteKeyWord(" BINARY")
	}

	if IsTypeChar(ft.Tp) || IsTypeBlob(ft.Tp) {
		if ft.Charset != "" && ft.Charset != charset.CharsetBin {
			ctx.WriteKeyWord(" CHARACTER SET " + ft.Charset)
		}
		if ft.Collate != "" && ft.Collate != charset.CharsetBin {
			ctx.WriteKeyWord(" COLLATE ")
			ctx.WritePlain(ft.Collate)
		}
	}

	return nil
}

// FormatAsCastType is used for write AST back to string.
func (ft *FieldType) FormatAsCastType(w io.Writer) {
	switch ft.Tp {
	case mysql.TypeVarString:
		if ft.Charset == charset.CharsetBin && ft.Collate == charset.CollationBin {
			fmt.Fprint(w, "BINARY")
		} else {
			fmt.Fprint(w, "CHAR")
		}
		if ft.Flen != UnspecifiedLength {
			fmt.Fprintf(w, "(%d)", ft.Flen)
		}
		if ft.Flag&mysql.BinaryFlag != 0 {
			fmt.Fprint(w, " BINARY")
		}
		if ft.Charset != charset.CharsetBin && ft.Charset != mysql.DefaultCharset {
			fmt.Fprintf(w, " CHARACTER SET %s", ft.Charset)
		}
	case mysql.TypeDate:
		fmt.Fprint(w, "DATE")
	case mysql.TypeDatetime:
		fmt.Fprint(w, "DATETIME")
		if ft.Decimal > 0 {
			fmt.Fprintf(w, "(%d)", ft.Decimal)
		}
	case mysql.TypeNewDecimal:
		fmt.Fprint(w, "DECIMAL")
		if ft.Flen > 0 && ft.Decimal > 0 {
			fmt.Fprintf(w, "(%d, %d)", ft.Flen, ft.Decimal)
		} else if ft.Flen > 0 {
			fmt.Fprintf(w, "(%d)", ft.Flen)
		}
	case mysql.TypeDuration:
		fmt.Fprint(w, "TIME")
		if ft.Decimal > 0 {
			fmt.Fprintf(w, "(%d)", ft.Decimal)
		}
	case mysql.TypeLonglong:
		if ft.Flag&mysql.UnsignedFlag != 0 {
			fmt.Fprint(w, "UNSIGNED")
		} else {
			fmt.Fprint(w, "SIGNED")
		}
	case mysql.TypeJSON:
		fmt.Fprint(w, "JSON")
	}
}

// VarStorageLen indicates this column is a variable length column.
const VarStorageLen = -1

// StorageLength is the length of stored value for the type.
func (ft *FieldType) StorageLength() int {
	switch ft.Tp {
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong,
		mysql.TypeLonglong, mysql.TypeDouble, mysql.TypeFloat, mysql.TypeYear, mysql.TypeDuration,
		mysql.TypeDate, mysql.TypeDatetime, mysql.TypeTimestamp, mysql.TypeEnum, mysql.TypeSet,
		mysql.TypeBit:
		// This may not be the accurate length, because we may encode them as varint.
		return 8
	case mysql.TypeNewDecimal:
		precision, frac := ft.Flen-ft.Decimal, ft.Decimal
		return precision/digitsPerWord*wordSize + dig2bytes[precision%digitsPerWord] + frac/digitsPerWord*wordSize + dig2bytes[frac%digitsPerWord]
	default:
		return VarStorageLen
	}
}
