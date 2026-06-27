// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains a modified copy of the encoding/xml encoder.
// All dynamic behavior has been removed, and reflecttion has been replaced with go/types.
// This allows us to statically find unmarshable types
// with the same rules for tags, shadowing and addressability as encoding/xml.
// This is used for SA1026 and SA5008.

// NOTE(dh): we do not check CanInterface in various places, which means we'll accept more marshaler implementations than encoding/xml does. This will lead to a small amount of false negatives.

package fakexml

import (
	"fmt"
	"go/types"
	"strings"

	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/knowledge"
	"honnef.co/go/tools/staticcheck/fakereflect"
)

func Marshal(v types.Type) error {
	return NewEncoder().Encode(v)
}

type Encoder struct {
	// TODO we track addressable and non-addressable instances separately out of an abundance of caution. We don't know
	// if this is actually required for correctness.
	seenCanAddr  typeutil.Map[struct{}]
	seenCantAddr typeutil.Map[struct{}]
}

func NewEncoder() *Encoder {
	e := &Encoder{}
	return e
}

func (enc *Encoder) Encode(v types.Type) error {
	rv := fakereflect.TypeAndCanAddr{Type: v}
	return enc.marshalValue(rv, nil, nil, "x")
}

func implementsMarshaler(v fakereflect.TypeAndCanAddr) bool {
	t := v.Type
	obj, _, _ := types.LookupFieldOrMethod(t, false, nil, "MarshalXML")
	if obj == nil {
		return false
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	params := fn.Type().(*types.Signature).Params()
	if params.Len() != 2 {
		return false
	}
	if !typeutil.IsPointerToTypeWithName(params.At(0).Type(), "encoding/xml.Encoder") {
		return false
	}
	if !typeutil.IsTypeWithName(params.At(1).Type(), "encoding/xml.StartElement") {
		return false
	}
	rets := fn.Type().(*types.Signature).Results()
	if rets.Len() != 1 {
		return false
	}
	if !typeutil.IsTypeWithName(rets.At(0).Type(), "error") {
		return false
	}
	return true
}

func implementsMarshalerAttr(v fakereflect.TypeAndCanAddr) bool {
	t := v.Type
	obj, _, _ := types.LookupFieldOrMethod(t, false, nil, "MarshalXMLAttr")
	if obj == nil {
		return false
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	params := fn.Type().(*types.Signature).Params()
	if params.Len() != 1 {
		return false
	}
	if !typeutil.IsTypeWithName(params.At(0).Type(), "encoding/xml.Name") {
		return false
	}
	rets := fn.Type().(*types.Signature).Results()
	if rets.Len() != 2 {
		return false
	}
	if !typeutil.IsTypeWithName(rets.At(0).Type(), "encoding/xml.Attr") {
		return false
	}
	if !typeutil.IsTypeWithName(rets.At(1).Type(), "error") {
		return false
	}
	return true
}

type CyclicTypeError struct {
	Type types.Type
	Path string
}

func (err *CyclicTypeError) Error() string {
	return "cyclic type"
}

// marshalValue writes one or more XML elements representing val.
// If val was obtained from a struct field, finfo must have its details.
func (e *Encoder) marshalValue(val fakereflect.TypeAndCanAddr, finfo *fieldInfo, startTemplate *StartElement, stack string) error {
	var m *typeutil.Map[struct{}]
	if val.CanAddr() {
		m = &e.seenCanAddr
	} else {
		m = &e.seenCantAddr
	}
	if _, ok := m.At(val.Type); ok {
		return nil
	}
	m.Set(val.Type, struct{}{})

	// Drill into interfaces and pointers.
	seen := map[fakereflect.TypeAndCanAddr]struct{}{}
	for val.IsInterface() || val.IsPtr() {
		if val.IsInterface() {
			return nil
		}
		val = val.Elem()
		if _, ok := seen[val]; ok {
			// Loop in type graph, e.g. 'type P *P'
			return &CyclicTypeError{val.Type, stack}
		}
		seen[val] = struct{}{}
	}

	// Check for marshaler.
	if implementsMarshaler(val) {
		return nil
	}
	if val.CanAddr() {
		pv := fakereflect.PtrTo(val)
		if implementsMarshaler(pv) {
			return nil
		}
	}

	// Check for text marshaler.
	if val.Implements(knowledge.Interfaces["encoding.TextMarshaler"]) {
		return nil
	}
	if val.CanAddr() {
		pv := fakereflect.PtrTo(val)
		if pv.Implements(knowledge.Interfaces["encoding.TextMarshaler"]) {
			return nil
		}
	}

	// Slices and arrays iterate over the elements. They do not have an enclosing tag.
	if (val.IsSlice() || val.IsArray()) && !isByteArray(val) && !isByteSlice(val) {
		if err := e.marshalValue(val.Elem(), finfo, startTemplate, stack+"[0]"); err != nil {
			return err
		}
		return nil
	}

	tinfo, err := getTypeInfo(val)
	if err != nil {
		return err
	}

	// Create start element.
	// Precedence for the XML element name is:
	// 0. startTemplate
	// 1. XMLName field in underlying struct;
	// 2. field name/tag in the struct field; and
	// 3. type name
	var start StartElement

	if startTemplate != nil {
		start.Name = startTemplate.Name
		start.Attr = append(start.Attr, startTemplate.Attr...)
	} else if tinfo.xmlname != nil {
		xmlname := tinfo.xmlname
		if xmlname.name != "" {
			start.Name.Space, start.Name.Local = xmlname.xmlns, xmlname.name
		}
	}

	// Attributes
	for i := range tinfo.fields {
		finfo := &tinfo.fields[i]
		if finfo.flags&fAttr == 0 {
			continue
		}
		fv := finfo.value(val)

		name := Name{Space: finfo.xmlns, Local: finfo.name}
		if err := e.marshalAttr(&start, name, fv, stack+pathByIndex(val, finfo.idx)); err != nil {
			return err
		}
	}

	if val.IsStruct() {
		return e.marshalStruct(tinfo, val, stack)
	} else {
		return e.marshalSimple(val, stack)
	}
}

func isSlice(v fakereflect.TypeAndCanAddr) bool {
	_, ok := v.Type.Underlying().(*types.Slice)
	return ok
}

func isByteSlice(v fakereflect.TypeAndCanAddr) bool {
	slice, ok := v.Type.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	basic, ok := slice.Elem().Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return basic.Kind() == types.Uint8
}

func isByteArray(v fakereflect.TypeAndCanAddr) bool {
	slice, ok := v.Type.Underlying().(*types.Array)
	if !ok {
		return false
	}
	basic, ok := slice.Elem().Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return basic.Kind() == types.Uint8
}

// marshalAttr marshals an attribute with the given name and value, adding to start.Attr.
func (e *Encoder) marshalAttr(start *StartElement, name Name, val fakereflect.TypeAndCanAddr, stack string) error {
	if implementsMarshalerAttr(val) {
		return nil
	}

	if val.CanAddr() {
		pv := fakereflect.PtrTo(val)
		if implementsMarshalerAttr(pv) {
			return nil
		}
	}

	if val.Implements(knowledge.Interfaces["encoding.TextMarshaler"]) {
		return nil
	}

	if val.CanAddr() {
		pv := fakereflect.PtrTo(val)
		if pv.Implements(knowledge.Interfaces["encoding.TextMarshaler"]) {
			return nil
		}
	}

	// Dereference or skip nil pointer
	if val.IsPtr() {
		val = val.Elem()
	}

	// Walk slices.
	if isSlice(val) && !isByteSlice(val) {
		if err := e.marshalAttr(start, name, val.Elem(), stack+"[0]"); err != nil {
			return err
		}
		return nil
	}

	if typeutil.IsTypeWithName(val.Type, "encoding/xml.Attr") {
		return nil
	}

	return e.marshalSimple(val, stack)
}

func (e *Encoder) marshalSimple(val fakereflect.TypeAndCanAddr, stack string) error {
	switch val.Type.Underlying().(type) {
	case *types.Basic, *types.Interface:
		return nil
	case *types.Slice, *types.Array:
		basic, ok := val.Elem().Type.Underlying().(*types.Basic)
		if !ok || basic.Kind() != types.Uint8 {
			return &UnsupportedTypeError{val.Type, stack}
		}
		return nil
	default:
		return &UnsupportedTypeError{val.Type, stack}
	}
}

func indirect(vf fakereflect.TypeAndCanAddr) fakereflect.TypeAndCanAddr {
	for vf.IsPtr() {
		vf = vf.Elem()
	}
	return vf
}

func pathByIndex(t fakereflect.TypeAndCanAddr, index []int) string {
	var path strings.Builder
	for _, i := range index {
		if t.IsPtr() {
			t = t.Elem()
		}
		path.WriteString("." + t.Field(i).Name)
		t = t.Field(i).Type
	}
	return path.String()
}

func (e *Encoder) marshalStruct(tinfo *typeInfo, val fakereflect.TypeAndCanAddr, stack string) error {
	for i := range tinfo.fields {
		finfo := &tinfo.fields[i]
		if finfo.flags&fAttr != 0 {
			continue
		}
		vf := finfo.value(val)

		switch finfo.flags & fMode {
		case fCDATA, fCharData:
			if vf.Implements(knowledge.Interfaces["encoding.TextMarshaler"]) {
				continue
			}
			if vf.CanAddr() {
				pv := fakereflect.PtrTo(vf)
				if pv.Implements(knowledge.Interfaces["encoding.TextMarshaler"]) {
					continue
				}
			}
			continue

		case fComment:
			vf = indirect(vf)
			if !(isByteSlice(vf) || isByteArray(vf)) {
				return fmt.Errorf("xml: bad type for comment field of %s", val)
			}
			continue

		case fInnerXML:
			vf = indirect(vf)
			if t, ok := vf.Type.(*types.Slice); (ok && types.Identical(t.Elem(), types.Typ[types.Byte])) || types.Identical(vf.Type, types.Typ[types.String]) {
				continue
			}

		case fElement, fElement | fAny:
		}
		if err := e.marshalValue(vf, finfo, nil, stack+pathByIndex(val, finfo.idx)); err != nil {
			return err
		}
	}
	return nil
}

// UnsupportedTypeError is returned when Marshal encounters a type
// that cannot be converted into XML.
type UnsupportedTypeError struct {
	Type types.Type
	Path string
}

func (e *UnsupportedTypeError) Error() string {
	return fmt.Sprintf("xml: unsupported type %s, via %s ", e.Type, e.Path)
}
