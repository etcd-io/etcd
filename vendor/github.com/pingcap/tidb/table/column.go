// Copyright 2016 The ql Authors. All rights reserved.
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

package table

import (
	"strings"
	"unicode/utf8"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/hack"
	log "github.com/sirupsen/logrus"
)

// Column provides meta data describing a table column.
type Column struct {
	*model.ColumnInfo
	// If this column is a generated column, the expression will be stored here.
	GeneratedExpr ast.ExprNode
}

// String implements fmt.Stringer interface.
func (c *Column) String() string {
	ans := []string{c.Name.O, types.TypeToStr(c.Tp, c.Charset)}
	if mysql.HasAutoIncrementFlag(c.Flag) {
		ans = append(ans, "AUTO_INCREMENT")
	}
	if mysql.HasNotNullFlag(c.Flag) {
		ans = append(ans, "NOT NULL")
	}
	return strings.Join(ans, " ")
}

// ToInfo casts Column to model.ColumnInfo
// NOTE: DONT modify return value.
func (c *Column) ToInfo() *model.ColumnInfo {
	return c.ColumnInfo
}

// FindCol finds column in cols by name.
func FindCol(cols []*Column, name string) *Column {
	for _, col := range cols {
		if strings.EqualFold(col.Name.O, name) {
			return col
		}
	}
	return nil
}

// ToColumn converts a *model.ColumnInfo to *Column.
func ToColumn(col *model.ColumnInfo) *Column {
	return &Column{
		col,
		nil,
	}
}

// FindCols finds columns in cols by names.
// If pkIsHandle is false and name is ExtraHandleName, the extra handle column will be added.
func FindCols(cols []*Column, names []string, pkIsHandle bool) ([]*Column, error) {
	var rcols []*Column
	for _, name := range names {
		col := FindCol(cols, name)
		if col != nil {
			rcols = append(rcols, col)
		} else if name == model.ExtraHandleName.L && !pkIsHandle {
			col := &Column{}
			col.ColumnInfo = model.NewExtraHandleColInfo()
			col.ColumnInfo.Offset = len(cols)
			rcols = append(rcols, col)
		} else {
			return nil, errUnknownColumn.GenWithStack("unknown column %s", name)
		}
	}

	return rcols, nil
}

// FindOnUpdateCols finds columns which have OnUpdateNow flag.
func FindOnUpdateCols(cols []*Column) []*Column {
	var rcols []*Column
	for _, col := range cols {
		if mysql.HasOnUpdateNowFlag(col.Flag) {
			rcols = append(rcols, col)
		}
	}

	return rcols
}

// truncateTrailingSpaces trancates trailing spaces for CHAR[(M)] column.
// fix: https://github.com/pingcap/tidb/issues/3660
func truncateTrailingSpaces(v *types.Datum) {
	if v.Kind() == types.KindNull {
		return
	}
	b := v.GetBytes()
	length := len(b)
	for length > 0 && b[length-1] == ' ' {
		length--
	}
	b = b[:length]
	v.SetString(hack.String(b))
}

// CastValues casts values based on columns type.
func CastValues(ctx sessionctx.Context, rec []types.Datum, cols []*Column) (err error) {
	sc := ctx.GetSessionVars().StmtCtx
	for _, c := range cols {
		var converted types.Datum
		converted, err = CastValue(ctx, rec[c.Offset], c.ToInfo())
		if err != nil {
			if sc.DupKeyAsWarning {
				sc.AppendWarning(err)
				log.Warnf("cast values failed:%v", err)
			} else {
				return errors.Trace(err)
			}
		}
		rec[c.Offset] = converted
	}
	return nil
}

func handleWrongUtf8Value(ctx sessionctx.Context, col *model.ColumnInfo, casted *types.Datum, str string, i int) (types.Datum, error) {
	sc := ctx.GetSessionVars().StmtCtx
	err := ErrTruncateWrongValue.FastGen("incorrect utf8 value %x(%s) for column %s", casted.GetBytes(), str, col.Name)
	log.Errorf("con:%d %v", ctx.GetSessionVars().ConnectionID, err)
	// Truncate to valid utf8 string.
	truncateVal := types.NewStringDatum(str[:i])
	err = sc.HandleTruncate(err)
	return truncateVal, err
}

// CastValue casts a value based on column type.
func CastValue(ctx sessionctx.Context, val types.Datum, col *model.ColumnInfo) (casted types.Datum, err error) {
	sc := ctx.GetSessionVars().StmtCtx
	casted, err = val.ConvertTo(sc, &col.FieldType)
	// TODO: make sure all truncate errors are handled by ConvertTo.
	err = sc.HandleTruncate(err)
	if err != nil {
		return casted, errors.Trace(err)
	}

	if col.Tp == mysql.TypeString && !types.IsBinaryStr(&col.FieldType) {
		truncateTrailingSpaces(&casted)
	}

	if ctx.GetSessionVars().SkipUTF8Check {
		return casted, nil
	}
	if !mysql.IsUTF8Charset(col.Charset) {
		return casted, nil
	}
	str := casted.GetString()
	utf8Charset := col.Charset == mysql.UTF8Charset
	for i, w := 0, 0; i < len(str); i += w {
		runeValue, width := utf8.DecodeRuneInString(str[i:])
		if runeValue == utf8.RuneError {
			if strings.HasPrefix(str[i:], string(utf8.RuneError)) {
				w = width
				continue
			}
			casted, err = handleWrongUtf8Value(ctx, col, &casted, str, i)
			break
		} else if width > 3 && utf8Charset {
			// Handle non-BMP characters.
			casted, err = handleWrongUtf8Value(ctx, col, &casted, str, i)
			break
		}
		w = width
	}

	return casted, errors.Trace(err)
}

// ColDesc describes column information like MySQL desc and show columns do.
type ColDesc struct {
	Field        string
	Type         string
	Collation    string
	Null         string
	Key          string
	DefaultValue interface{}
	Extra        string
	Privileges   string
	Comment      string
}

const defaultPrivileges = "select,insert,update,references"

// GetTypeDesc gets the description for column type.
func (c *Column) GetTypeDesc() string {
	desc := c.FieldType.CompactStr()
	if mysql.HasUnsignedFlag(c.Flag) && c.Tp != mysql.TypeBit && c.Tp != mysql.TypeYear {
		desc += " UNSIGNED"
	}
	if mysql.HasZerofillFlag(c.Flag) && c.Tp != mysql.TypeYear {
		desc += " ZEROFILL"
	}
	return desc
}

// NewColDesc returns a new ColDesc for a column.
func NewColDesc(col *Column) *ColDesc {
	// TODO: if we have no primary key and a unique index which's columns are all not null
	// we will set these columns' flag as PriKeyFlag
	// see https://dev.mysql.com/doc/refman/5.7/en/show-columns.html
	// create table
	name := col.Name
	nullFlag := "YES"
	if mysql.HasNotNullFlag(col.Flag) {
		nullFlag = "NO"
	}
	keyFlag := ""
	if mysql.HasPriKeyFlag(col.Flag) {
		keyFlag = "PRI"
	} else if mysql.HasUniKeyFlag(col.Flag) {
		keyFlag = "UNI"
	} else if mysql.HasMultipleKeyFlag(col.Flag) {
		keyFlag = "MUL"
	}
	var defaultValue interface{}
	if !mysql.HasNoDefaultValueFlag(col.Flag) {
		defaultValue = col.GetDefaultValue()
	}

	extra := ""
	if mysql.HasAutoIncrementFlag(col.Flag) {
		extra = "auto_increment"
	} else if mysql.HasOnUpdateNowFlag(col.Flag) {
		extra = "on update CURRENT_TIMESTAMP"
	} else if col.IsGenerated() {
		if col.GeneratedStored {
			extra = "STORED GENERATED"
		} else {
			extra = "VIRTUAL GENERATED"
		}
	}

	return &ColDesc{
		Field:        name.O,
		Type:         col.GetTypeDesc(),
		Collation:    col.Collate,
		Null:         nullFlag,
		Key:          keyFlag,
		DefaultValue: defaultValue,
		Extra:        extra,
		Privileges:   defaultPrivileges,
		Comment:      col.Comment,
	}
}

// ColDescFieldNames returns the fields name in result set for desc and show columns.
func ColDescFieldNames(full bool) []string {
	if full {
		return []string{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"}
	}
	return []string{"Field", "Type", "Null", "Key", "Default", "Extra"}
}

// CheckOnce checks if there are duplicated column names in cols.
func CheckOnce(cols []*Column) error {
	m := map[string]struct{}{}
	for _, col := range cols {
		name := col.Name
		_, ok := m[name.L]
		if ok {
			return errDuplicateColumn.GenWithStack("column specified twice - %s", name)
		}

		m[name.L] = struct{}{}
	}

	return nil
}

// CheckNotNull checks if nil value set to a column with NotNull flag is set.
func (c *Column) CheckNotNull(data types.Datum) error {
	if mysql.HasNotNullFlag(c.Flag) && data.IsNull() {
		return ErrColumnCantNull.GenWithStackByArgs(c.Name)
	}
	return nil
}

// HandleBadNull handles the bad null error.
// If BadNullAsWarning is true, it will append the error as a warning, else return the error.
func (c *Column) HandleBadNull(d types.Datum, sc *stmtctx.StatementContext) (types.Datum, error) {
	if err := c.CheckNotNull(d); err != nil {
		if sc.BadNullAsWarning {
			sc.AppendWarning(err)
			return GetZeroValue(c.ToInfo()), nil
		}
		return types.Datum{}, errors.Trace(err)
	}
	return d, nil
}

// IsPKHandleColumn checks if the column is primary key handle column.
func (c *Column) IsPKHandleColumn(tbInfo *model.TableInfo) bool {
	return mysql.HasPriKeyFlag(c.Flag) && tbInfo.PKIsHandle
}

// CheckNotNull checks if row has nil value set to a column with NotNull flag set.
func CheckNotNull(cols []*Column, row []types.Datum) error {
	for _, c := range cols {
		if err := c.CheckNotNull(row[c.Offset]); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// GetColOriginDefaultValue gets default value of the column from original default value.
func GetColOriginDefaultValue(ctx sessionctx.Context, col *model.ColumnInfo) (types.Datum, error) {
	return getColDefaultValue(ctx, col, col.OriginDefaultValue)
}

// GetColDefaultValue gets default value of the column.
func GetColDefaultValue(ctx sessionctx.Context, col *model.ColumnInfo) (types.Datum, error) {
	return getColDefaultValue(ctx, col, col.GetDefaultValue())
}

func getColDefaultValue(ctx sessionctx.Context, col *model.ColumnInfo, defaultVal interface{}) (types.Datum, error) {
	if defaultVal == nil {
		return getColDefaultValueFromNil(ctx, col)
	}

	// Check and get timestamp/datetime default value.
	if col.Tp == mysql.TypeTimestamp || col.Tp == mysql.TypeDatetime {
		value, err := expression.GetTimeValue(ctx, defaultVal, col.Tp, col.Decimal)
		if err != nil {
			return types.Datum{}, errGetDefaultFailed.GenWithStack("Field '%s' get default value fail - %s",
				col.Name, errors.Trace(err))
		}
		return value, nil
	}
	value, err := CastValue(ctx, types.NewDatum(defaultVal), col)
	if err != nil {
		return types.Datum{}, errors.Trace(err)
	}
	return value, nil
}

func getColDefaultValueFromNil(ctx sessionctx.Context, col *model.ColumnInfo) (types.Datum, error) {
	if !mysql.HasNotNullFlag(col.Flag) {
		return types.Datum{}, nil
	}
	if col.Tp == mysql.TypeEnum {
		// For enum type, if no default value and not null is set,
		// the default value is the first element of the enum list
		defEnum, err := types.ParseEnumValue(col.FieldType.Elems, 1)
		if err != nil {
			return types.Datum{}, err
		}
		return types.NewMysqlEnumDatum(defEnum), nil
	}
	if mysql.HasAutoIncrementFlag(col.Flag) {
		// Auto increment column doesn't has default value and we should not return error.
		return GetZeroValue(col), nil
	}
	if col.IsGenerated() {
		return types.Datum{}, nil
	}
	sc := ctx.GetSessionVars().StmtCtx
	if sc.BadNullAsWarning {
		sc.AppendWarning(ErrColumnCantNull.GenWithStackByArgs(col.Name))
		return GetZeroValue(col), nil
	}
	return types.Datum{}, ErrNoDefaultValue.GenWithStack("Field '%s' doesn't have a default value", col.Name)
}

// GetZeroValue gets zero value for given column type.
func GetZeroValue(col *model.ColumnInfo) types.Datum {
	var d types.Datum
	switch col.Tp {
	case mysql.TypeTiny, mysql.TypeInt24, mysql.TypeShort, mysql.TypeLong, mysql.TypeLonglong, mysql.TypeYear:
		if mysql.HasUnsignedFlag(col.Flag) {
			d.SetUint64(0)
		} else {
			d.SetInt64(0)
		}
	case mysql.TypeFloat:
		d.SetFloat32(0)
	case mysql.TypeDouble:
		d.SetFloat64(0)
	case mysql.TypeNewDecimal:
		d.SetMysqlDecimal(new(types.MyDecimal))
	case mysql.TypeString:
		if col.Flen > 0 && col.Charset == charset.CharsetBin {
			d.SetBytes(make([]byte, col.Flen))
		} else {
			d.SetString("")
		}
	case mysql.TypeVarString, mysql.TypeVarchar:
		d.SetString("")
	case mysql.TypeBlob, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeLongBlob:
		d.SetBytes([]byte{})
	case mysql.TypeDuration:
		d.SetMysqlDuration(types.ZeroDuration)
	case mysql.TypeDate:
		d.SetMysqlTime(types.ZeroDate)
	case mysql.TypeTimestamp:
		d.SetMysqlTime(types.ZeroTimestamp)
	case mysql.TypeDatetime:
		d.SetMysqlTime(types.ZeroDatetime)
	case mysql.TypeBit:
		d.SetMysqlBit(types.ZeroBinaryLiteral)
	case mysql.TypeSet:
		d.SetMysqlSet(types.Set{})
	case mysql.TypeEnum:
		d.SetMysqlEnum(types.Enum{})
	case mysql.TypeJSON:
		d.SetMysqlJSON(json.CreateBinary(nil))
	}
	return d
}
