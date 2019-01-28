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
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/hack"
	log "github.com/sirupsen/logrus"
)

// Kind constants.
const (
	KindNull          byte = 0
	KindInt64         byte = 1
	KindUint64        byte = 2
	KindFloat32       byte = 3
	KindFloat64       byte = 4
	KindString        byte = 5
	KindBytes         byte = 6
	KindBinaryLiteral byte = 7 // Used for BIT / HEX literals.
	KindMysqlDecimal  byte = 8
	KindMysqlDuration byte = 9
	KindMysqlEnum     byte = 10
	KindMysqlBit      byte = 11 // Used for BIT table column values.
	KindMysqlSet      byte = 12
	KindMysqlTime     byte = 13
	KindInterface     byte = 14
	KindMinNotNull    byte = 15
	KindMaxValue      byte = 16
	KindRaw           byte = 17
	KindMysqlJSON     byte = 18
)

// Datum is a data box holds different kind of data.
// It has better performance and is easier to use than `interface{}`.
type Datum struct {
	k         byte        // datum kind.
	collation uint8       // collation can hold uint8 values.
	decimal   uint16      // decimal can hold uint16 values.
	length    uint32      // length can hold uint32 values.
	i         int64       // i can hold int64 uint64 float64 values.
	b         []byte      // b can hold string or []byte values.
	x         interface{} // x hold all other types.
}

// Copy deep copies a Datum.
func (d *Datum) Copy() *Datum {
	ret := *d
	if d.b != nil {
		ret.b = make([]byte, len(d.b))
		copy(ret.b, d.b)
	}
	switch ret.Kind() {
	case KindMysqlDecimal:
		d := *d.GetMysqlDecimal()
		ret.SetMysqlDecimal(&d)
	case KindMysqlTime:
		ret.SetMysqlTime(d.GetMysqlTime())
	}
	return &ret
}

// Kind gets the kind of the datum.
func (d *Datum) Kind() byte {
	return d.k
}

// Collation gets the collation of the datum.
func (d *Datum) Collation() byte {
	return d.collation
}

// SetCollation sets the collation of the datum.
func (d *Datum) SetCollation(collation byte) {
	d.collation = collation
}

// Frac gets the frac of the datum.
func (d *Datum) Frac() int {
	return int(d.decimal)
}

// SetFrac sets the frac of the datum.
func (d *Datum) SetFrac(frac int) {
	d.decimal = uint16(frac)
}

// Length gets the length of the datum.
func (d *Datum) Length() int {
	return int(d.length)
}

// SetLength sets the length of the datum.
func (d *Datum) SetLength(l int) {
	d.length = uint32(l)
}

// IsNull checks if datum is null.
func (d *Datum) IsNull() bool {
	return d.k == KindNull
}

// GetInt64 gets int64 value.
func (d *Datum) GetInt64() int64 {
	return d.i
}

// SetInt64 sets int64 value.
func (d *Datum) SetInt64(i int64) {
	d.k = KindInt64
	d.i = i
}

// GetUint64 gets uint64 value.
func (d *Datum) GetUint64() uint64 {
	return uint64(d.i)
}

// SetUint64 sets uint64 value.
func (d *Datum) SetUint64(i uint64) {
	d.k = KindUint64
	d.i = int64(i)
}

// GetFloat64 gets float64 value.
func (d *Datum) GetFloat64() float64 {
	return math.Float64frombits(uint64(d.i))
}

// SetFloat64 sets float64 value.
func (d *Datum) SetFloat64(f float64) {
	d.k = KindFloat64
	d.i = int64(math.Float64bits(f))
}

// GetFloat32 gets float32 value.
func (d *Datum) GetFloat32() float32 {
	return float32(math.Float64frombits(uint64(d.i)))
}

// SetFloat32 sets float32 value.
func (d *Datum) SetFloat32(f float32) {
	d.k = KindFloat32
	d.i = int64(math.Float64bits(float64(f)))
}

// GetString gets string value.
func (d *Datum) GetString() string {
	return hack.String(d.b)
}

// SetString sets string value.
func (d *Datum) SetString(s string) {
	d.k = KindString
	sink(s)
	d.b = hack.Slice(s)
}

// sink prevents s from being allocated on the stack.
var sink = func(s string) {
}

// GetBytes gets bytes value.
func (d *Datum) GetBytes() []byte {
	return d.b
}

// SetBytes sets bytes value to datum.
func (d *Datum) SetBytes(b []byte) {
	d.k = KindBytes
	d.b = b
}

// SetBytesAsString sets bytes value to datum as string type.
func (d *Datum) SetBytesAsString(b []byte) {
	d.k = KindString
	d.b = b
}

// GetInterface gets interface value.
func (d *Datum) GetInterface() interface{} {
	return d.x
}

// SetInterface sets interface to datum.
func (d *Datum) SetInterface(x interface{}) {
	d.k = KindInterface
	d.x = x
}

// SetNull sets datum to nil.
func (d *Datum) SetNull() {
	d.k = KindNull
	d.x = nil
}

// GetBinaryLiteral gets Bit value
func (d *Datum) GetBinaryLiteral() BinaryLiteral {
	return d.b
}

// GetMysqlBit gets MysqlBit value
func (d *Datum) GetMysqlBit() BinaryLiteral {
	return d.GetBinaryLiteral()
}

// SetBinaryLiteral sets Bit value
func (d *Datum) SetBinaryLiteral(b BinaryLiteral) {
	d.k = KindBinaryLiteral
	d.b = b
}

// SetMysqlBit sets MysqlBit value
func (d *Datum) SetMysqlBit(b BinaryLiteral) {
	d.k = KindMysqlBit
	d.b = b
}

// GetMysqlDecimal gets Decimal value
func (d *Datum) GetMysqlDecimal() *MyDecimal {
	return d.x.(*MyDecimal)
}

// SetMysqlDecimal sets Decimal value
func (d *Datum) SetMysqlDecimal(b *MyDecimal) {
	d.k = KindMysqlDecimal
	d.x = b
}

// GetMysqlDuration gets Duration value
func (d *Datum) GetMysqlDuration() Duration {
	return Duration{Duration: time.Duration(d.i), Fsp: int(d.decimal)}
}

// SetMysqlDuration sets Duration value
func (d *Datum) SetMysqlDuration(b Duration) {
	d.k = KindMysqlDuration
	d.i = int64(b.Duration)
	d.decimal = uint16(b.Fsp)
}

// GetMysqlEnum gets Enum value
func (d *Datum) GetMysqlEnum() Enum {
	return Enum{Value: uint64(d.i), Name: hack.String(d.b)}
}

// SetMysqlEnum sets Enum value
func (d *Datum) SetMysqlEnum(b Enum) {
	d.k = KindMysqlEnum
	d.i = int64(b.Value)
	sink(b.Name)
	d.b = hack.Slice(b.Name)
}

// GetMysqlSet gets Set value
func (d *Datum) GetMysqlSet() Set {
	return Set{Value: uint64(d.i), Name: hack.String(d.b)}
}

// SetMysqlSet sets Set value
func (d *Datum) SetMysqlSet(b Set) {
	d.k = KindMysqlSet
	d.i = int64(b.Value)
	sink(b.Name)
	d.b = hack.Slice(b.Name)
}

// GetMysqlJSON gets json.BinaryJSON value
func (d *Datum) GetMysqlJSON() json.BinaryJSON {
	return json.BinaryJSON{TypeCode: byte(d.i), Value: d.b}
}

// SetMysqlJSON sets json.BinaryJSON value
func (d *Datum) SetMysqlJSON(b json.BinaryJSON) {
	d.k = KindMysqlJSON
	d.i = int64(b.TypeCode)
	d.b = b.Value
}

// GetMysqlTime gets types.Time value
func (d *Datum) GetMysqlTime() Time {
	return d.x.(Time)
}

// SetMysqlTime sets types.Time value
func (d *Datum) SetMysqlTime(b Time) {
	d.k = KindMysqlTime
	d.x = b
}

// SetRaw sets raw value.
func (d *Datum) SetRaw(b []byte) {
	d.k = KindRaw
	d.b = b
}

// GetRaw gets raw value.
func (d *Datum) GetRaw() []byte {
	return d.b
}

// SetAutoID set the auto increment ID according to its int flag.
func (d *Datum) SetAutoID(id int64, flag uint) {
	if mysql.HasUnsignedFlag(flag) {
		d.SetUint64(uint64(id))
	} else {
		d.SetInt64(id)
	}
}

// GetValue gets the value of the datum of any kind.
func (d *Datum) GetValue() interface{} {
	switch d.k {
	case KindInt64:
		return d.GetInt64()
	case KindUint64:
		return d.GetUint64()
	case KindFloat32:
		return d.GetFloat32()
	case KindFloat64:
		return d.GetFloat64()
	case KindString:
		return d.GetString()
	case KindBytes:
		return d.GetBytes()
	case KindMysqlDecimal:
		return d.GetMysqlDecimal()
	case KindMysqlDuration:
		return d.GetMysqlDuration()
	case KindMysqlEnum:
		return d.GetMysqlEnum()
	case KindBinaryLiteral, KindMysqlBit:
		return d.GetBinaryLiteral()
	case KindMysqlSet:
		return d.GetMysqlSet()
	case KindMysqlJSON:
		return d.GetMysqlJSON()
	case KindMysqlTime:
		return d.GetMysqlTime()
	default:
		return d.GetInterface()
	}
}

// SetValue sets any kind of value.
func (d *Datum) SetValue(val interface{}) {
	switch x := val.(type) {
	case nil:
		d.SetNull()
	case bool:
		if x {
			d.SetInt64(1)
		} else {
			d.SetInt64(0)
		}
	case int:
		d.SetInt64(int64(x))
	case int64:
		d.SetInt64(x)
	case uint64:
		d.SetUint64(x)
	case float32:
		d.SetFloat32(x)
	case float64:
		d.SetFloat64(x)
	case string:
		d.SetString(x)
	case []byte:
		d.SetBytes(x)
	case *MyDecimal:
		d.SetMysqlDecimal(x)
	case Duration:
		d.SetMysqlDuration(x)
	case Enum:
		d.SetMysqlEnum(x)
	case BinaryLiteral:
		d.SetBinaryLiteral(x)
	case BitLiteral: // Store as BinaryLiteral for Bit and Hex literals
		d.SetBinaryLiteral(BinaryLiteral(x))
	case HexLiteral:
		d.SetBinaryLiteral(BinaryLiteral(x))
	case Set:
		d.SetMysqlSet(x)
	case json.BinaryJSON:
		d.SetMysqlJSON(x)
	case Time:
		d.SetMysqlTime(x)
	default:
		d.SetInterface(x)
	}
}

// CompareDatum compares datum to another datum.
// TODO: return error properly.
func (d *Datum) CompareDatum(sc *stmtctx.StatementContext, ad *Datum) (int, error) {
	if d.k == KindMysqlJSON && ad.k != KindMysqlJSON {
		cmp, err := ad.CompareDatum(sc, d)
		return cmp * -1, errors.Trace(err)
	}
	switch ad.k {
	case KindNull:
		if d.k == KindNull {
			return 0, nil
		}
		return 1, nil
	case KindMinNotNull:
		if d.k == KindNull {
			return -1, nil
		} else if d.k == KindMinNotNull {
			return 0, nil
		}
		return 1, nil
	case KindMaxValue:
		if d.k == KindMaxValue {
			return 0, nil
		}
		return -1, nil
	case KindInt64:
		return d.compareInt64(sc, ad.GetInt64())
	case KindUint64:
		return d.compareUint64(sc, ad.GetUint64())
	case KindFloat32, KindFloat64:
		return d.compareFloat64(sc, ad.GetFloat64())
	case KindString:
		return d.compareString(sc, ad.GetString())
	case KindBytes:
		return d.compareBytes(sc, ad.GetBytes())
	case KindMysqlDecimal:
		return d.compareMysqlDecimal(sc, ad.GetMysqlDecimal())
	case KindMysqlDuration:
		return d.compareMysqlDuration(sc, ad.GetMysqlDuration())
	case KindMysqlEnum:
		return d.compareMysqlEnum(sc, ad.GetMysqlEnum())
	case KindBinaryLiteral, KindMysqlBit:
		return d.compareBinaryLiteral(sc, ad.GetBinaryLiteral())
	case KindMysqlSet:
		return d.compareMysqlSet(sc, ad.GetMysqlSet())
	case KindMysqlJSON:
		return d.compareMysqlJSON(sc, ad.GetMysqlJSON())
	case KindMysqlTime:
		return d.compareMysqlTime(sc, ad.GetMysqlTime())
	default:
		return 0, nil
	}
}

func (d *Datum) compareInt64(sc *stmtctx.StatementContext, i int64) (int, error) {
	switch d.k {
	case KindMaxValue:
		return 1, nil
	case KindInt64:
		return CompareInt64(d.i, i), nil
	case KindUint64:
		if i < 0 || d.GetUint64() > math.MaxInt64 {
			return 1, nil
		}
		return CompareInt64(d.i, i), nil
	default:
		return d.compareFloat64(sc, float64(i))
	}
}

func (d *Datum) compareUint64(sc *stmtctx.StatementContext, u uint64) (int, error) {
	switch d.k {
	case KindMaxValue:
		return 1, nil
	case KindInt64:
		if d.i < 0 || u > math.MaxInt64 {
			return -1, nil
		}
		return CompareInt64(d.i, int64(u)), nil
	case KindUint64:
		return CompareUint64(d.GetUint64(), u), nil
	default:
		return d.compareFloat64(sc, float64(u))
	}
}

func (d *Datum) compareFloat64(sc *stmtctx.StatementContext, f float64) (int, error) {
	switch d.k {
	case KindNull, KindMinNotNull:
		return -1, nil
	case KindMaxValue:
		return 1, nil
	case KindInt64:
		return CompareFloat64(float64(d.i), f), nil
	case KindUint64:
		return CompareFloat64(float64(d.GetUint64()), f), nil
	case KindFloat32, KindFloat64:
		return CompareFloat64(d.GetFloat64(), f), nil
	case KindString, KindBytes:
		fVal, err := StrToFloat(sc, d.GetString())
		return CompareFloat64(fVal, f), errors.Trace(err)
	case KindMysqlDecimal:
		fVal, err := d.GetMysqlDecimal().ToFloat64()
		return CompareFloat64(fVal, f), errors.Trace(err)
	case KindMysqlDuration:
		fVal := d.GetMysqlDuration().Seconds()
		return CompareFloat64(fVal, f), nil
	case KindMysqlEnum:
		fVal := d.GetMysqlEnum().ToNumber()
		return CompareFloat64(fVal, f), nil
	case KindBinaryLiteral, KindMysqlBit:
		val, err := d.GetBinaryLiteral().ToInt(sc)
		fVal := float64(val)
		return CompareFloat64(fVal, f), errors.Trace(err)
	case KindMysqlSet:
		fVal := d.GetMysqlSet().ToNumber()
		return CompareFloat64(fVal, f), nil
	case KindMysqlTime:
		fVal, err := d.GetMysqlTime().ToNumber().ToFloat64()
		return CompareFloat64(fVal, f), errors.Trace(err)
	default:
		return -1, nil
	}
}

func (d *Datum) compareString(sc *stmtctx.StatementContext, s string) (int, error) {
	switch d.k {
	case KindNull, KindMinNotNull:
		return -1, nil
	case KindMaxValue:
		return 1, nil
	case KindString, KindBytes:
		return CompareString(d.GetString(), s), nil
	case KindMysqlDecimal:
		dec := new(MyDecimal)
		err := sc.HandleTruncate(dec.FromString(hack.Slice(s)))
		return d.GetMysqlDecimal().Compare(dec), errors.Trace(err)
	case KindMysqlTime:
		dt, err := ParseDatetime(sc, s)
		return d.GetMysqlTime().Compare(dt), errors.Trace(err)
	case KindMysqlDuration:
		dur, err := ParseDuration(sc, s, MaxFsp)
		return d.GetMysqlDuration().Compare(dur), errors.Trace(err)
	case KindMysqlSet:
		return CompareString(d.GetMysqlSet().String(), s), nil
	case KindMysqlEnum:
		return CompareString(d.GetMysqlEnum().String(), s), nil
	case KindBinaryLiteral, KindMysqlBit:
		return CompareString(d.GetBinaryLiteral().ToString(), s), nil
	default:
		fVal, err := StrToFloat(sc, s)
		if err != nil {
			return 0, errors.Trace(err)
		}
		return d.compareFloat64(sc, fVal)
	}
}

func (d *Datum) compareBytes(sc *stmtctx.StatementContext, b []byte) (int, error) {
	return d.compareString(sc, hack.String(b))
}

func (d *Datum) compareMysqlDecimal(sc *stmtctx.StatementContext, dec *MyDecimal) (int, error) {
	switch d.k {
	case KindNull, KindMinNotNull:
		return -1, nil
	case KindMaxValue:
		return 1, nil
	case KindMysqlDecimal:
		return d.GetMysqlDecimal().Compare(dec), nil
	case KindString, KindBytes:
		dDec := new(MyDecimal)
		err := sc.HandleTruncate(dDec.FromString(d.GetBytes()))
		return dDec.Compare(dec), errors.Trace(err)
	default:
		dVal, err := d.ConvertTo(sc, NewFieldType(mysql.TypeNewDecimal))
		if err != nil {
			return 0, errors.Trace(err)
		}
		return dVal.GetMysqlDecimal().Compare(dec), nil
	}
}

func (d *Datum) compareMysqlDuration(sc *stmtctx.StatementContext, dur Duration) (int, error) {
	switch d.k {
	case KindMysqlDuration:
		return d.GetMysqlDuration().Compare(dur), nil
	case KindString, KindBytes:
		dDur, err := ParseDuration(sc, d.GetString(), MaxFsp)
		return dDur.Compare(dur), errors.Trace(err)
	default:
		return d.compareFloat64(sc, dur.Seconds())
	}
}

func (d *Datum) compareMysqlEnum(sc *stmtctx.StatementContext, enum Enum) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		return CompareString(d.GetString(), enum.String()), nil
	default:
		return d.compareFloat64(sc, enum.ToNumber())
	}
}

func (d *Datum) compareBinaryLiteral(sc *stmtctx.StatementContext, b BinaryLiteral) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		return CompareString(d.GetString(), b.ToString()), nil
	case KindBinaryLiteral, KindMysqlBit:
		return CompareString(d.GetBinaryLiteral().ToString(), b.ToString()), nil
	default:
		val, err := b.ToInt(sc)
		if err != nil {
			return 0, errors.Trace(err)
		}
		result, err := d.compareFloat64(sc, float64(val))
		return result, errors.Trace(err)
	}
}

func (d *Datum) compareMysqlSet(sc *stmtctx.StatementContext, set Set) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		return CompareString(d.GetString(), set.String()), nil
	default:
		return d.compareFloat64(sc, set.ToNumber())
	}
}

func (d *Datum) compareMysqlJSON(sc *stmtctx.StatementContext, target json.BinaryJSON) (int, error) {
	origin, err := d.ToMysqlJSON()
	if err != nil {
		return 0, errors.Trace(err)
	}
	return json.CompareBinary(origin, target), nil
}

func (d *Datum) compareMysqlTime(sc *stmtctx.StatementContext, time Time) (int, error) {
	switch d.k {
	case KindString, KindBytes:
		dt, err := ParseDatetime(sc, d.GetString())
		return dt.Compare(time), errors.Trace(err)
	case KindMysqlTime:
		return d.GetMysqlTime().Compare(time), nil
	default:
		fVal, err := time.ToNumber().ToFloat64()
		if err != nil {
			return 0, errors.Trace(err)
		}
		return d.compareFloat64(sc, fVal)
	}
}

// ConvertTo converts a datum to the target field type.
func (d *Datum) ConvertTo(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	if d.k == KindNull {
		return Datum{}, nil
	}
	switch target.Tp { // TODO: implement mysql types convert when "CAST() AS" syntax are supported.
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong:
		unsigned := mysql.HasUnsignedFlag(target.Flag)
		if unsigned {
			return d.convertToUint(sc, target)
		}
		return d.convertToInt(sc, target)
	case mysql.TypeFloat, mysql.TypeDouble:
		return d.convertToFloat(sc, target)
	case mysql.TypeBlob, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeLongBlob,
		mysql.TypeString, mysql.TypeVarchar, mysql.TypeVarString:
		return d.convertToString(sc, target)
	case mysql.TypeTimestamp:
		return d.convertToMysqlTimestamp(sc, target)
	case mysql.TypeDatetime, mysql.TypeDate:
		return d.convertToMysqlTime(sc, target)
	case mysql.TypeDuration:
		return d.convertToMysqlDuration(sc, target)
	case mysql.TypeNewDecimal:
		return d.convertToMysqlDecimal(sc, target)
	case mysql.TypeYear:
		return d.convertToMysqlYear(sc, target)
	case mysql.TypeEnum:
		return d.convertToMysqlEnum(sc, target)
	case mysql.TypeBit:
		return d.convertToMysqlBit(sc, target)
	case mysql.TypeSet:
		return d.convertToMysqlSet(sc, target)
	case mysql.TypeJSON:
		return d.convertToMysqlJSON(sc, target)
	case mysql.TypeNull:
		return Datum{}, nil
	default:
		panic("should never happen")
	}
}

func (d *Datum) convertToFloat(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var (
		f   float64
		ret Datum
		err error
	)
	switch d.k {
	case KindNull:
		return ret, nil
	case KindInt64:
		f = float64(d.GetInt64())
	case KindUint64:
		f = float64(d.GetUint64())
	case KindFloat32, KindFloat64:
		f = d.GetFloat64()
	case KindString, KindBytes:
		f, err = StrToFloat(sc, d.GetString())
	case KindMysqlTime:
		f, err = d.GetMysqlTime().ToNumber().ToFloat64()
	case KindMysqlDuration:
		f, err = d.GetMysqlDuration().ToNumber().ToFloat64()
	case KindMysqlDecimal:
		f, err = d.GetMysqlDecimal().ToFloat64()
	case KindMysqlSet:
		f = d.GetMysqlSet().ToNumber()
	case KindMysqlEnum:
		f = d.GetMysqlEnum().ToNumber()
	case KindBinaryLiteral, KindMysqlBit:
		val, err1 := d.GetBinaryLiteral().ToInt(sc)
		f, err = float64(val), err1
	case KindMysqlJSON:
		f, err = ConvertJSONToFloat(sc, d.GetMysqlJSON())
	default:
		return invalidConv(d, target.Tp)
	}
	var err1 error
	f, err1 = ProduceFloatWithSpecifiedTp(f, target, sc)
	if err == nil && err1 != nil {
		err = err1
	}
	if target.Tp == mysql.TypeFloat {
		ret.SetFloat32(float32(f))
	} else {
		ret.SetFloat64(f)
	}
	return ret, errors.Trace(err)
}

// ProduceFloatWithSpecifiedTp produces a new float64 according to `flen` and `decimal`.
func ProduceFloatWithSpecifiedTp(f float64, target *FieldType, sc *stmtctx.StatementContext) (_ float64, err error) {
	// For float and following double type, we will only truncate it for float(M, D) format.
	// If no D is set, we will handle it like origin float whether M is set or not.
	if target.Flen != UnspecifiedLength && target.Decimal != UnspecifiedLength {
		f, err = TruncateFloat(f, target.Flen, target.Decimal)
		if err = sc.HandleOverflow(err, err); err != nil {
			return f, errors.Trace(err)
		}
	}
	if mysql.HasUnsignedFlag(target.Flag) && f < 0 {
		return 0, overflow(f, target.Tp)
	}
	return f, nil
}

func (d *Datum) convertToString(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var ret Datum
	var s string
	switch d.k {
	case KindInt64:
		s = strconv.FormatInt(d.GetInt64(), 10)
	case KindUint64:
		s = strconv.FormatUint(d.GetUint64(), 10)
	case KindFloat32:
		s = strconv.FormatFloat(d.GetFloat64(), 'f', -1, 32)
	case KindFloat64:
		s = strconv.FormatFloat(d.GetFloat64(), 'f', -1, 64)
	case KindString, KindBytes:
		s = d.GetString()
	case KindMysqlTime:
		s = d.GetMysqlTime().String()
	case KindMysqlDuration:
		s = d.GetMysqlDuration().String()
	case KindMysqlDecimal:
		s = d.GetMysqlDecimal().String()
	case KindMysqlEnum:
		s = d.GetMysqlEnum().String()
	case KindMysqlSet:
		s = d.GetMysqlSet().String()
	case KindBinaryLiteral, KindMysqlBit:
		s = d.GetBinaryLiteral().ToString()
	case KindMysqlJSON:
		s = d.GetMysqlJSON().String()
	default:
		return invalidConv(d, target.Tp)
	}
	s, err := ProduceStrWithSpecifiedTp(s, target, sc)
	ret.SetString(s)
	if target.Charset == charset.CharsetBin {
		ret.k = KindBytes
	}
	return ret, errors.Trace(err)
}

// ProduceStrWithSpecifiedTp produces a new string according to `flen` and `chs`.
func ProduceStrWithSpecifiedTp(s string, tp *FieldType, sc *stmtctx.StatementContext) (_ string, err error) {
	flen, chs := tp.Flen, tp.Charset
	if flen >= 0 {
		// Flen is the rune length, not binary length, for UTF8 charset, we need to calculate the
		// rune count and truncate to Flen runes if it is too long.
		if chs == charset.CharsetUTF8 || chs == charset.CharsetUTF8MB4 {
			characterLen := utf8.RuneCountInString(s)
			if characterLen > flen {
				// 1. If len(s) is 0 and flen is 0, truncateLen will be 0, don't truncate s.
				//    CREATE TABLE t (a char(0));
				//    INSERT INTO t VALUES (``);
				// 2. If len(s) is 10 and flen is 0, truncateLen will be 0 too, but we still need to truncate s.
				//    SELECT 1, CAST(1234 AS CHAR(0));
				// So truncateLen is not a suitable variable to determine to do truncate or not.
				var runeCount int
				var truncateLen int
				for i := range s {
					if runeCount == flen {
						truncateLen = i
						break
					}
					runeCount++
				}
				err = ErrDataTooLong.GenWithStack("Data Too Long, field len %d, data len %d", flen, characterLen)
				s = truncateStr(s, truncateLen)
			}
		} else if len(s) > flen {
			err = ErrDataTooLong.GenWithStack("Data Too Long, field len %d, data len %d", flen, len(s))
			s = truncateStr(s, flen)
		} else if tp.Tp == mysql.TypeString && IsBinaryStr(tp) && len(s) < flen {
			padding := make([]byte, flen-len(s))
			s = string(append([]byte(s), padding...))
		}
	}
	return s, errors.Trace(sc.HandleTruncate(err))
}

func (d *Datum) convertToInt(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	i64, err := d.toSignedInteger(sc, target.Tp)
	return NewIntDatum(i64), errors.Trace(err)
}

func (d *Datum) convertToUint(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	tp := target.Tp
	upperBound := UnsignedUpperBound[tp]
	var (
		val uint64
		err error
		ret Datum
	)
	switch d.k {
	case KindInt64:
		val, err = ConvertIntToUint(d.GetInt64(), upperBound, tp)
	case KindUint64:
		val, err = ConvertUintToUint(d.GetUint64(), upperBound, tp)
	case KindFloat32, KindFloat64:
		val, err = ConvertFloatToUint(d.GetFloat64(), upperBound, tp)
	case KindString, KindBytes:
		val, err = StrToUint(sc, d.GetString())
		if err != nil {
			return ret, errors.Trace(err)
		}
		val, err = ConvertUintToUint(val, upperBound, tp)
		if err != nil {
			return ret, errors.Trace(err)
		}
		ret.SetUint64(val)
	case KindMysqlTime:
		dec := d.GetMysqlTime().ToNumber()
		err = dec.Round(dec, 0, ModeHalfEven)
		ival, err1 := dec.ToInt()
		if err == nil {
			err = err1
		}
		val, err1 = ConvertIntToUint(ival, upperBound, tp)
		if err == nil {
			err = err1
		}
	case KindMysqlDuration:
		dec := d.GetMysqlDuration().ToNumber()
		err = dec.Round(dec, 0, ModeHalfEven)
		ival, err1 := dec.ToInt()
		if err1 == nil {
			val, err = ConvertIntToUint(ival, upperBound, tp)
		}
	case KindMysqlDecimal:
		fval, err1 := d.GetMysqlDecimal().ToFloat64()
		val, err = ConvertFloatToUint(fval, upperBound, tp)
		if err == nil {
			err = err1
		}
	case KindMysqlEnum:
		val, err = ConvertFloatToUint(d.GetMysqlEnum().ToNumber(), upperBound, tp)
	case KindMysqlSet:
		val, err = ConvertFloatToUint(d.GetMysqlSet().ToNumber(), upperBound, tp)
	case KindBinaryLiteral, KindMysqlBit:
		val, err = d.GetBinaryLiteral().ToInt(sc)
	case KindMysqlJSON:
		var i64 int64
		i64, err = ConvertJSONToInt(sc, d.GetMysqlJSON(), true)
		val = uint64(i64)
	default:
		return invalidConv(d, target.Tp)
	}
	ret.SetUint64(val)
	if err != nil {
		return ret, errors.Trace(err)
	}
	return ret, nil
}

func (d *Datum) convertToMysqlTimestamp(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var (
		ret Datum
		t   Time
		err error
	)
	fsp := DefaultFsp
	if target.Decimal != UnspecifiedLength {
		fsp = target.Decimal
	}
	switch d.k {
	case KindMysqlTime:
		t = d.GetMysqlTime()
		t, err = t.RoundFrac(sc, fsp)
	case KindMysqlDuration:
		t, err = d.GetMysqlDuration().ConvertToTime(sc, mysql.TypeTimestamp)
		if err != nil {
			ret.SetValue(t)
			return ret, errors.Trace(err)
		}
		t, err = t.RoundFrac(sc, fsp)
	case KindString, KindBytes:
		t, err = ParseTime(sc, d.GetString(), mysql.TypeTimestamp, fsp)
	case KindInt64:
		t, err = ParseTimeFromNum(sc, d.GetInt64(), mysql.TypeTimestamp, fsp)
	default:
		return invalidConv(d, mysql.TypeTimestamp)
	}
	t.Type = mysql.TypeTimestamp
	ret.SetMysqlTime(t)
	if err != nil {
		return ret, errors.Trace(err)
	}
	return ret, nil
}

func (d *Datum) convertToMysqlTime(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	tp := target.Tp
	fsp := DefaultFsp
	if target.Decimal != UnspecifiedLength {
		fsp = target.Decimal
	}
	var (
		ret Datum
		t   Time
		err error
	)
	switch d.k {
	case KindMysqlTime:
		t, err = d.GetMysqlTime().Convert(sc, tp)
		if err != nil {
			ret.SetValue(t)
			return ret, errors.Trace(err)
		}
		t, err = t.RoundFrac(sc, fsp)
	case KindMysqlDuration:
		t, err = d.GetMysqlDuration().ConvertToTime(sc, tp)
		if err != nil {
			ret.SetValue(t)
			return ret, errors.Trace(err)
		}
		t, err = t.RoundFrac(sc, fsp)
	case KindString, KindBytes:
		t, err = ParseTime(sc, d.GetString(), tp, fsp)
	case KindInt64:
		t, err = ParseTimeFromNum(sc, d.GetInt64(), tp, fsp)
	default:
		return invalidConv(d, tp)
	}
	if tp == mysql.TypeDate {
		// Truncate hh:mm:ss part if the type is Date.
		t.Time = FromDate(t.Time.Year(), t.Time.Month(), t.Time.Day(), 0, 0, 0, 0)
	}
	ret.SetValue(t)
	if err != nil {
		return ret, errors.Trace(err)
	}
	return ret, nil
}

func (d *Datum) convertToMysqlDuration(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	tp := target.Tp
	fsp := DefaultFsp
	if target.Decimal != UnspecifiedLength {
		fsp = target.Decimal
	}
	var ret Datum
	switch d.k {
	case KindMysqlTime:
		dur, err := d.GetMysqlTime().ConvertToDuration()
		if err != nil {
			ret.SetValue(dur)
			return ret, errors.Trace(err)
		}
		dur, err = dur.RoundFrac(fsp)
		ret.SetValue(dur)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindMysqlDuration:
		dur, err := d.GetMysqlDuration().RoundFrac(fsp)
		ret.SetValue(dur)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindInt64, KindFloat32, KindFloat64, KindMysqlDecimal:
		// TODO: We need a ParseDurationFromNum to avoid the cost of converting a num to string.
		timeStr, err := d.ToString()
		if err != nil {
			return ret, errors.Trace(err)
		}
		timeNum, err := d.ToInt64(sc)
		if err != nil {
			return ret, errors.Trace(err)
		}
		// For huge numbers(>'0001-00-00 00-00-00') try full DATETIME in ParseDuration.
		if timeNum > MaxDuration && timeNum < 10000000000 {
			// mysql return max in no strict sql mode.
			ret.SetValue(Duration{Duration: MaxTime, Fsp: 0})
			return ret, ErrInvalidTimeFormat.GenWithStack("Incorrect time value: '%s'", timeStr)
		}
		if timeNum < -MaxDuration {
			return ret, ErrInvalidTimeFormat.GenWithStack("Incorrect time value: '%s'", timeStr)
		}
		t, err := ParseDuration(sc, timeStr, fsp)
		ret.SetValue(t)
		if err != nil {
			return ret, errors.Trace(err)
		}
	case KindString, KindBytes:
		t, err := ParseDuration(sc, d.GetString(), fsp)
		ret.SetValue(t)
		if err != nil {
			return ret, errors.Trace(err)
		}
	default:
		return invalidConv(d, tp)
	}
	return ret, nil
}

func (d *Datum) convertToMysqlDecimal(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var ret Datum
	ret.SetLength(target.Flen)
	ret.SetFrac(target.Decimal)
	var dec = &MyDecimal{}
	var err error
	switch d.k {
	case KindInt64:
		dec.FromInt(d.GetInt64())
	case KindUint64:
		dec.FromUint(d.GetUint64())
	case KindFloat32, KindFloat64:
		err = dec.FromFloat64(d.GetFloat64())
	case KindString, KindBytes:
		err = dec.FromString(d.GetBytes())
	case KindMysqlDecimal:
		*dec = *d.GetMysqlDecimal()
	case KindMysqlTime:
		dec = d.GetMysqlTime().ToNumber()
	case KindMysqlDuration:
		dec = d.GetMysqlDuration().ToNumber()
	case KindMysqlEnum:
		err = dec.FromFloat64(d.GetMysqlEnum().ToNumber())
	case KindMysqlSet:
		err = dec.FromFloat64(d.GetMysqlSet().ToNumber())
	case KindBinaryLiteral, KindMysqlBit:
		val, err1 := d.GetBinaryLiteral().ToInt(sc)
		err = err1
		dec.FromUint(val)
	case KindMysqlJSON:
		f, err1 := ConvertJSONToFloat(sc, d.GetMysqlJSON())
		if err1 != nil {
			return ret, errors.Trace(err1)
		}
		err = dec.FromFloat64(f)
	default:
		return invalidConv(d, target.Tp)
	}
	var err1 error
	dec, err1 = ProduceDecWithSpecifiedTp(dec, target, sc)
	if err == nil && err1 != nil {
		err = err1
	}
	if dec.negative && mysql.HasUnsignedFlag(target.Flag) {
		*dec = zeroMyDecimal
		if err == nil {
			err = ErrOverflow.GenWithStackByArgs("DECIMAL", fmt.Sprintf("(%d, %d)", target.Flen, target.Decimal))
		}
	}
	ret.SetValue(dec)
	return ret, errors.Trace(err)
}

// ProduceDecWithSpecifiedTp produces a new decimal according to `flen` and `decimal`.
func ProduceDecWithSpecifiedTp(dec *MyDecimal, tp *FieldType, sc *stmtctx.StatementContext) (_ *MyDecimal, err error) {
	flen, decimal := tp.Flen, tp.Decimal
	if flen != UnspecifiedLength && decimal != UnspecifiedLength {
		if flen < decimal {
			return nil, ErrMBiggerThanD.GenWithStackByArgs("")
		}
		prec, frac := dec.PrecisionAndFrac()
		if !dec.IsZero() && prec-frac > flen-decimal {
			dec = NewMaxOrMinDec(dec.IsNegative(), flen, decimal)
			// select (cast 111 as decimal(1)) causes a warning in MySQL.
			err = ErrOverflow.GenWithStackByArgs("DECIMAL", fmt.Sprintf("(%d, %d)", flen, decimal))
		} else if frac != decimal {
			old := *dec
			err = dec.Round(dec, decimal, ModeHalfEven)
			if err != nil {
				return nil, errors.Trace(err)
			}
			if !dec.IsZero() && frac > decimal && dec.Compare(&old) != 0 {
				if sc.InInsertStmt || sc.InUpdateOrDeleteStmt {
					// fix https://github.com/pingcap/tidb/issues/3895
					// fix https://github.com/pingcap/tidb/issues/5532
					sc.AppendWarning(ErrTruncated)
					err = nil
				} else {
					err = sc.HandleTruncate(ErrTruncated)
				}
			}
		}
	}

	if ErrOverflow.Equal(err) {
		// TODO: warnErr need to be ErrWarnDataOutOfRange
		err = sc.HandleOverflow(err, err)
	}
	unsigned := mysql.HasUnsignedFlag(tp.Flag)
	if unsigned && dec.IsNegative() {
		dec = dec.FromUint(0)
	}
	return dec, errors.Trace(err)
}

func (d *Datum) convertToMysqlYear(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var (
		ret     Datum
		y       int64
		err     error
		fromStr bool
	)
	switch d.k {
	case KindString, KindBytes:
		y, err = StrToInt(sc, d.GetString())
		if err != nil {
			return ret, errors.Trace(err)
		}
		fromStr = true
	case KindMysqlTime:
		y = int64(d.GetMysqlTime().Time.Year())
	case KindMysqlDuration:
		y = int64(time.Now().Year())
	default:
		ret, err = d.convertToInt(sc, NewFieldType(mysql.TypeLonglong))
		if err != nil {
			return invalidConv(d, target.Tp)
		}
		y = ret.GetInt64()
	}
	y, err = AdjustYear(y, fromStr)
	if err != nil {
		return invalidConv(d, target.Tp)
	}
	ret.SetInt64(y)
	return ret, nil
}

func (d *Datum) convertToMysqlBit(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var ret Datum
	var uintValue uint64
	var err error
	switch d.k {
	case KindString, KindBytes:
		uintValue, err = BinaryLiteral(d.b).ToInt(sc)
	default:
		uintDatum, err1 := d.convertToUint(sc, target)
		uintValue, err = uintDatum.GetUint64(), err1
	}
	if target.Flen < 64 && uintValue >= 1<<(uint64(target.Flen)) {
		return Datum{}, errors.Trace(ErrOverflow.GenWithStackByArgs("BIT", fmt.Sprintf("(%d)", target.Flen)))
	}
	byteSize := (target.Flen + 7) >> 3
	ret.SetMysqlBit(NewBinaryLiteralFromUint(uintValue, byteSize))
	return ret, errors.Trace(err)
}

func (d *Datum) convertToMysqlEnum(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var (
		ret Datum
		e   Enum
		err error
	)
	switch d.k {
	case KindString, KindBytes:
		e, err = ParseEnumName(target.Elems, d.GetString())
	default:
		var uintDatum Datum
		uintDatum, err = d.convertToUint(sc, target)
		if err != nil {
			return ret, errors.Trace(err)
		}
		e, err = ParseEnumValue(target.Elems, uintDatum.GetUint64())
	}
	if err != nil {
		log.Error(err)
		err = errors.Trace(ErrTruncated)
	}
	ret.SetValue(e)
	return ret, err
}

func (d *Datum) convertToMysqlSet(sc *stmtctx.StatementContext, target *FieldType) (Datum, error) {
	var (
		ret Datum
		s   Set
		err error
	)
	switch d.k {
	case KindString, KindBytes:
		s, err = ParseSetName(target.Elems, d.GetString())
	default:
		var uintDatum Datum
		uintDatum, err = d.convertToUint(sc, target)
		if err != nil {
			return ret, errors.Trace(err)
		}
		s, err = ParseSetValue(target.Elems, uintDatum.GetUint64())
	}

	if err != nil {
		return invalidConv(d, target.Tp)
	}
	ret.SetValue(s)
	return ret, nil
}

func (d *Datum) convertToMysqlJSON(sc *stmtctx.StatementContext, target *FieldType) (ret Datum, err error) {
	switch d.k {
	case KindString, KindBytes:
		var j json.BinaryJSON
		if j, err = json.ParseBinaryFromString(d.GetString()); err == nil {
			ret.SetMysqlJSON(j)
		}
	case KindInt64:
		i64 := d.GetInt64()
		ret.SetMysqlJSON(json.CreateBinary(i64))
	case KindUint64:
		u64 := d.GetUint64()
		ret.SetMysqlJSON(json.CreateBinary(u64))
	case KindFloat32, KindFloat64:
		f64 := d.GetFloat64()
		ret.SetMysqlJSON(json.CreateBinary(f64))
	case KindMysqlDecimal:
		var f64 float64
		if f64, err = d.GetMysqlDecimal().ToFloat64(); err == nil {
			ret.SetMysqlJSON(json.CreateBinary(f64))
		}
	case KindMysqlJSON:
		ret = *d
	default:
		var s string
		if s, err = d.ToString(); err == nil {
			// TODO: fix precision of MysqlTime. For example,
			// On MySQL 5.7 CAST(NOW() AS JSON) -> "2011-11-11 11:11:11.111111",
			// But now we can only return "2011-11-11 11:11:11".
			ret.SetMysqlJSON(json.CreateBinary(s))
		}
	}
	return ret, errors.Trace(err)
}

// ToBool converts to a bool.
// We will use 1 for true, and 0 for false.
func (d *Datum) ToBool(sc *stmtctx.StatementContext) (int64, error) {
	var err error
	isZero := false
	switch d.Kind() {
	case KindInt64:
		isZero = d.GetInt64() == 0
	case KindUint64:
		isZero = d.GetUint64() == 0
	case KindFloat32:
		isZero = RoundFloat(d.GetFloat64()) == 0
	case KindFloat64:
		isZero = RoundFloat(d.GetFloat64()) == 0
	case KindString, KindBytes:
		iVal, err1 := StrToInt(sc, d.GetString())
		isZero, err = iVal == 0, err1
	case KindMysqlTime:
		isZero = d.GetMysqlTime().IsZero()
	case KindMysqlDuration:
		isZero = d.GetMysqlDuration().Duration == 0
	case KindMysqlDecimal:
		v, err1 := d.GetMysqlDecimal().ToFloat64()
		isZero, err = RoundFloat(v) == 0, err1
	case KindMysqlEnum:
		isZero = d.GetMysqlEnum().ToNumber() == 0
	case KindMysqlSet:
		isZero = d.GetMysqlSet().ToNumber() == 0
	case KindBinaryLiteral, KindMysqlBit:
		val, err1 := d.GetBinaryLiteral().ToInt(sc)
		isZero, err = val == 0, err1
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to bool", d.GetValue(), d.GetValue())
	}
	var ret int64
	if isZero {
		ret = 0
	} else {
		ret = 1
	}
	if err != nil {
		return ret, errors.Trace(err)
	}
	return ret, nil
}

// ConvertDatumToDecimal converts datum to decimal.
func ConvertDatumToDecimal(sc *stmtctx.StatementContext, d Datum) (*MyDecimal, error) {
	dec := new(MyDecimal)
	var err error
	switch d.Kind() {
	case KindInt64:
		dec.FromInt(d.GetInt64())
	case KindUint64:
		dec.FromUint(d.GetUint64())
	case KindFloat32:
		err = dec.FromFloat64(float64(d.GetFloat32()))
	case KindFloat64:
		err = dec.FromFloat64(d.GetFloat64())
	case KindString:
		err = sc.HandleTruncate(dec.FromString(d.GetBytes()))
	case KindMysqlDecimal:
		*dec = *d.GetMysqlDecimal()
	case KindMysqlEnum:
		dec.FromUint(d.GetMysqlEnum().Value)
	case KindMysqlSet:
		dec.FromUint(d.GetMysqlSet().Value)
	case KindBinaryLiteral, KindMysqlBit:
		val, err1 := d.GetBinaryLiteral().ToInt(sc)
		dec.FromUint(val)
		err = err1
	case KindMysqlJSON:
		f, err1 := ConvertJSONToFloat(sc, d.GetMysqlJSON())
		if err1 != nil {
			return nil, errors.Trace(err1)
		}
		err = dec.FromFloat64(f)
	default:
		err = fmt.Errorf("can't convert %v to decimal", d.GetValue())
	}
	return dec, errors.Trace(err)
}

// ToDecimal converts to a decimal.
func (d *Datum) ToDecimal(sc *stmtctx.StatementContext) (*MyDecimal, error) {
	switch d.Kind() {
	case KindMysqlTime:
		return d.GetMysqlTime().ToNumber(), nil
	case KindMysqlDuration:
		return d.GetMysqlDuration().ToNumber(), nil
	default:
		return ConvertDatumToDecimal(sc, *d)
	}
}

// ToInt64 converts to a int64.
func (d *Datum) ToInt64(sc *stmtctx.StatementContext) (int64, error) {
	return d.toSignedInteger(sc, mysql.TypeLonglong)
}

func (d *Datum) toSignedInteger(sc *stmtctx.StatementContext, tp byte) (int64, error) {
	lowerBound := SignedLowerBound[tp]
	upperBound := SignedUpperBound[tp]
	switch d.Kind() {
	case KindInt64:
		return ConvertIntToInt(d.GetInt64(), lowerBound, upperBound, tp)
	case KindUint64:
		return ConvertUintToInt(d.GetUint64(), upperBound, tp)
	case KindFloat32:
		return ConvertFloatToInt(float64(d.GetFloat32()), lowerBound, upperBound, tp)
	case KindFloat64:
		return ConvertFloatToInt(d.GetFloat64(), lowerBound, upperBound, tp)
	case KindString, KindBytes:
		iVal, err := StrToInt(sc, d.GetString())
		iVal, err2 := ConvertIntToInt(iVal, lowerBound, upperBound, tp)
		if err == nil {
			err = err2
		}
		return iVal, errors.Trace(err)
	case KindMysqlTime:
		// 2011-11-10 11:11:11.999999 -> 20111110111112
		// 2011-11-10 11:59:59.999999 -> 20111110120000
		t, err := d.GetMysqlTime().RoundFrac(sc, DefaultFsp)
		if err != nil {
			return 0, errors.Trace(err)
		}
		ival, err := t.ToNumber().ToInt()
		ival, err2 := ConvertIntToInt(ival, lowerBound, upperBound, tp)
		if err == nil {
			err = err2
		}
		return ival, errors.Trace(err)
	case KindMysqlDuration:
		// 11:11:11.999999 -> 111112
		// 11:59:59.999999 -> 120000
		dur, err := d.GetMysqlDuration().RoundFrac(DefaultFsp)
		if err != nil {
			return 0, errors.Trace(err)
		}
		ival, err := dur.ToNumber().ToInt()
		ival, err2 := ConvertIntToInt(ival, lowerBound, upperBound, tp)
		if err == nil {
			err = err2
		}
		return ival, errors.Trace(err)
	case KindMysqlDecimal:
		var to MyDecimal
		err := d.GetMysqlDecimal().Round(&to, 0, ModeHalfEven)
		ival, err1 := to.ToInt()
		if err == nil {
			err = err1
		}
		ival, err2 := ConvertIntToInt(ival, lowerBound, upperBound, tp)
		if err == nil {
			err = err2
		}
		return ival, errors.Trace(err)
	case KindMysqlEnum:
		fval := d.GetMysqlEnum().ToNumber()
		return ConvertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlSet:
		fval := d.GetMysqlSet().ToNumber()
		return ConvertFloatToInt(fval, lowerBound, upperBound, tp)
	case KindMysqlJSON:
		return ConvertJSONToInt(sc, d.GetMysqlJSON(), false)
	case KindBinaryLiteral, KindMysqlBit:
		val, err := d.GetBinaryLiteral().ToInt(sc)
		return int64(val), errors.Trace(err)
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to int64", d.GetValue(), d.GetValue())
	}
}

// ToFloat64 converts to a float64
func (d *Datum) ToFloat64(sc *stmtctx.StatementContext) (float64, error) {
	switch d.Kind() {
	case KindInt64:
		return float64(d.GetInt64()), nil
	case KindUint64:
		return float64(d.GetUint64()), nil
	case KindFloat32:
		return float64(d.GetFloat32()), nil
	case KindFloat64:
		return d.GetFloat64(), nil
	case KindString:
		return StrToFloat(sc, d.GetString())
	case KindBytes:
		return StrToFloat(sc, string(d.GetBytes()))
	case KindMysqlTime:
		f, err := d.GetMysqlTime().ToNumber().ToFloat64()
		return f, errors.Trace(err)
	case KindMysqlDuration:
		f, err := d.GetMysqlDuration().ToNumber().ToFloat64()
		return f, errors.Trace(err)
	case KindMysqlDecimal:
		f, err := d.GetMysqlDecimal().ToFloat64()
		return f, errors.Trace(err)
	case KindMysqlEnum:
		return d.GetMysqlEnum().ToNumber(), nil
	case KindMysqlSet:
		return d.GetMysqlSet().ToNumber(), nil
	case KindBinaryLiteral, KindMysqlBit:
		val, err := d.GetBinaryLiteral().ToInt(sc)
		return float64(val), errors.Trace(err)
	case KindMysqlJSON:
		f, err := ConvertJSONToFloat(sc, d.GetMysqlJSON())
		return f, errors.Trace(err)
	default:
		return 0, errors.Errorf("cannot convert %v(type %T) to float64", d.GetValue(), d.GetValue())
	}
}

// ToString gets the string representation of the datum.
func (d *Datum) ToString() (string, error) {
	switch d.Kind() {
	case KindInt64:
		return strconv.FormatInt(d.GetInt64(), 10), nil
	case KindUint64:
		return strconv.FormatUint(d.GetUint64(), 10), nil
	case KindFloat32:
		return strconv.FormatFloat(float64(d.GetFloat32()), 'f', -1, 32), nil
	case KindFloat64:
		return strconv.FormatFloat(d.GetFloat64(), 'f', -1, 64), nil
	case KindString:
		return d.GetString(), nil
	case KindBytes:
		return d.GetString(), nil
	case KindMysqlTime:
		return d.GetMysqlTime().String(), nil
	case KindMysqlDuration:
		return d.GetMysqlDuration().String(), nil
	case KindMysqlDecimal:
		return d.GetMysqlDecimal().String(), nil
	case KindMysqlEnum:
		return d.GetMysqlEnum().String(), nil
	case KindMysqlSet:
		return d.GetMysqlSet().String(), nil
	case KindMysqlJSON:
		return d.GetMysqlJSON().String(), nil
	case KindBinaryLiteral, KindMysqlBit:
		return d.GetBinaryLiteral().ToString(), nil
	default:
		return "", errors.Errorf("cannot convert %v(type %T) to string", d.GetValue(), d.GetValue())
	}
}

// ToBytes gets the bytes representation of the datum.
func (d *Datum) ToBytes() ([]byte, error) {
	switch d.k {
	case KindString, KindBytes:
		return d.GetBytes(), nil
	default:
		str, err := d.ToString()
		if err != nil {
			return nil, errors.Trace(err)
		}
		return []byte(str), nil
	}
}

// ToMysqlJSON is similar to convertToMysqlJSON, except the
// latter parses from string, but the former uses it as primitive.
func (d *Datum) ToMysqlJSON() (j json.BinaryJSON, err error) {
	var in interface{}
	switch d.Kind() {
	case KindMysqlJSON:
		j = d.GetMysqlJSON()
		return
	case KindInt64:
		in = d.GetInt64()
	case KindUint64:
		in = d.GetUint64()
	case KindFloat32, KindFloat64:
		in = d.GetFloat64()
	case KindMysqlDecimal:
		in, err = d.GetMysqlDecimal().ToFloat64()
	case KindString, KindBytes:
		in = d.GetString()
	case KindBinaryLiteral, KindMysqlBit:
		in = d.GetBinaryLiteral().ToString()
	case KindNull:
		in = nil
	default:
		in, err = d.ToString()
	}
	if err != nil {
		err = errors.Trace(err)
		return
	}
	j = json.CreateBinary(in)
	return
}

func invalidConv(d *Datum, tp byte) (Datum, error) {
	return Datum{}, errors.Errorf("cannot convert datum from %s to type %s.", KindStr(d.Kind()), TypeStr(tp))
}

func (d *Datum) convergeType(hasUint, hasDecimal, hasFloat *bool) (x Datum) {
	x = *d
	switch d.Kind() {
	case KindUint64:
		*hasUint = true
	case KindFloat32:
		f := d.GetFloat32()
		x.SetFloat64(float64(f))
		*hasFloat = true
	case KindFloat64:
		*hasFloat = true
	case KindMysqlDecimal:
		*hasDecimal = true
	}
	return x
}

// CoerceDatum changes type.
// If a or b is Float, changes the both to Float.
// Else if a or b is Decimal, changes the both to Decimal.
// Else if a or b is Uint and op is not div, mod, or intDiv changes the both to Uint.
func CoerceDatum(sc *stmtctx.StatementContext, a, b Datum) (x, y Datum, err error) {
	if a.IsNull() || b.IsNull() {
		return x, y, nil
	}
	var hasUint, hasDecimal, hasFloat bool
	x = a.convergeType(&hasUint, &hasDecimal, &hasFloat)
	y = b.convergeType(&hasUint, &hasDecimal, &hasFloat)
	if hasFloat {
		switch x.Kind() {
		case KindInt64:
			x.SetFloat64(float64(x.GetInt64()))
		case KindUint64:
			x.SetFloat64(float64(x.GetUint64()))
		case KindMysqlEnum:
			x.SetFloat64(x.GetMysqlEnum().ToNumber())
		case KindMysqlSet:
			x.SetFloat64(x.GetMysqlSet().ToNumber())
		case KindMysqlDecimal:
			var fval float64
			fval, err = x.ToFloat64(sc)
			if err != nil {
				return x, y, errors.Trace(err)
			}
			x.SetFloat64(fval)
		}
		switch y.Kind() {
		case KindInt64:
			y.SetFloat64(float64(y.GetInt64()))
		case KindUint64:
			y.SetFloat64(float64(y.GetUint64()))
		case KindBinaryLiteral, KindMysqlBit:
			var fval uint64
			fval, err = y.GetBinaryLiteral().ToInt(sc)
			if err != nil {
				return x, y, errors.Trace(err)
			}
			y.SetFloat64(float64(fval))
		case KindMysqlEnum:
			y.SetFloat64(y.GetMysqlEnum().ToNumber())
		case KindMysqlSet:
			y.SetFloat64(y.GetMysqlSet().ToNumber())
		case KindMysqlDecimal:
			var fval float64
			fval, err = y.ToFloat64(sc)
			if err != nil {
				return x, y, errors.Trace(err)
			}
			y.SetFloat64(fval)
		}
	} else if hasDecimal {
		var dec *MyDecimal
		dec, err = ConvertDatumToDecimal(sc, x)
		if err != nil {
			return x, y, errors.Trace(err)
		}
		x.SetMysqlDecimal(dec)
		dec, err = ConvertDatumToDecimal(sc, y)
		if err != nil {
			return x, y, errors.Trace(err)
		}
		y.SetMysqlDecimal(dec)
	}
	return
}

// NewDatum creates a new Datum from an interface{}.
func NewDatum(in interface{}) (d Datum) {
	switch x := in.(type) {
	case []interface{}:
		d.SetValue(MakeDatums(x...))
	default:
		d.SetValue(in)
	}
	return d
}

// NewIntDatum creates a new Datum from an int64 value.
func NewIntDatum(i int64) (d Datum) {
	d.SetInt64(i)
	return d
}

// NewUintDatum creates a new Datum from an uint64 value.
func NewUintDatum(i uint64) (d Datum) {
	d.SetUint64(i)
	return d
}

// NewBytesDatum creates a new Datum from a byte slice.
func NewBytesDatum(b []byte) (d Datum) {
	d.SetBytes(b)
	return d
}

// NewStringDatum creates a new Datum from a string.
func NewStringDatum(s string) (d Datum) {
	d.SetString(s)
	return d
}

// NewFloat64Datum creates a new Datum from a float64 value.
func NewFloat64Datum(f float64) (d Datum) {
	d.SetFloat64(f)
	return d
}

// NewFloat32Datum creates a new Datum from a float32 value.
func NewFloat32Datum(f float32) (d Datum) {
	d.SetFloat32(f)
	return d
}

// NewDurationDatum creates a new Datum from a Duration value.
func NewDurationDatum(dur Duration) (d Datum) {
	d.SetMysqlDuration(dur)
	return d
}

// NewTimeDatum creates a new Time from a Time value.
func NewTimeDatum(t Time) (d Datum) {
	d.SetMysqlTime(t)
	return d
}

// NewDecimalDatum creates a new Datum form a MyDecimal value.
func NewDecimalDatum(dec *MyDecimal) (d Datum) {
	d.SetMysqlDecimal(dec)
	return d
}

// NewBinaryLiteralDatum creates a new BinaryLiteral Datum for a BinaryLiteral value.
func NewBinaryLiteralDatum(b BinaryLiteral) (d Datum) {
	d.SetBinaryLiteral(b)
	return d
}

// NewMysqlBitDatum creates a new MysqlBit Datum for a BinaryLiteral value.
func NewMysqlBitDatum(b BinaryLiteral) (d Datum) {
	d.SetMysqlBit(b)
	return d
}

// NewMysqlEnumDatum creates a new MysqlEnum Datum for a Enum value.
func NewMysqlEnumDatum(e Enum) (d Datum) {
	d.SetMysqlEnum(e)
	return d
}

// MakeDatums creates datum slice from interfaces.
func MakeDatums(args ...interface{}) []Datum {
	datums := make([]Datum, len(args))
	for i, v := range args {
		datums[i] = NewDatum(v)
	}
	return datums
}

// MinNotNullDatum returns a datum represents minimum not null value.
func MinNotNullDatum() Datum {
	return Datum{k: KindMinNotNull}
}

// MaxValueDatum returns a datum represents max value.
func MaxValueDatum() Datum {
	return Datum{k: KindMaxValue}
}

// EqualDatums compare if a and b contains the same datum values.
func EqualDatums(sc *stmtctx.StatementContext, a []Datum, b []Datum) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}
	if a == nil && b == nil {
		return true, nil
	}
	if a == nil || b == nil {
		return false, nil
	}
	for i, ai := range a {
		v, err := ai.CompareDatum(sc, &b[i])
		if err != nil {
			return false, errors.Trace(err)
		}
		if v != 0 {
			return false, nil
		}
	}
	return true, nil
}

// SortDatums sorts a slice of datum.
func SortDatums(sc *stmtctx.StatementContext, datums []Datum) error {
	sorter := datumsSorter{datums: datums, sc: sc}
	sort.Sort(&sorter)
	return sorter.err
}

type datumsSorter struct {
	datums []Datum
	sc     *stmtctx.StatementContext
	err    error
}

func (ds *datumsSorter) Len() int {
	return len(ds.datums)
}

func (ds *datumsSorter) Less(i, j int) bool {
	cmp, err := ds.datums[i].CompareDatum(ds.sc, &ds.datums[j])
	if err != nil {
		ds.err = errors.Trace(err)
		return true
	}
	return cmp < 0
}

func (ds *datumsSorter) Swap(i, j int) {
	ds.datums[i], ds.datums[j] = ds.datums[j], ds.datums[i]
}

func handleTruncateError(sc *stmtctx.StatementContext) error {
	if sc.IgnoreTruncate {
		return nil
	}
	if !sc.TruncateAsWarning {
		return ErrTruncated
	}
	sc.AppendWarning(ErrTruncated)
	return nil
}

// DatumsToString converts several datums to formatted string.
func DatumsToString(datums []Datum, handleSpecialValue bool) (string, error) {
	var strs []string
	for _, datum := range datums {
		if handleSpecialValue {
			switch datum.Kind() {
			case KindNull:
				strs = append(strs, "NULL")
				continue
			case KindMinNotNull:
				strs = append(strs, "-inf")
				continue
			case KindMaxValue:
				strs = append(strs, "+inf")
				continue
			}
		}
		str, err := datum.ToString()
		if err != nil {
			return "", errors.Trace(err)
		}
		strs = append(strs, str)
	}
	size := len(datums)
	if size > 1 {
		strs[0] = "(" + strs[0]
		strs[size-1] = strs[size-1] + ")"
	}
	return strings.Join(strs, ", "), nil
}

// DatumsToStrNoErr converts some datums to a formatted string.
// If an error occurs, it will print a log instead of returning an error.
func DatumsToStrNoErr(datums []Datum) string {
	str, err := DatumsToString(datums, true)
	terror.Log(errors.Trace(err))
	return str
}

// CopyDatum returns a new copy of the datum.
// TODO: Abandon this function.
func CopyDatum(datum Datum) Datum {
	return *datum.Copy()
}

// CopyRow deep copies a Datum slice.
func CopyRow(dr []Datum) []Datum {
	c := make([]Datum, len(dr))
	for i, d := range dr {
		c[i] = *d.Copy()
	}
	return c
}
