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

package json

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/util/hack"
)

/*
   The binary JSON format from MySQL 5.7 is as follows:

   JSON doc ::= type value
   type ::=
       0x01 |       // large JSON object
       0x03 |       // large JSON array
       0x04 |       // literal (true/false/null)
       0x05 |       // int16
       0x06 |       // uint16
       0x07 |       // int32
       0x08 |       // uint32
       0x09 |       // int64
       0x0a |       // uint64
       0x0b |       // double
       0x0c |       // utf8mb4 string

   value ::=
       object  |
       array   |
       literal |
       number  |
       string  |

   object ::= element-count size key-entry* value-entry* key* value*

   array ::= element-count size value-entry* value*

   // number of members in object or number of elements in array
   element-count ::= uint32

   // number of bytes in the binary representation of the object or array
   size ::= uint32

   key-entry ::= key-offset key-length

   key-offset ::= uint32

   key-length ::= uint16    // key length must be less than 64KB

   value-entry ::= type offset-or-inlined-value

   // This field holds either the offset to where the value is stored,
   // or the value itself if it is small enough to be inlined (that is,
   // if it is a JSON literal or a small enough [u]int).
   offset-or-inlined-value ::= uint32

   key ::= utf8mb4-data

   literal ::=
       0x00 |   // JSON null literal
       0x01 |   // JSON true literal
       0x02 |   // JSON false literal

   number ::=  ....    // little-endian format for [u]int(16|32|64), whereas
                       // double is stored in a platform-independent, eight-byte
                       // format using float8store()

   string ::= data-length utf8mb4-data

   data-length ::= uint8*    // If the high bit of a byte is 1, the length
                             // field is continued in the next byte,
                             // otherwise it is the last byte of the length
                             // field. So we need 1 byte to represent
                             // lengths up to 127, 2 bytes to represent
                             // lengths up to 16383, and so on...
*/

// BinaryJSON represents a binary encoded JSON object.
// It can be randomly accessed without deserialization.
type BinaryJSON struct {
	TypeCode TypeCode
	Value    []byte
}

// String implements fmt.Stringer interface.
func (bj BinaryJSON) String() string {
	out, err := bj.MarshalJSON()
	terror.Log(err)
	return string(out)
}

// Copy makes a copy of the BinaryJSON
func (bj BinaryJSON) Copy() BinaryJSON {
	buf := make([]byte, len(bj.Value))
	copy(buf, bj.Value)
	return BinaryJSON{TypeCode: bj.TypeCode, Value: buf}
}

// MarshalJSON implements the json.Marshaler interface.
func (bj BinaryJSON) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, len(bj.Value)*3/2)
	return bj.marshalTo(buf)
}

func (bj BinaryJSON) marshalTo(buf []byte) ([]byte, error) {
	switch bj.TypeCode {
	case TypeCodeString:
		return marshalStringTo(buf, bj.GetString()), nil
	case TypeCodeLiteral:
		return marshalLiteralTo(buf, bj.Value[0]), nil
	case TypeCodeInt64:
		return strconv.AppendInt(buf, bj.GetInt64(), 10), nil
	case TypeCodeUint64:
		return strconv.AppendUint(buf, bj.GetUint64(), 10), nil
	case TypeCodeFloat64:
		return bj.marshalFloat64To(buf)
	case TypeCodeArray:
		return bj.marshalArrayTo(buf)
	case TypeCodeObject:
		return bj.marshalObjTo(buf)
	}
	return buf, nil
}

// GetInt64 gets the int64 value.
func (bj BinaryJSON) GetInt64() int64 {
	return int64(endian.Uint64(bj.Value))
}

// GetUint64 gets the uint64 value.
func (bj BinaryJSON) GetUint64() uint64 {
	return endian.Uint64(bj.Value)
}

// GetFloat64 gets the float64 value.
func (bj BinaryJSON) GetFloat64() float64 {
	return math.Float64frombits(bj.GetUint64())
}

// GetString gets the string value.
func (bj BinaryJSON) GetString() []byte {
	strLen, lenLen := uint64(bj.Value[0]), 1
	if strLen >= utf8.RuneSelf {
		strLen, lenLen = binary.Uvarint(bj.Value)
	}
	return bj.Value[lenLen : lenLen+int(strLen)]
}

// GetKeys gets the keys of the object
func (bj BinaryJSON) GetKeys() BinaryJSON {
	count := bj.GetElemCount()
	ret := make([]BinaryJSON, 0, count)
	for i := 0; i < count; i++ {
		ret = append(ret, CreateBinary(string(bj.objectGetKey(i))))
	}
	return buildBinaryArray(ret)
}

// GetElemCount gets the count of Object or Array.
func (bj BinaryJSON) GetElemCount() int {
	return int(endian.Uint32(bj.Value))
}

func (bj BinaryJSON) arrayGetElem(idx int) BinaryJSON {
	return bj.valEntryGet(headerSize + idx*valEntrySize)
}

func (bj BinaryJSON) objectGetKey(i int) []byte {
	keyOff := int(endian.Uint32(bj.Value[headerSize+i*keyEntrySize:]))
	keyLen := int(endian.Uint16(bj.Value[headerSize+i*keyEntrySize+keyLenOff:]))
	return bj.Value[keyOff : keyOff+keyLen]
}

func (bj BinaryJSON) objectGetVal(i int) BinaryJSON {
	elemCount := bj.GetElemCount()
	return bj.valEntryGet(headerSize + elemCount*keyEntrySize + i*valEntrySize)
}

func (bj BinaryJSON) valEntryGet(valEntryOff int) BinaryJSON {
	tpCode := bj.Value[valEntryOff]
	valOff := endian.Uint32(bj.Value[valEntryOff+valTypeSize:])
	switch tpCode {
	case TypeCodeLiteral:
		return BinaryJSON{TypeCode: TypeCodeLiteral, Value: bj.Value[valEntryOff+valTypeSize : valEntryOff+valTypeSize+1]}
	case TypeCodeUint64, TypeCodeInt64, TypeCodeFloat64:
		return BinaryJSON{TypeCode: tpCode, Value: bj.Value[valOff : valOff+8]}
	case TypeCodeString:
		strLen, lenLen := uint64(bj.Value[valOff]), 1
		if strLen >= utf8.RuneSelf {
			strLen, lenLen = binary.Uvarint(bj.Value[valOff:])
		}
		totalLen := uint32(lenLen) + uint32(strLen)
		return BinaryJSON{TypeCode: tpCode, Value: bj.Value[valOff : valOff+totalLen]}
	}
	dataSize := endian.Uint32(bj.Value[valOff+dataSizeOff:])
	return BinaryJSON{TypeCode: tpCode, Value: bj.Value[valOff : valOff+dataSize]}
}

func (bj BinaryJSON) marshalFloat64To(buf []byte) ([]byte, error) {
	// NOTE: copied from Go standard library.
	f := bj.GetFloat64()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return buf, &json.UnsupportedValueError{Str: strconv.FormatFloat(f, 'g', -1, 64)}
	}

	// Convert as if by ES6 number to string conversion.
	// This matches most other JSON generators.
	// See golang.org/issue/6384 and golang.org/issue/14135.
	// Like fmt %g, but the exponent cutoffs are different
	// and exponents themselves are not padded to two digits.
	abs := math.Abs(f)
	ffmt := byte('f')
	// Note: Must use float32 comparisons for underlying float32 value to get precise cutoffs right.
	if abs != 0 {
		if abs < 1e-6 || abs >= 1e21 {
			ffmt = 'e'
		}
	}
	buf = strconv.AppendFloat(buf, f, ffmt, -1, 64)
	if ffmt == 'e' {
		// clean up e-09 to e-9
		n := len(buf)
		if n >= 4 && buf[n-4] == 'e' && buf[n-3] == '-' && buf[n-2] == '0' {
			buf[n-2] = buf[n-1]
			buf = buf[:n-1]
		}
	}
	return buf, nil
}

func (bj BinaryJSON) marshalArrayTo(buf []byte) ([]byte, error) {
	elemCount := int(endian.Uint32(bj.Value))
	buf = append(buf, '[')
	for i := 0; i < elemCount; i++ {
		if i != 0 {
			buf = append(buf, ", "...)
		}
		var err error
		buf, err = bj.arrayGetElem(i).marshalTo(buf)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return append(buf, ']'), nil
}

func (bj BinaryJSON) marshalObjTo(buf []byte) ([]byte, error) {
	elemCount := int(endian.Uint32(bj.Value))
	buf = append(buf, '{')
	for i := 0; i < elemCount; i++ {
		if i != 0 {
			buf = append(buf, ", "...)
		}
		buf = marshalStringTo(buf, bj.objectGetKey(i))
		buf = append(buf, ": "...)
		var err error
		buf, err = bj.objectGetVal(i).marshalTo(buf)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return append(buf, '}'), nil
}

func marshalStringTo(buf, s []byte) []byte {
	// NOTE: copied from Go standard library.
	// NOTE: keep in sync with string above.
	buf = append(buf, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if htmlSafeSet[b] {
				i++
				continue
			}
			if start < i {
				buf = append(buf, s[start:i]...)
			}
			switch b {
			case '\\', '"':
				buf = append(buf, '\\', b)
			case '\n':
				buf = append(buf, '\\', 'n')
			case '\r':
				buf = append(buf, '\\', 'r')
			case '\t':
				buf = append(buf, '\\', 't')
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				buf = append(buf, `\u00`...)
				buf = append(buf, hexChars[b>>4], hexChars[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRune(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				buf = append(buf, s[start:i]...)
			}
			buf = append(buf, `\ufffd`...)
			i += size
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				buf = append(buf, s[start:i]...)
			}
			buf = append(buf, `\u202`...)
			buf = append(buf, hexChars[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		buf = append(buf, s[start:]...)
	}
	buf = append(buf, '"')
	return buf
}

func (bj BinaryJSON) marshalValueEntryTo(buf []byte, entryOff int) ([]byte, error) {
	tpCode := bj.Value[entryOff]
	switch tpCode {
	case TypeCodeLiteral:
		buf = marshalLiteralTo(buf, bj.Value[entryOff+1])
	default:
		offset := endian.Uint32(bj.Value[entryOff+1:])
		tmp := BinaryJSON{TypeCode: tpCode, Value: bj.Value[offset:]}
		var err error
		buf, err = tmp.marshalTo(buf)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return buf, nil
}

func marshalLiteralTo(b []byte, litType byte) []byte {
	switch litType {
	case LiteralFalse:
		return append(b, "false"...)
	case LiteralTrue:
		return append(b, "true"...)
	case LiteralNil:
		return append(b, "null"...)
	}
	return b
}

// ParseBinaryFromString parses a json from string.
func ParseBinaryFromString(s string) (bj BinaryJSON, err error) {
	if len(s) == 0 {
		err = ErrInvalidJSONText.GenWithStackByArgs("The document is empty")
		return
	}
	if err = bj.UnmarshalJSON(hack.Slice(s)); err != nil {
		err = ErrInvalidJSONText.GenWithStackByArgs(err)
	}
	return
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (bj *BinaryJSON) UnmarshalJSON(data []byte) error {
	var decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var in interface{}
	err := decoder.Decode(&in)
	if err != nil {
		return errors.Trace(err)
	}
	buf := make([]byte, 0, len(data))
	var typeCode TypeCode
	typeCode, buf, err = appendBinary(buf, in)
	if err != nil {
		return errors.Trace(err)
	}
	bj.TypeCode = typeCode
	bj.Value = buf
	return nil
}

// CreateBinary creates a BinaryJSON from interface.
func CreateBinary(in interface{}) BinaryJSON {
	typeCode, buf, err := appendBinary(nil, in)
	if err != nil {
		panic(err)
	}
	return BinaryJSON{TypeCode: typeCode, Value: buf}
}

func appendBinary(buf []byte, in interface{}) (TypeCode, []byte, error) {
	var typeCode byte
	var err error
	switch x := in.(type) {
	case nil:
		typeCode = TypeCodeLiteral
		buf = append(buf, LiteralNil)
	case bool:
		typeCode = TypeCodeLiteral
		if x {
			buf = append(buf, LiteralTrue)
		} else {
			buf = append(buf, LiteralFalse)
		}
	case int64:
		typeCode = TypeCodeInt64
		buf = appendBinaryUint64(buf, uint64(x))
	case uint64:
		typeCode = TypeCodeUint64
		buf = appendBinaryUint64(buf, x)
	case float64:
		typeCode = TypeCodeFloat64
		buf = appendBinaryFloat64(buf, x)
	case json.Number:
		typeCode, buf, err = appendBinaryNumber(buf, x)
		if err != nil {
			return typeCode, nil, errors.Trace(err)
		}
	case string:
		typeCode = TypeCodeString
		buf = appendBinaryString(buf, x)
	case BinaryJSON:
		typeCode = x.TypeCode
		buf = append(buf, x.Value...)
	case []interface{}:
		typeCode = TypeCodeArray
		buf, err = appendBinaryArray(buf, x)
		if err != nil {
			return typeCode, nil, errors.Trace(err)
		}
	case map[string]interface{}:
		typeCode = TypeCodeObject
		buf, err = appendBinaryObject(buf, x)
		if err != nil {
			return typeCode, nil, errors.Trace(err)
		}
	default:
		msg := fmt.Sprintf(unknownTypeErrorMsg, reflect.TypeOf(in))
		err = errors.New(msg)
	}
	return typeCode, buf, err
}

func appendZero(buf []byte, length int) []byte {
	var tmp [8]byte
	rem := length % 8
	loop := length / 8
	for i := 0; i < loop; i++ {
		buf = append(buf, tmp[:]...)
	}
	for i := 0; i < rem; i++ {
		buf = append(buf, 0)
	}
	return buf
}

func appendUint32(buf []byte, v uint32) []byte {
	var tmp [4]byte
	endian.PutUint32(tmp[:], v)
	return append(buf, tmp[:]...)
}

func appendBinaryNumber(buf []byte, x json.Number) (TypeCode, []byte, error) {
	var typeCode TypeCode
	if strings.ContainsAny(string(x), "Ee.") {
		typeCode = TypeCodeFloat64
		f64, err := x.Float64()
		if err != nil {
			return typeCode, nil, errors.Trace(err)
		}
		buf = appendBinaryFloat64(buf, f64)
	} else {
		typeCode = TypeCodeInt64
		i64, err := x.Int64()
		if err != nil {
			typeCode = TypeCodeFloat64
			f64, err := x.Float64()
			if err != nil {
				return typeCode, nil, errors.Trace(err)
			}
			buf = appendBinaryFloat64(buf, f64)
		} else {
			buf = appendBinaryUint64(buf, uint64(i64))
		}
	}
	return typeCode, buf, nil
}

func appendBinaryString(buf []byte, v string) []byte {
	begin := len(buf)
	buf = appendZero(buf, binary.MaxVarintLen64)
	lenLen := binary.PutUvarint(buf[begin:], uint64(len(v)))
	buf = buf[:len(buf)-binary.MaxVarintLen64+lenLen]
	buf = append(buf, v...)
	return buf
}

func appendBinaryFloat64(buf []byte, v float64) []byte {
	off := len(buf)
	buf = appendZero(buf, 8)
	endian.PutUint64(buf[off:], math.Float64bits(v))
	return buf
}

func appendBinaryUint64(buf []byte, v uint64) []byte {
	off := len(buf)
	buf = appendZero(buf, 8)
	endian.PutUint64(buf[off:], v)
	return buf
}

func appendBinaryArray(buf []byte, array []interface{}) ([]byte, error) {
	docOff := len(buf)
	buf = appendUint32(buf, uint32(len(array)))
	buf = appendZero(buf, dataSizeOff)
	valEntryBegin := len(buf)
	buf = appendZero(buf, len(array)*valEntrySize)
	for i, val := range array {
		var err error
		buf, err = appendBinaryValElem(buf, docOff, valEntryBegin+i*valEntrySize, val)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	docSize := len(buf) - docOff
	endian.PutUint32(buf[docOff+dataSizeOff:], uint32(docSize))
	return buf, nil
}

func appendBinaryValElem(buf []byte, docOff, valEntryOff int, val interface{}) ([]byte, error) {
	var typeCode TypeCode
	var err error
	elemDocOff := len(buf)
	typeCode, buf, err = appendBinary(buf, val)
	if err != nil {
		return nil, errors.Trace(err)
	}
	switch typeCode {
	case TypeCodeLiteral:
		litCode := buf[elemDocOff]
		buf = buf[:elemDocOff]
		buf[valEntryOff] = TypeCodeLiteral
		buf[valEntryOff+1] = litCode
		return buf, nil
	}
	buf[valEntryOff] = typeCode
	valOff := elemDocOff - docOff
	endian.PutUint32(buf[valEntryOff+1:], uint32(valOff))
	return buf, nil
}

type field struct {
	key string
	val interface{}
}

func appendBinaryObject(buf []byte, x map[string]interface{}) ([]byte, error) {
	docOff := len(buf)
	buf = appendUint32(buf, uint32(len(x)))
	buf = appendZero(buf, dataSizeOff)
	keyEntryBegin := len(buf)
	buf = appendZero(buf, len(x)*keyEntrySize)
	valEntryBegin := len(buf)
	buf = appendZero(buf, len(x)*valEntrySize)

	fields := make([]field, 0, len(x))
	for key, val := range x {
		fields = append(fields, field{key: key, val: val})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].key < fields[j].key
	})
	for i, field := range fields {
		keyEntryOff := keyEntryBegin + i*keyEntrySize
		keyOff := len(buf) - docOff
		keyLen := uint32(len(field.key))
		endian.PutUint32(buf[keyEntryOff:], uint32(keyOff))
		endian.PutUint16(buf[keyEntryOff+keyLenOff:], uint16(keyLen))
		buf = append(buf, field.key...)
	}
	for i, field := range fields {
		var err error
		buf, err = appendBinaryValElem(buf, docOff, valEntryBegin+i*valEntrySize, field.val)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	docSize := len(buf) - docOff
	endian.PutUint32(buf[docOff+dataSizeOff:], uint32(docSize))
	return buf, nil
}
