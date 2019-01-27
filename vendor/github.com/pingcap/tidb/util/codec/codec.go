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

package codec

import (
	"encoding/binary"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
	"github.com/pingcap/tidb/util/chunk"
)

// First byte in the encoded value which specifies the encoding type.
const (
	NilFlag          byte = 0
	bytesFlag        byte = 1
	compactBytesFlag byte = 2
	intFlag          byte = 3
	uintFlag         byte = 4
	floatFlag        byte = 5
	decimalFlag      byte = 6
	durationFlag     byte = 7
	varintFlag       byte = 8
	uvarintFlag      byte = 9
	jsonFlag         byte = 10
	maxFlag          byte = 250
)

// encode will encode a datum and append it to a byte slice. If comparable is true, the encoded bytes can be sorted as it's original order.
// If hash is true, the encoded bytes can be checked equal as it's original value.
func encode(sc *stmtctx.StatementContext, b []byte, vals []types.Datum, comparable bool, hash bool) (_ []byte, err error) {
	for i, length := 0, len(vals); i < length; i++ {
		switch vals[i].Kind() {
		case types.KindInt64:
			b = encodeSignedInt(b, vals[i].GetInt64(), comparable)
		case types.KindUint64:
			if hash {
				integer := vals[i].GetInt64()
				if integer < 0 {
					b = encodeUnsignedInt(b, uint64(integer), comparable)
				} else {
					b = encodeSignedInt(b, integer, comparable)
				}
			} else {
				b = encodeUnsignedInt(b, vals[i].GetUint64(), comparable)
			}
		case types.KindFloat32, types.KindFloat64:
			b = append(b, floatFlag)
			b = EncodeFloat(b, vals[i].GetFloat64())
		case types.KindString, types.KindBytes:
			b = encodeBytes(b, vals[i].GetBytes(), comparable)
		case types.KindMysqlTime:
			b = append(b, uintFlag)
			t := vals[i].GetMysqlTime()
			// Encoding timestamp need to consider timezone.
			// If it's not in UTC, transform to UTC first.
			if t.Type == mysql.TypeTimestamp && sc.TimeZone != time.UTC {
				err = t.ConvertTimeZone(sc.TimeZone, time.UTC)
				if err != nil {
					return nil, errors.Trace(err)
				}
			}
			var v uint64
			v, err = t.ToPackedUint()
			if err != nil {
				return nil, errors.Trace(err)
			}
			b = EncodeUint(b, v)
		case types.KindMysqlDuration:
			// duration may have negative value, so we cannot use String to encode directly.
			b = append(b, durationFlag)
			b = EncodeInt(b, int64(vals[i].GetMysqlDuration().Duration))
		case types.KindMysqlDecimal:
			b = append(b, decimalFlag)
			if hash {
				// If hash is true, we only consider the original value of this decimal and ignore it's precision.
				dec := vals[i].GetMysqlDecimal()
				precision, frac := dec.PrecisionAndFrac()
				var bin []byte
				bin, err = dec.ToBin(precision, frac)
				if err != nil {
					return nil, errors.Trace(err)
				}
				b = append(b, bin...)
			} else {
				b, err = EncodeDecimal(b, vals[i].GetMysqlDecimal(), vals[i].Length(), vals[i].Frac())
				if terror.ErrorEqual(err, types.ErrTruncated) {
					err = sc.HandleTruncate(err)
				} else if terror.ErrorEqual(err, types.ErrOverflow) {
					err = sc.HandleOverflow(err, err)
				}
			}
		case types.KindMysqlEnum:
			b = encodeUnsignedInt(b, uint64(vals[i].GetMysqlEnum().ToNumber()), comparable)
		case types.KindMysqlSet:
			b = encodeUnsignedInt(b, uint64(vals[i].GetMysqlSet().ToNumber()), comparable)
		case types.KindMysqlBit, types.KindBinaryLiteral:
			// We don't need to handle errors here since the literal is ensured to be able to store in uint64 in convertToMysqlBit.
			var val uint64
			val, err = vals[i].GetBinaryLiteral().ToInt(sc)
			terror.Log(errors.Trace(err))
			b = encodeUnsignedInt(b, val, comparable)
		case types.KindMysqlJSON:
			b = append(b, jsonFlag)
			j := vals[i].GetMysqlJSON()
			b = append(b, j.TypeCode)
			b = append(b, j.Value...)
		case types.KindNull:
			b = append(b, NilFlag)
		case types.KindMinNotNull:
			b = append(b, bytesFlag)
		case types.KindMaxValue:
			b = append(b, maxFlag)
		default:
			return nil, errors.Errorf("unsupport encode type %d", vals[i].Kind())
		}
	}

	return b, errors.Trace(err)
}

func encodeBytes(b []byte, v []byte, comparable bool) []byte {
	if comparable {
		b = append(b, bytesFlag)
		b = EncodeBytes(b, v)
	} else {
		b = append(b, compactBytesFlag)
		b = EncodeCompactBytes(b, v)
	}
	return b
}

func encodeSignedInt(b []byte, v int64, comparable bool) []byte {
	if comparable {
		b = append(b, intFlag)
		b = EncodeInt(b, v)
	} else {
		b = append(b, varintFlag)
		b = EncodeVarint(b, v)
	}
	return b
}

func encodeUnsignedInt(b []byte, v uint64, comparable bool) []byte {
	if comparable {
		b = append(b, uintFlag)
		b = EncodeUint(b, v)
	} else {
		b = append(b, uvarintFlag)
		b = EncodeUvarint(b, v)
	}
	return b
}

// EncodeKey appends the encoded values to byte slice b, returns the appended
// slice. It guarantees the encoded value is in ascending order for comparison.
// For Decimal type, datum must set datum's length and frac.
func EncodeKey(sc *stmtctx.StatementContext, b []byte, v ...types.Datum) ([]byte, error) {
	return encode(sc, b, v, true, false)
}

// EncodeValue appends the encoded values to byte slice b, returning the appended
// slice. It does not guarantee the order for comparison.
func EncodeValue(sc *stmtctx.StatementContext, b []byte, v ...types.Datum) ([]byte, error) {
	return encode(sc, b, v, false, false)
}

func encodeChunkRow(sc *stmtctx.StatementContext, b []byte, row chunk.Row, allTypes []*types.FieldType, colIdx []int, comparable, hash bool) (_ []byte, err error) {
	for _, i := range colIdx {
		if row.IsNull(i) {
			b = append(b, NilFlag)
			continue
		}
		switch allTypes[i].Tp {
		case mysql.TypeTiny, mysql.TypeShort, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong, mysql.TypeYear:
			if !mysql.HasUnsignedFlag(allTypes[i].Flag) {
				b = encodeSignedInt(b, row.GetInt64(i), comparable)
				break
			}
			// encode unsigned integers.
			if hash {
				integer := row.GetInt64(i)
				if integer < 0 {
					b = encodeUnsignedInt(b, uint64(integer), comparable)
				} else {
					b = encodeSignedInt(b, integer, comparable)
				}
			} else {
				b = encodeUnsignedInt(b, row.GetUint64(i), comparable)
			}
		case mysql.TypeFloat:
			b = append(b, floatFlag)
			b = EncodeFloat(b, float64(row.GetFloat32(i)))
		case mysql.TypeDouble:
			b = append(b, floatFlag)
			b = EncodeFloat(b, row.GetFloat64(i))
		case mysql.TypeVarchar, mysql.TypeVarString, mysql.TypeString, mysql.TypeBlob, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeLongBlob:
			b = encodeBytes(b, row.GetBytes(i), comparable)
		case mysql.TypeDate, mysql.TypeDatetime, mysql.TypeTimestamp:
			b = append(b, uintFlag)
			t := row.GetTime(i)
			// Encoding timestamp need to consider timezone.
			// If it's not in UTC, transform to UTC first.
			if t.Type == mysql.TypeTimestamp && sc.TimeZone != time.UTC {
				err = t.ConvertTimeZone(sc.TimeZone, time.UTC)
				if err != nil {
					return nil, errors.Trace(err)
				}
			}
			var v uint64
			v, err = t.ToPackedUint()
			if err != nil {
				return nil, errors.Trace(err)
			}
			b = EncodeUint(b, v)
		case mysql.TypeDuration:
			// duration may have negative value, so we cannot use String to encode directly.
			b = append(b, durationFlag)
			b = EncodeInt(b, int64(row.GetDuration(i, 0).Duration))
		case mysql.TypeNewDecimal:
			b = append(b, decimalFlag)
			if hash {
				// If hash is true, we only consider the original value of this decimal and ignore it's precision.
				dec := row.GetMyDecimal(i)
				precision, frac := dec.PrecisionAndFrac()
				var bin []byte
				bin, err = dec.ToBin(precision, frac)
				if err != nil {
					return nil, errors.Trace(err)
				}
				b = append(b, bin...)
			} else {
				b, err = EncodeDecimal(b, row.GetMyDecimal(i), allTypes[i].Flen, allTypes[i].Decimal)
				if terror.ErrorEqual(err, types.ErrTruncated) {
					err = sc.HandleTruncate(err)
				}
			}
		case mysql.TypeEnum:
			b = encodeUnsignedInt(b, uint64(row.GetEnum(i).ToNumber()), comparable)
		case mysql.TypeSet:
			b = encodeUnsignedInt(b, uint64(row.GetSet(i).ToNumber()), comparable)
		case mysql.TypeBit:
			// We don't need to handle errors here since the literal is ensured to be able to store in uint64 in convertToMysqlBit.
			var val uint64
			val, err = types.BinaryLiteral(row.GetBytes(i)).ToInt(sc)
			terror.Log(errors.Trace(err))
			b = encodeUnsignedInt(b, val, comparable)
		case mysql.TypeJSON:
			b = append(b, jsonFlag)
			j := row.GetJSON(i)
			b = append(b, j.TypeCode)
			b = append(b, j.Value...)
		default:
			return nil, errors.Errorf("unsupport column type for encode %d", allTypes[i].Tp)
		}
	}
	return b, errors.Trace(err)
}

// HashValues appends the encoded values to byte slice b, returning the appended
// slice. If two datums are equal, they will generate the same bytes.
func HashValues(sc *stmtctx.StatementContext, b []byte, v ...types.Datum) ([]byte, error) {
	return encode(sc, b, v, false, true)
}

// HashChunkRow appends the encoded values to byte slice "b", returning the appended slice.
// If two rows are equal, it will generate the same bytes.
func HashChunkRow(sc *stmtctx.StatementContext, b []byte, row chunk.Row, allTypes []*types.FieldType, colIdx []int) ([]byte, error) {
	return encodeChunkRow(sc, b, row, allTypes, colIdx, false, true)
}

// Decode decodes values from a byte slice generated with EncodeKey or EncodeValue
// before.
// size is the size of decoded datum slice.
func Decode(b []byte, size int) ([]types.Datum, error) {
	if len(b) < 1 {
		return nil, errors.New("invalid encoded key")
	}

	var (
		err    error
		values = make([]types.Datum, 0, size)
	)

	for len(b) > 0 {
		var d types.Datum
		b, d, err = DecodeOne(b)
		if err != nil {
			return nil, errors.Trace(err)
		}

		values = append(values, d)
	}

	return values, nil
}

// DecodeRange decodes the range values from a byte slice that generated by EncodeKey.
// It handles some special values like `MinNotNull` and `MaxValueDatum`.
func DecodeRange(b []byte, size int) ([]types.Datum, error) {
	if len(b) < 1 {
		return nil, errors.New("invalid encoded key: length of key is zero")
	}

	var (
		err    error
		values = make([]types.Datum, 0, size)
	)

	for len(b) > 1 {
		var d types.Datum
		b, d, err = DecodeOne(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		values = append(values, d)
	}

	if len(b) == 1 {
		switch b[0] {
		case NilFlag:
			values = append(values, types.Datum{})
		case bytesFlag:
			values = append(values, types.MinNotNullDatum())
		// `maxFlag + 1` for PrefixNext
		case maxFlag, maxFlag + 1:
			values = append(values, types.MaxValueDatum())
		default:
			return nil, errors.Errorf("invalid encoded key flag %v", b[0])
		}
	}
	return values, nil
}

// DecodeOne decodes on datum from a byte slice generated with EncodeKey or EncodeValue.
func DecodeOne(b []byte) (remain []byte, d types.Datum, err error) {
	if len(b) < 1 {
		return nil, d, errors.New("invalid encoded key")
	}
	flag := b[0]
	b = b[1:]
	switch flag {
	case intFlag:
		var v int64
		b, v, err = DecodeInt(b)
		d.SetInt64(v)
	case uintFlag:
		var v uint64
		b, v, err = DecodeUint(b)
		d.SetUint64(v)
	case varintFlag:
		var v int64
		b, v, err = DecodeVarint(b)
		d.SetInt64(v)
	case uvarintFlag:
		var v uint64
		b, v, err = DecodeUvarint(b)
		d.SetUint64(v)
	case floatFlag:
		var v float64
		b, v, err = DecodeFloat(b)
		d.SetFloat64(v)
	case bytesFlag:
		var v []byte
		b, v, err = DecodeBytes(b, nil)
		d.SetBytes(v)
	case compactBytesFlag:
		var v []byte
		b, v, err = DecodeCompactBytes(b)
		d.SetBytes(v)
	case decimalFlag:
		var (
			dec             *types.MyDecimal
			precision, frac int
		)
		b, dec, precision, frac, err = DecodeDecimal(b)
		if err == nil {
			d.SetMysqlDecimal(dec)
			d.SetLength(precision)
			d.SetFrac(frac)
		}
	case durationFlag:
		var r int64
		b, r, err = DecodeInt(b)
		if err == nil {
			// use max fsp, let outer to do round manually.
			v := types.Duration{Duration: time.Duration(r), Fsp: types.MaxFsp}
			d.SetValue(v)
		}
	case jsonFlag:
		var size int
		size, err = json.PeekBytesAsJSON(b)
		if err != nil {
			return b, d, err
		}
		j := json.BinaryJSON{TypeCode: b[0], Value: b[1:size]}
		d.SetMysqlJSON(j)
		b = b[size:]
	case NilFlag:
	default:
		return b, d, errors.Errorf("invalid encoded key flag %v", flag)
	}
	if err != nil {
		return b, d, errors.Trace(err)
	}
	return b, d, nil
}

// CutOne cuts the first encoded value from b.
// It will return the first encoded item and the remains as byte slice.
func CutOne(b []byte) (data []byte, remain []byte, err error) {
	l, err := peek(b)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	return b[:l], b[l:], nil
}

// SetRawValues set raw datum values from a row data.
func SetRawValues(data []byte, values []types.Datum) error {
	for i := 0; i < len(values); i++ {
		l, err := peek(data)
		if err != nil {
			return errors.Trace(err)
		}
		values[i].SetRaw(data[:l:l])
		data = data[l:]
	}
	return nil
}

// peek peeks the first encoded value from b and returns its length.
func peek(b []byte) (length int, err error) {
	if len(b) < 1 {
		return 0, errors.New("invalid encoded key")
	}
	flag := b[0]
	length++
	b = b[1:]
	var l int
	switch flag {
	case NilFlag:
	case intFlag, uintFlag, floatFlag, durationFlag:
		// Those types are stored in 8 bytes.
		l = 8
	case bytesFlag:
		l, err = peekBytes(b, false)
	case compactBytesFlag:
		l, err = peekCompactBytes(b)
	case decimalFlag:
		l, err = types.DecimalPeak(b)
	case varintFlag:
		l, err = peekVarint(b)
	case uvarintFlag:
		l, err = peekUvarint(b)
	case jsonFlag:
		l, err = json.PeekBytesAsJSON(b)
	default:
		return 0, errors.Errorf("invalid encoded key flag %v", flag)
	}
	if err != nil {
		return 0, errors.Trace(err)
	}
	length += l
	return
}

func peekBytes(b []byte, reverse bool) (int, error) {
	offset := 0
	for {
		if len(b) < offset+encGroupSize+1 {
			return 0, errors.New("insufficient bytes to decode value")
		}
		// The byte slice is encoded into many groups.
		// For each group, there are 8 bytes for data and 1 byte for marker.
		marker := b[offset+encGroupSize]
		var padCount byte
		if reverse {
			padCount = marker
		} else {
			padCount = encMarker - marker
		}
		offset += encGroupSize + 1
		// When padCount is not zero, it means we get the end of the byte slice.
		if padCount != 0 {
			break
		}
	}
	return offset, nil
}

func peekCompactBytes(b []byte) (int, error) {
	// Get length.
	v, n := binary.Varint(b)
	vi := int(v)
	if n < 0 {
		return 0, errors.New("value larger than 64 bits")
	} else if n == 0 {
		return 0, errors.New("insufficient bytes to decode value")
	}
	if len(b) < vi+n {
		return 0, errors.Errorf("insufficient bytes to decode value, expected length: %v", n)
	}
	return n + vi, nil
}

func peekVarint(b []byte) (int, error) {
	_, n := binary.Varint(b)
	if n < 0 {
		return 0, errors.New("value larger than 64 bits")
	}
	return n, nil
}

func peekUvarint(b []byte) (int, error) {
	_, n := binary.Uvarint(b)
	if n < 0 {
		return 0, errors.New("value larger than 64 bits")
	}
	return n, nil
}

// Decoder is used to decode value to chunk.
type Decoder struct {
	chk      *chunk.Chunk
	timezone *time.Location

	// buf is only used for DecodeBytes to avoid the cost of makeslice.
	buf []byte
}

// NewDecoder creates a Decoder.
func NewDecoder(chk *chunk.Chunk, timezone *time.Location) *Decoder {
	return &Decoder{
		chk:      chk,
		timezone: timezone,
	}
}

// DecodeOne decodes one value to chunk and returns the remained bytes.
func (decoder *Decoder) DecodeOne(b []byte, colIdx int, ft *types.FieldType) (remain []byte, err error) {
	if len(b) < 1 {
		return nil, errors.New("invalid encoded key")
	}
	chk := decoder.chk
	flag := b[0]
	b = b[1:]
	switch flag {
	case intFlag:
		var v int64
		b, v, err = DecodeInt(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		appendIntToChunk(v, chk, colIdx, ft)
	case uintFlag:
		var v uint64
		b, v, err = DecodeUint(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		err = appendUintToChunk(v, chk, colIdx, ft, decoder.timezone)
	case varintFlag:
		var v int64
		b, v, err = DecodeVarint(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		appendIntToChunk(v, chk, colIdx, ft)
	case uvarintFlag:
		var v uint64
		b, v, err = DecodeUvarint(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		err = appendUintToChunk(v, chk, colIdx, ft, decoder.timezone)
	case floatFlag:
		var v float64
		b, v, err = DecodeFloat(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		appendFloatToChunk(v, chk, colIdx, ft)
	case bytesFlag:
		b, decoder.buf, err = DecodeBytes(b, decoder.buf)
		if err != nil {
			return nil, errors.Trace(err)
		}
		chk.AppendBytes(colIdx, decoder.buf)
	case compactBytesFlag:
		var v []byte
		b, v, err = DecodeCompactBytes(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		chk.AppendBytes(colIdx, v)
	case decimalFlag:
		var dec *types.MyDecimal
		b, dec, _, _, err = DecodeDecimal(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		chk.AppendMyDecimal(colIdx, dec)
	case durationFlag:
		var r int64
		b, r, err = DecodeInt(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		v := types.Duration{Duration: time.Duration(r), Fsp: ft.Decimal}
		chk.AppendDuration(colIdx, v)
	case jsonFlag:
		var size int
		size, err = json.PeekBytesAsJSON(b)
		if err != nil {
			return nil, errors.Trace(err)
		}
		chk.AppendJSON(colIdx, json.BinaryJSON{TypeCode: b[0], Value: b[1:size]})
		b = b[size:]
	case NilFlag:
		chk.AppendNull(colIdx)
	default:
		return nil, errors.Errorf("invalid encoded key flag %v", flag)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	return b, nil
}

func appendIntToChunk(val int64, chk *chunk.Chunk, colIdx int, ft *types.FieldType) {
	switch ft.Tp {
	case mysql.TypeDuration:
		v := types.Duration{Duration: time.Duration(val), Fsp: ft.Decimal}
		chk.AppendDuration(colIdx, v)
	default:
		chk.AppendInt64(colIdx, val)
	}
}

func appendUintToChunk(val uint64, chk *chunk.Chunk, colIdx int, ft *types.FieldType, loc *time.Location) error {
	switch ft.Tp {
	case mysql.TypeDate, mysql.TypeDatetime, mysql.TypeTimestamp:
		var t types.Time
		t.Type = ft.Tp
		t.Fsp = ft.Decimal
		var err error
		err = t.FromPackedUint(val)
		if err != nil {
			return errors.Trace(err)
		}
		if ft.Tp == mysql.TypeTimestamp && !t.IsZero() {
			err = t.ConvertTimeZone(time.UTC, loc)
			if err != nil {
				return errors.Trace(err)
			}
		}
		chk.AppendTime(colIdx, t)
	case mysql.TypeEnum:
		// ignore error deliberately, to read empty enum value.
		enum, err := types.ParseEnumValue(ft.Elems, val)
		if err != nil {
			enum = types.Enum{}
		}
		chk.AppendEnum(colIdx, enum)
	case mysql.TypeSet:
		set, err := types.ParseSetValue(ft.Elems, val)
		if err != nil {
			return errors.Trace(err)
		}
		chk.AppendSet(colIdx, set)
	case mysql.TypeBit:
		byteSize := (ft.Flen + 7) >> 3
		chk.AppendBytes(colIdx, types.NewBinaryLiteralFromUint(val, byteSize))
	default:
		chk.AppendUint64(colIdx, val)
	}
	return nil
}

func appendFloatToChunk(val float64, chk *chunk.Chunk, colIdx int, ft *types.FieldType) {
	if ft.Tp == mysql.TypeFloat {
		chk.AppendFloat32(colIdx, float32(val))
	} else {
		chk.AppendFloat64(colIdx, val)
	}
}
