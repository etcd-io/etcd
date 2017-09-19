// +build !go1.7 safe appengine

// Copyright (c) 2012-2015 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"reflect"
	"sync/atomic"
)

// stringView returns a view of the []byte as a string.
// In unsafe mode, it doesn't incur allocation and copying caused by conversion.
// In regular safe mode, it is an allocation and copy.
//
// Usage: Always maintain a reference to v while result of this call is in use,
//        and call keepAlive4BytesView(v) at point where done with view.
func stringView(v []byte) string {
	return string(v)
}

// bytesView returns a view of the string as a []byte.
// In unsafe mode, it doesn't incur allocation and copying caused by conversion.
// In regular safe mode, it is an allocation and copy.
//
// Usage: Always maintain a reference to v while result of this call is in use,
//        and call keepAlive4BytesView(v) at point where done with view.
func bytesView(v string) []byte {
	return []byte(v)
}

// // keepAlive4BytesView maintains a reference to the input parameter for bytesView.
// //
// // Usage: call this at point where done with the bytes view.
// func keepAlive4BytesView(v string) {}

// // keepAlive4BytesView maintains a reference to the input parameter for stringView.
// //
// // Usage: call this at point where done with the string view.
// func keepAlive4StringView(v []byte) {}

func rv2i(rv reflect.Value) interface{} {
	return rv.Interface()
}

func rt2id(rt reflect.Type) uintptr {
	return reflect.ValueOf(rt).Pointer()
}

// --------------------------
type ptrToRvMap struct{}

func (_ *ptrToRvMap) init() {}
func (_ *ptrToRvMap) get(i interface{}) reflect.Value {
	return reflect.ValueOf(i).Elem()
}

// --------------------------
type atomicTypeInfoSlice struct {
	v atomic.Value
}

func (x *atomicTypeInfoSlice) load() *[]rtid2ti {
	i := x.v.Load()
	if i == nil {
		return nil
	}
	return i.(*[]rtid2ti)
}

func (x *atomicTypeInfoSlice) store(p *[]rtid2ti) {
	x.v.Store(p)
}

// --------------------------
func (f *decFnInfo) raw(rv reflect.Value) {
	rv.SetBytes(f.d.raw())
}

func (f *decFnInfo) kString(rv reflect.Value) {
	rv.SetString(f.d.d.DecodeString())
}

func (f *decFnInfo) kBool(rv reflect.Value) {
	rv.SetBool(f.d.d.DecodeBool())
}

func (f *decFnInfo) kFloat32(rv reflect.Value) {
	rv.SetFloat(f.d.d.DecodeFloat(true))
}

func (f *decFnInfo) kFloat64(rv reflect.Value) {
	rv.SetFloat(f.d.d.DecodeFloat(false))
}

func (f *decFnInfo) kInt(rv reflect.Value) {
	rv.SetInt(f.d.d.DecodeInt(intBitsize))
}

func (f *decFnInfo) kInt8(rv reflect.Value) {
	rv.SetInt(f.d.d.DecodeInt(8))
}

func (f *decFnInfo) kInt16(rv reflect.Value) {
	rv.SetInt(f.d.d.DecodeInt(16))
}

func (f *decFnInfo) kInt32(rv reflect.Value) {
	rv.SetInt(f.d.d.DecodeInt(32))
}

func (f *decFnInfo) kInt64(rv reflect.Value) {
	rv.SetInt(f.d.d.DecodeInt(64))
}

func (f *decFnInfo) kUint(rv reflect.Value) {
	rv.SetUint(f.d.d.DecodeUint(uintBitsize))
}

func (f *decFnInfo) kUintptr(rv reflect.Value) {
	rv.SetUint(f.d.d.DecodeUint(uintBitsize))
}

func (f *decFnInfo) kUint8(rv reflect.Value) {
	rv.SetUint(f.d.d.DecodeUint(8))
}

func (f *decFnInfo) kUint16(rv reflect.Value) {
	rv.SetUint(f.d.d.DecodeUint(16))
}

func (f *decFnInfo) kUint32(rv reflect.Value) {
	rv.SetUint(f.d.d.DecodeUint(32))
}

func (f *decFnInfo) kUint64(rv reflect.Value) {
	rv.SetUint(f.d.d.DecodeUint(64))
}

// func i2rv(i interface{}) reflect.Value {
// 	return reflect.ValueOf(i)
// }
