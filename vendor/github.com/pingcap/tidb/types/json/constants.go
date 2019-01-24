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
	"encoding/binary"
	"unicode/utf8"

	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
)

// TypeCode indicates JSON type.
type TypeCode = byte

const (
	// TypeCodeObject indicates the JSON is an object.
	TypeCodeObject TypeCode = 0x01
	// TypeCodeArray indicates the JSON is an array.
	TypeCodeArray TypeCode = 0x03
	// TypeCodeLiteral indicates the JSON is a literal.
	TypeCodeLiteral TypeCode = 0x04
	// TypeCodeInt64 indicates the JSON is a signed integer.
	TypeCodeInt64 TypeCode = 0x09
	// TypeCodeUint64 indicates the JSON is a unsigned integer.
	TypeCodeUint64 TypeCode = 0x0a
	// TypeCodeFloat64 indicates the JSON is a double float number.
	TypeCodeFloat64 TypeCode = 0x0b
	// TypeCodeString indicates the JSON is a string.
	TypeCodeString TypeCode = 0x0c
)

const (
	// LiteralNil represents JSON null.
	LiteralNil byte = 0x00
	// LiteralTrue represents JSON true.
	LiteralTrue byte = 0x01
	// LiteralFalse represents JSON false.
	LiteralFalse byte = 0x02
)

const unknownTypeCodeErrorMsg = "unknown type code: %d"
const unknownTypeErrorMsg = "unknown type: %s"

// htmlSafeSet holds the value true if the ASCII character with the given
// array position can be safely represented inside a JSON string, embedded
// inside of HTML <script> tags, without any additional escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), the backslash character ("\"), HTML opening and closing
// tags ("<" and ">"), and the ampersand ("&").
var htmlSafeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      false,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      false,
	'=':      true,
	'>':      false,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}

var (
	hexChars = "0123456789abcdef"
	endian   = binary.LittleEndian
)

const (
	headerSize   = 8 // element size + data size.
	dataSizeOff  = 4
	keyEntrySize = 6 // keyOff +  keyLen
	keyLenOff    = 4
	valTypeSize  = 1
	valEntrySize = 5
)

// jsonTypePrecedences is for comparing two json.
// See: https://dev.mysql.com/doc/refman/5.7/en/json.html#json-comparison
var jsonTypePrecedences = map[string]int{
	"BLOB":             -1,
	"BIT":              -2,
	"OPAQUE":           -3,
	"DATETIME":         -4,
	"TIME":             -5,
	"DATE":             -6,
	"BOOLEAN":          -7,
	"ARRAY":            -8,
	"OBJECT":           -9,
	"STRING":           -10,
	"INTEGER":          -11,
	"UNSIGNED INTEGER": -11,
	"DOUBLE":           -11,
	"NULL":             -12,
}

// ModifyType is for modify a JSON. There are three valid values:
// ModifyInsert, ModifyReplace and ModifySet.
type ModifyType byte

const (
	// ModifyInsert is for insert a new element into a JSON.
	ModifyInsert ModifyType = 0x01
	// ModifyReplace is for replace an old elemList from a JSON.
	ModifyReplace ModifyType = 0x02
	// ModifySet = ModifyInsert | ModifyReplace
	ModifySet ModifyType = 0x03
)

var (
	// ErrInvalidJSONText means invalid JSON text.
	ErrInvalidJSONText = terror.ClassJSON.New(mysql.ErrInvalidJSONText, mysql.MySQLErrName[mysql.ErrInvalidJSONText])
	// ErrInvalidJSONPath means invalid JSON path.
	ErrInvalidJSONPath = terror.ClassJSON.New(mysql.ErrInvalidJSONPath, mysql.MySQLErrName[mysql.ErrInvalidJSONPath])
	// ErrInvalidJSONData means invalid JSON data.
	ErrInvalidJSONData = terror.ClassJSON.New(mysql.ErrInvalidJSONData, mysql.MySQLErrName[mysql.ErrInvalidJSONData])
	// ErrInvalidJSONPathWildcard means invalid JSON path that contain wildcard characters.
	ErrInvalidJSONPathWildcard = terror.ClassJSON.New(mysql.ErrInvalidJSONPathWildcard, mysql.MySQLErrName[mysql.ErrInvalidJSONPathWildcard])
	// ErrInvalidJSONContainsPathType means invalid JSON contains path type.
	ErrInvalidJSONContainsPathType = terror.ClassJSON.New(mysql.ErrInvalidJSONContainsPathType, mysql.MySQLErrName[mysql.ErrInvalidJSONContainsPathType])
)

func init() {
	terror.ErrClassToMySQLCodes[terror.ClassJSON] = map[terror.ErrCode]uint16{
		mysql.ErrInvalidJSONText:             mysql.ErrInvalidJSONText,
		mysql.ErrInvalidJSONPath:             mysql.ErrInvalidJSONPath,
		mysql.ErrInvalidJSONData:             mysql.ErrInvalidJSONData,
		mysql.ErrInvalidJSONPathWildcard:     mysql.ErrInvalidJSONPathWildcard,
		mysql.ErrInvalidJSONContainsPathType: mysql.ErrInvalidJSONContainsPathType,
	}
}

// json_contains_path function type choices
// See: https://dev.mysql.com/doc/refman/5.7/en/json-search-functions.html#function_json-contains-path
const (
	// 'all': 1 if all paths exist within the document, 0 otherwise.
	ContainsPathAll = "all"
	// 'one': 1 if at least one path exists within the document, 0 otherwise.
	ContainsPathOne = "one"
)
