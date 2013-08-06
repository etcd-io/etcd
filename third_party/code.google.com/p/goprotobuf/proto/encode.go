// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2010 The Go Authors.  All rights reserved.
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

package proto

/*
 * Routines for encoding data into the wire format for protocol buffers.
 */

import (
	"errors"
	"reflect"
	"sort"
)

// ErrRequiredNotSet is the error returned if Marshal is called with
// a protocol buffer struct whose required fields have not
// all been initialized. It is also the error returned if Unmarshal is
// called with an encoded protocol buffer that does not include all the
// required fields.
type ErrRequiredNotSet struct {
	t reflect.Type
}

func (e *ErrRequiredNotSet) Error() string {
	return "proto: required fields not set in " + e.t.String()
}

var (
	// ErrRepeatedHasNil is the error returned if Marshal is called with
	// a struct with a repeated field containing a nil element.
	ErrRepeatedHasNil = errors.New("proto: repeated field has nil element")

	// ErrNil is the error returned if Marshal is called with nil.
	ErrNil = errors.New("proto: Marshal called with nil")
)

// The fundamental encoders that put bytes on the wire.
// Those that take integer types all accept uint64 and are
// therefore of type valueEncoder.

const maxVarintBytes = 10 // maximum length of a varint

// EncodeVarint returns the varint encoding of x.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
// Not used by the package itself, but helpful to clients
// wishing to use the same encoding.
func EncodeVarint(x uint64) []byte {
	var buf [maxVarintBytes]byte
	var n int
	for n = 0; x > 127; n++ {
		buf[n] = 0x80 | uint8(x&0x7F)
		x >>= 7
	}
	buf[n] = uint8(x)
	n++
	return buf[0:n]
}

// EncodeVarint writes a varint-encoded integer to the Buffer.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
func (p *Buffer) EncodeVarint(x uint64) error {
	for x >= 1<<7 {
		p.buf = append(p.buf, uint8(x&0x7f|0x80))
		x >>= 7
	}
	p.buf = append(p.buf, uint8(x))
	return nil
}

// EncodeFixed64 writes a 64-bit integer to the Buffer.
// This is the format for the
// fixed64, sfixed64, and double protocol buffer types.
func (p *Buffer) EncodeFixed64(x uint64) error {
	p.buf = append(p.buf,
		uint8(x),
		uint8(x>>8),
		uint8(x>>16),
		uint8(x>>24),
		uint8(x>>32),
		uint8(x>>40),
		uint8(x>>48),
		uint8(x>>56))
	return nil
}

// EncodeFixed32 writes a 32-bit integer to the Buffer.
// This is the format for the
// fixed32, sfixed32, and float protocol buffer types.
func (p *Buffer) EncodeFixed32(x uint64) error {
	p.buf = append(p.buf,
		uint8(x),
		uint8(x>>8),
		uint8(x>>16),
		uint8(x>>24))
	return nil
}

// EncodeZigzag64 writes a zigzag-encoded 64-bit integer
// to the Buffer.
// This is the format used for the sint64 protocol buffer type.
func (p *Buffer) EncodeZigzag64(x uint64) error {
	// use signed number to get arithmetic right shift.
	return p.EncodeVarint(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}

// EncodeZigzag32 writes a zigzag-encoded 32-bit integer
// to the Buffer.
// This is the format used for the sint32 protocol buffer type.
func (p *Buffer) EncodeZigzag32(x uint64) error {
	// use signed number to get arithmetic right shift.
	return p.EncodeVarint(uint64((uint32(x) << 1) ^ uint32((int32(x) >> 31))))
}

// EncodeRawBytes writes a count-delimited byte buffer to the Buffer.
// This is the format used for the bytes protocol buffer
// type and for embedded messages.
func (p *Buffer) EncodeRawBytes(b []byte) error {
	p.EncodeVarint(uint64(len(b)))
	p.buf = append(p.buf, b...)
	return nil
}

// EncodeStringBytes writes an encoded string to the Buffer.
// This is the format used for the proto2 string type.
func (p *Buffer) EncodeStringBytes(s string) error {
	p.EncodeVarint(uint64(len(s)))
	p.buf = append(p.buf, s...)
	return nil
}

// Marshaler is the interface representing objects that can marshal themselves.
type Marshaler interface {
	Marshal() ([]byte, error)
}

// Marshal takes the protocol buffer
// and encodes it into the wire format, returning the data.
func Marshal(pb Message) ([]byte, error) {
	// Can the object marshal itself?
	if m, ok := pb.(Marshaler); ok {
		return m.Marshal()
	}
	p := NewBuffer(nil)
	err := p.Marshal(pb)
	if err != nil {
		return nil, err
	}
	return p.buf, err
}

// Marshal takes the protocol buffer
// and encodes it into the wire format, writing the result to the
// Buffer.
func (p *Buffer) Marshal(pb Message) error {
	// Can the object marshal itself?
	if m, ok := pb.(Marshaler); ok {
		data, err := m.Marshal()
		if err != nil {
			return err
		}
		p.buf = append(p.buf, data...)
		return nil
	}

	t, base, err := getbase(pb)
	if structPointer_IsNil(base) {
		return ErrNil
	}
	if err == nil {
		err = p.enc_struct(t.Elem(), GetProperties(t.Elem()), base)
	}

	if collectStats {
		stats.Encode++
	}

	return err
}

// Individual type encoders.

// Encode a bool.
func (o *Buffer) enc_bool(p *Properties, base structPointer) error {
	v := *structPointer_Bool(base, p.field)
	if v == nil {
		return ErrNil
	}
	x := 0
	if *v {
		x = 1
	}
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, uint64(x))
	return nil
}

// Encode an int32.
func (o *Buffer) enc_int32(p *Properties, base structPointer) error {
	v := structPointer_Word32(base, p.field)
	if word32_IsNil(v) {
		return ErrNil
	}
	x := word32_Get(v)
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, uint64(x))
	return nil
}

// Encode an int64.
func (o *Buffer) enc_int64(p *Properties, base structPointer) error {
	v := structPointer_Word64(base, p.field)
	if word64_IsNil(v) {
		return ErrNil
	}
	x := word64_Get(v)
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, x)
	return nil
}

// Encode a string.
func (o *Buffer) enc_string(p *Properties, base structPointer) error {
	v := *structPointer_String(base, p.field)
	if v == nil {
		return ErrNil
	}
	x := *v
	o.buf = append(o.buf, p.tagcode...)
	o.EncodeStringBytes(x)
	return nil
}

// All protocol buffer fields are nillable, but be careful.
func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}
	return false
}

// Encode a message struct.
func (o *Buffer) enc_struct_message(p *Properties, base structPointer) error {
	structp := structPointer_GetStructPointer(base, p.field)
	if structPointer_IsNil(structp) {
		return ErrNil
	}

	// Can the object marshal itself?
	if p.isMarshaler {
		m := structPointer_Interface(structp, p.stype).(Marshaler)
		data, err := m.Marshal()
		if err != nil {
			return err
		}
		o.buf = append(o.buf, p.tagcode...)
		o.EncodeRawBytes(data)
		return nil
	}

	// need the length before we can write out the message itself,
	// so marshal into a separate byte buffer first.
	obuf := o.buf
	o.buf = o.bufalloc()

	err := o.enc_struct(p.stype, p.sprop, structp)

	nbuf := o.buf
	o.buf = obuf
	if err != nil {
		o.buffree(nbuf)
		return err
	}
	o.buf = append(o.buf, p.tagcode...)
	o.EncodeRawBytes(nbuf)
	o.buffree(nbuf)
	return nil
}

// Encode a group struct.
func (o *Buffer) enc_struct_group(p *Properties, base structPointer) error {
	b := structPointer_GetStructPointer(base, p.field)
	if structPointer_IsNil(b) {
		return ErrNil
	}

	o.EncodeVarint(uint64((p.Tag << 3) | WireStartGroup))
	err := o.enc_struct(p.stype, p.sprop, b)
	if err != nil {
		return err
	}
	o.EncodeVarint(uint64((p.Tag << 3) | WireEndGroup))
	return nil
}

// Encode a slice of bools ([]bool).
func (o *Buffer) enc_slice_bool(p *Properties, base structPointer) error {
	s := *structPointer_BoolSlice(base, p.field)
	l := len(s)
	if l == 0 {
		return ErrNil
	}
	for _, x := range s {
		o.buf = append(o.buf, p.tagcode...)
		v := uint64(0)
		if x {
			v = 1
		}
		p.valEnc(o, v)
	}
	return nil
}

// Encode a slice of bools ([]bool) in packed format.
func (o *Buffer) enc_slice_packed_bool(p *Properties, base structPointer) error {
	s := *structPointer_BoolSlice(base, p.field)
	l := len(s)
	if l == 0 {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	o.EncodeVarint(uint64(l)) // each bool takes exactly one byte
	for _, x := range s {
		v := uint64(0)
		if x {
			v = 1
		}
		p.valEnc(o, v)
	}
	return nil
}

// Encode a slice of bytes ([]byte).
func (o *Buffer) enc_slice_byte(p *Properties, base structPointer) error {
	s := *structPointer_Bytes(base, p.field)
	if s == nil {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	o.EncodeRawBytes(s)
	return nil
}

// Encode a slice of int32s ([]int32).
func (o *Buffer) enc_slice_int32(p *Properties, base structPointer) error {
	s := structPointer_Word32Slice(base, p.field)
	l := s.Len()
	if l == 0 {
		return ErrNil
	}
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		x := s.Index(i)
		p.valEnc(o, uint64(x))
	}
	return nil
}

// Encode a slice of int32s ([]int32) in packed format.
func (o *Buffer) enc_slice_packed_int32(p *Properties, base structPointer) error {
	s := structPointer_Word32Slice(base, p.field)
	l := s.Len()
	if l == 0 {
		return ErrNil
	}
	// TODO: Reuse a Buffer.
	buf := NewBuffer(nil)
	for i := 0; i < l; i++ {
		p.valEnc(buf, uint64(s.Index(i)))
	}

	o.buf = append(o.buf, p.tagcode...)
	o.EncodeVarint(uint64(len(buf.buf)))
	o.buf = append(o.buf, buf.buf...)
	return nil
}

// Encode a slice of int64s ([]int64).
func (o *Buffer) enc_slice_int64(p *Properties, base structPointer) error {
	s := structPointer_Word64Slice(base, p.field)
	l := s.Len()
	if l == 0 {
		return ErrNil
	}
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		p.valEnc(o, s.Index(i))
	}
	return nil
}

// Encode a slice of int64s ([]int64) in packed format.
func (o *Buffer) enc_slice_packed_int64(p *Properties, base structPointer) error {
	s := structPointer_Word64Slice(base, p.field)
	l := s.Len()
	if l == 0 {
		return ErrNil
	}
	// TODO: Reuse a Buffer.
	buf := NewBuffer(nil)
	for i := 0; i < l; i++ {
		p.valEnc(buf, s.Index(i))
	}

	o.buf = append(o.buf, p.tagcode...)
	o.EncodeVarint(uint64(len(buf.buf)))
	o.buf = append(o.buf, buf.buf...)
	return nil
}

// Encode a slice of slice of bytes ([][]byte).
func (o *Buffer) enc_slice_slice_byte(p *Properties, base structPointer) error {
	ss := *structPointer_BytesSlice(base, p.field)
	l := len(ss)
	if l == 0 {
		return ErrNil
	}
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		s := ss[i]
		o.EncodeRawBytes(s)
	}
	return nil
}

// Encode a slice of strings ([]string).
func (o *Buffer) enc_slice_string(p *Properties, base structPointer) error {
	ss := *structPointer_StringSlice(base, p.field)
	l := len(ss)
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		s := ss[i]
		o.EncodeStringBytes(s)
	}
	return nil
}

// Encode a slice of message structs ([]*struct).
func (o *Buffer) enc_slice_struct_message(p *Properties, base structPointer) error {
	s := structPointer_StructPointerSlice(base, p.field)
	l := s.Len()

	for i := 0; i < l; i++ {
		structp := s.Index(i)
		if structPointer_IsNil(structp) {
			return ErrRepeatedHasNil
		}

		// Can the object marshal itself?
		if p.isMarshaler {
			m := structPointer_Interface(structp, p.stype).(Marshaler)
			data, err := m.Marshal()
			if err != nil {
				return err
			}
			o.buf = append(o.buf, p.tagcode...)
			o.EncodeRawBytes(data)
			continue
		}

		obuf := o.buf
		o.buf = o.bufalloc()

		err := o.enc_struct(p.stype, p.sprop, structp)

		nbuf := o.buf
		o.buf = obuf
		if err != nil {
			o.buffree(nbuf)
			if err == ErrNil {
				return ErrRepeatedHasNil
			}
			return err
		}
		o.buf = append(o.buf, p.tagcode...)
		o.EncodeRawBytes(nbuf)

		o.buffree(nbuf)
	}
	return nil
}

// Encode a slice of group structs ([]*struct).
func (o *Buffer) enc_slice_struct_group(p *Properties, base structPointer) error {
	s := structPointer_StructPointerSlice(base, p.field)
	l := s.Len()

	for i := 0; i < l; i++ {
		b := s.Index(i)
		if structPointer_IsNil(b) {
			return ErrRepeatedHasNil
		}

		o.EncodeVarint(uint64((p.Tag << 3) | WireStartGroup))

		err := o.enc_struct(p.stype, p.sprop, b)

		if err != nil {
			if err == ErrNil {
				return ErrRepeatedHasNil
			}
			return err
		}

		o.EncodeVarint(uint64((p.Tag << 3) | WireEndGroup))
	}
	return nil
}

// Encode an extension map.
func (o *Buffer) enc_map(p *Properties, base structPointer) error {
	v := *structPointer_ExtMap(base, p.field)
	if err := encodeExtensionMap(v); err != nil {
		return err
	}
	// Fast-path for common cases: zero or one extensions.
	if len(v) <= 1 {
		for _, e := range v {
			o.buf = append(o.buf, e.enc...)
		}
		return nil
	}

	// Sort keys to provide a deterministic encoding.
	keys := make([]int, 0, len(v))
	for k := range v {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	for _, k := range keys {
		o.buf = append(o.buf, v[int32(k)].enc...)
	}
	return nil
}

// Encode a struct.
func (o *Buffer) enc_struct(t reflect.Type, prop *StructProperties, base structPointer) error {
	required := prop.reqCount
	// Encode fields in tag order so that decoders may use optimizations
	// that depend on the ordering.
	// http://code.google.com/apis/protocolbuffers/docs/encoding.html#order
	for _, i := range prop.order {
		p := prop.Prop[i]
		if p.enc != nil {
			err := p.enc(o, p, base)
			if err != nil {
				if err != ErrNil {
					return err
				}
			} else if p.Required {
				required--
			}
		}
	}
	// See if we encoded all required fields.
	if required > 0 {
		return &ErrRequiredNotSet{t}
	}

	// Add unrecognized fields at the end.
	if prop.unrecField.IsValid() {
		v := *structPointer_Bytes(base, prop.unrecField)
		if len(v) > 0 {
			o.buf = append(o.buf, v...)
		}
	}

	return nil
}
