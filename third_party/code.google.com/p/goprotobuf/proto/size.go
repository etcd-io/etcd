// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2012 The Go Authors.  All rights reserved.
// http://code.google.com/p/goprotobuf/
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Functions to determine the size of an encoded protocol buffer.

package proto

import (
	"log"
	"reflect"
	"strings"
)

// Size returns the encoded size of a protocol buffer.
// This function is expensive enough to be avoided unless proven worthwhile with instrumentation.
func Size(pb Message) int {
	in := reflect.ValueOf(pb)
	if in.IsNil() {
		return 0
	}
	return sizeStruct(in.Elem())
}

func sizeStruct(x reflect.Value) (n int) {
	sprop := GetProperties(x.Type())
	for _, prop := range sprop.Prop {
		if strings.HasPrefix(prop.Name, "XXX_") { // handled below
			continue
		}
		fi, _ := sprop.decoderTags.get(prop.Tag)
		f := x.Field(fi)
		switch f.Kind() {
		case reflect.Ptr:
			if f.IsNil() {
				continue
			}
			n += len(prop.tagcode)
			f = f.Elem() // avoid a recursion in sizeField
		case reflect.Slice:
			if f.IsNil() {
				continue
			}
			if f.Len() == 0 && f.Type().Elem().Kind() != reflect.Uint8 {
				// short circuit for empty repeated fields.
				// []byte isn't a repeated field.
				continue
			}
		default:
			log.Printf("proto: unknown struct field type %v", f.Type())
			continue
		}
		n += sizeField(f, prop)
	}

	if em, ok := x.Addr().Interface().(extendableProto); ok {
		for _, ext := range em.ExtensionMap() {
			ms := len(ext.enc)
			if ext.enc == nil {
				props := new(Properties)
				props.Init(reflect.TypeOf(ext.desc.ExtensionType), "x", ext.desc.Tag, nil)
				ms = len(props.tagcode) + sizeField(reflect.ValueOf(ext.value), props)
			}
			n += ms
		}
	}

	if uf := x.FieldByName("XXX_unrecognized"); uf.IsValid() {
		n += uf.Len()
	}

	return n
}

func sizeField(x reflect.Value, prop *Properties) (n int) {
	if x.Type().Kind() == reflect.Slice {
		n := x.Len()
		et := x.Type().Elem()
		if et.Kind() == reflect.Uint8 {
			// []byte is easy.
			return len(prop.tagcode) + sizeVarint(uint64(n)) + n
		}

		var nb int

		// []bool and repeated fixed integer types are easy.
		switch {
		case et.Kind() == reflect.Bool:
			nb += n
		case prop.WireType == WireFixed64:
			nb += n * 8
		case prop.WireType == WireFixed32:
			nb += n * 4
		default:
			for i := 0; i < n; i++ {
				nb += sizeField(x.Index(i), prop)
			}
		}
		// Non-packed repeated fields have a per-element header of the tagcode.
		// Packed repeated fields only have a single header: the tag code plus a varint of the number of bytes.
		if !prop.Packed {
			nb += len(prop.tagcode) * n
		} else {
			nb += len(prop.tagcode) + sizeVarint(uint64(nb))
		}
		return nb
	}

	// easy scalars
	switch prop.WireType {
	case WireFixed64:
		return 8
	case WireFixed32:
		return 4
	}

	switch x.Kind() {
	case reflect.Bool:
		return 1
	case reflect.Int32, reflect.Int64:
		if prop.Wire == "varint" {
			return sizeVarint(uint64(x.Int()))
		} else if prop.Wire == "zigzag32" || prop.Wire == "zigzag64" {
			return sizeZigZag(uint64(x.Int()))
		}
	case reflect.Ptr:
		return sizeField(x.Elem(), prop)
	case reflect.String:
		n := x.Len()
		return sizeVarint(uint64(n)) + n
	case reflect.Struct:
		nb := sizeStruct(x)
		if prop.Wire == "group" {
			// Groups have start and end tags instead of a start tag and a length.
			return nb + len(prop.tagcode)
		}
		return sizeVarint(uint64(nb)) + nb
	case reflect.Uint32, reflect.Uint64:
		if prop.Wire == "varint" {
			return sizeVarint(uint64(x.Uint()))
		} else if prop.Wire == "zigzag32" || prop.Wire == "zigzag64" {
			return sizeZigZag(uint64(x.Int()))
		}
	default:
		log.Printf("proto.sizeField: unhandled kind %v", x.Kind())
	}

	// unknown type, so not a protocol buffer
	log.Printf("proto: don't know size of %v", x.Type())
	return 0
}

func sizeVarint(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}

func sizeZigZag(x uint64) (n int) {
	return sizeVarint(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
