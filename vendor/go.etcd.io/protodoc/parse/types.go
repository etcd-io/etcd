// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parse

import "fmt"

// https://developers.google.com/protocol-buffers/docs/proto3#scalar
type (
	ProtoType       int
	ProtoTypeCpp    int
	ProtoTypeJava   int
	ProtoTypePython int
	ProtoTypeGo     int
	ProtoTypeRuby   int
	ProtoTypeCsharp int
)

const (
	Double ProtoType = iota
	Float
	Int32
	Int64
	Uint32
	Uint64
	Sint32
	Sint64
	Fixed32
	Fixed64
	Sfixed32
	Sfixed64
	Bool
	String
	Bytes
)

// https://developers.google.com/protocol-buffers/docs/proto3#scalar
var (
	ProtoTypes = [...]string{
		"double",
		"float",
		"int32",
		"int64",
		"uint32",
		"uint64",
		"sint32",
		"sint64",
		"fixed32",
		"fixed64",
		"sfixed32",
		"sfixed64",
		"bool",
		"string",
		"bytes",
	}

	protoTypesMap = map[string]ProtoType{
		"double":   Double,
		"float":    Float,
		"int32":    Int32,
		"int64":    Int64,
		"uint32":   Uint32,
		"uint64":   Uint64,
		"sint32":   Sint32,
		"sint64":   Sint64,
		"fixed32":  Fixed32,
		"fixed64":  Fixed64,
		"sfixed32": Sfixed32,
		"sfixed64": Sfixed64,
		"bool":     Bool,
		"string":   String,
		"bytes":    Bytes,
	}

	ToCpp = map[ProtoType]string{
		Double:   "double",
		Float:    "float",
		Int32:    "int32",
		Int64:    "int64",
		Uint32:   "uint32",
		Uint64:   "uint64",
		Sint32:   "int32",
		Sint64:   "int64",
		Fixed32:  "uint32",
		Fixed64:  "uint64",
		Sfixed32: "int32",
		Sfixed64: "int64",
		Bool:     "bool",
		String:   "string",
		Bytes:    "string",
	}

	ToJava = map[ProtoType]string{
		Double:   "double",
		Float:    "float",
		Int32:    "int",
		Int64:    "long",
		Uint32:   "int",
		Uint64:   "long",
		Sint32:   "int",
		Sint64:   "long",
		Fixed32:  "int",
		Fixed64:  "long",
		Sfixed32: "int32",
		Sfixed64: "int64",
		Bool:     "boolean",
		String:   "String",
		Bytes:    "ByteString",
	}

	ToPython = map[ProtoType]string{
		Double:   "float",
		Float:    "float",
		Int32:    "int",
		Int64:    "int/long",
		Uint32:   "int/long",
		Uint64:   "int/long",
		Sint32:   "int",
		Sint64:   "int/long",
		Fixed32:  "int",
		Fixed64:  "int/long",
		Sfixed32: "int",
		Sfixed64: "int/long",
		Bool:     "boolean",
		String:   "str/unicode",
		Bytes:    "str",
	}

	ToGo = map[ProtoType]string{
		Double:   "float64",
		Float:    "float32",
		Int32:    "int32",
		Int64:    "int64",
		Uint32:   "uint32",
		Uint64:   "uint64",
		Sint32:   "int32",
		Sint64:   "int64",
		Fixed32:  "uint32",
		Fixed64:  "uint64",
		Sfixed32: "int32",
		Sfixed64: "int64",
		Bool:     "bool",
		String:   "string",
		Bytes:    "[]byte",
	}

	ToRuby = map[ProtoType]string{
		Double:   "Float",
		Float:    "Float",
		Int32:    "Fixnum or Bignum (as required)",
		Int64:    "Bignum",
		Uint32:   "Fixnum or Bignum (as required)",
		Uint64:   "Bignum",
		Sint32:   "Fixnum or Bignum (as required)",
		Sint64:   "Bignum",
		Fixed32:  "Fixnum or Bignum (as required)",
		Fixed64:  "Bignum",
		Sfixed32: "Fixnum or Bignum (as required)",
		Sfixed64: "Bignum",
		Bool:     "TrueClass/FalseClass",
		String:   "String (UTF-8)",
		Bytes:    "String (ASCII-8BIT)",
	}

	ToCsharp = map[ProtoType]string{
		Double:   "double",
		Float:    "float",
		Int32:    "int",
		Int64:    "long",
		Uint32:   "uint",
		Uint64:   "ulong",
		Sint32:   "int",
		Sint64:   "long",
		Fixed32:  "uint",
		Fixed64:  "ulong",
		Sfixed32: "int",
		Sfixed64: "long",
		Bool:     "bool",
		String:   "string",
		Bytes:    "ByteString",
	}
)

// ToProtoType parses string to return 'ProtoType'.
func ToProtoType(s string) (ProtoType, error) {
	v, ok := protoTypesMap[s]
	if !ok {
		return 0, fmt.Errorf("%q is not defined", s)
	}
	return v, nil
}

func (t ProtoType) String() string {
	return ProtoTypes[t]
}

func (t ProtoType) Cpp() string {
	return ToCpp[t]
}

func (t ProtoType) Java() string {
	return ToJava[t]
}

func (t ProtoType) Python() string {
	return ToPython[t]
}

func (t ProtoType) Go() string {
	return ToGo[t]
}

func (t ProtoType) Ruby() string {
	return ToRuby[t]
}

func (t ProtoType) Csharp() string {
	return ToCsharp[t]
}
