// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zapcore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapObjectEncoderAdd(t *testing.T) {
	// Expected output of a turducken.
	wantTurducken := map[string]interface{}{
		"ducks": []interface{}{
			map[string]interface{}{"in": "chicken"},
			map[string]interface{}{"in": "chicken"},
		},
	}

	tests := []struct {
		desc     string
		f        func(ObjectEncoder)
		expected interface{}
	}{
		{
			desc: "AddObject",
			f: func(e ObjectEncoder) {
				assert.NoError(t, e.AddObject("k", loggable{true}), "Expected AddObject to succeed.")
			},
			expected: map[string]interface{}{"loggable": "yes"},
		},
		{
			desc: "AddObject (nested)",
			f: func(e ObjectEncoder) {
				assert.NoError(t, e.AddObject("k", turducken{}), "Expected AddObject to succeed.")
			},
			expected: wantTurducken,
		},
		{
			desc: "AddArray",
			f: func(e ObjectEncoder) {
				assert.NoError(t, e.AddArray("k", ArrayMarshalerFunc(func(arr ArrayEncoder) error {
					arr.AppendBool(true)
					arr.AppendBool(false)
					arr.AppendBool(true)
					return nil
				})), "Expected AddArray to succeed.")
			},
			expected: []interface{}{true, false, true},
		},
		{
			desc: "AddArray (nested)",
			f: func(e ObjectEncoder) {
				assert.NoError(t, e.AddArray("k", turduckens(2)), "Expected AddArray to succeed.")
			},
			expected: []interface{}{wantTurducken, wantTurducken},
		},
		{
			desc: "AddArray (empty)",
			f: func(e ObjectEncoder) {
				assert.NoError(t, e.AddArray("k", turduckens(0)), "Expected AddArray to succeed.")
			},
			expected: []interface{}{},
		},
		{
			desc:     "AddBinary",
			f:        func(e ObjectEncoder) { e.AddBinary("k", []byte("foo")) },
			expected: []byte("foo"),
		},
		{
			desc:     "AddByteString",
			f:        func(e ObjectEncoder) { e.AddByteString("k", []byte("foo")) },
			expected: "foo",
		},
		{
			desc:     "AddBool",
			f:        func(e ObjectEncoder) { e.AddBool("k", true) },
			expected: true,
		},
		{
			desc:     "AddComplex128",
			f:        func(e ObjectEncoder) { e.AddComplex128("k", 1+2i) },
			expected: 1 + 2i,
		},
		{
			desc:     "AddComplex64",
			f:        func(e ObjectEncoder) { e.AddComplex64("k", 1+2i) },
			expected: complex64(1 + 2i),
		},
		{
			desc:     "AddDuration",
			f:        func(e ObjectEncoder) { e.AddDuration("k", time.Millisecond) },
			expected: time.Millisecond,
		},
		{
			desc:     "AddFloat64",
			f:        func(e ObjectEncoder) { e.AddFloat64("k", 3.14) },
			expected: 3.14,
		},
		{
			desc:     "AddFloat32",
			f:        func(e ObjectEncoder) { e.AddFloat32("k", 3.14) },
			expected: float32(3.14),
		},
		{
			desc:     "AddInt",
			f:        func(e ObjectEncoder) { e.AddInt("k", 42) },
			expected: 42,
		},
		{
			desc:     "AddInt64",
			f:        func(e ObjectEncoder) { e.AddInt64("k", 42) },
			expected: int64(42),
		},
		{
			desc:     "AddInt32",
			f:        func(e ObjectEncoder) { e.AddInt32("k", 42) },
			expected: int32(42),
		},
		{
			desc:     "AddInt16",
			f:        func(e ObjectEncoder) { e.AddInt16("k", 42) },
			expected: int16(42),
		},
		{
			desc:     "AddInt8",
			f:        func(e ObjectEncoder) { e.AddInt8("k", 42) },
			expected: int8(42),
		},
		{
			desc:     "AddString",
			f:        func(e ObjectEncoder) { e.AddString("k", "v") },
			expected: "v",
		},
		{
			desc:     "AddTime",
			f:        func(e ObjectEncoder) { e.AddTime("k", time.Unix(0, 100)) },
			expected: time.Unix(0, 100),
		},
		{
			desc:     "AddUint",
			f:        func(e ObjectEncoder) { e.AddUint("k", 42) },
			expected: uint(42),
		},
		{
			desc:     "AddUint64",
			f:        func(e ObjectEncoder) { e.AddUint64("k", 42) },
			expected: uint64(42),
		},
		{
			desc:     "AddUint32",
			f:        func(e ObjectEncoder) { e.AddUint32("k", 42) },
			expected: uint32(42),
		},
		{
			desc:     "AddUint16",
			f:        func(e ObjectEncoder) { e.AddUint16("k", 42) },
			expected: uint16(42),
		},
		{
			desc:     "AddUint8",
			f:        func(e ObjectEncoder) { e.AddUint8("k", 42) },
			expected: uint8(42),
		},
		{
			desc:     "AddUintptr",
			f:        func(e ObjectEncoder) { e.AddUintptr("k", 42) },
			expected: uintptr(42),
		},
		{
			desc: "AddReflected",
			f: func(e ObjectEncoder) {
				assert.NoError(t, e.AddReflected("k", map[string]interface{}{"foo": 5}), "Expected AddReflected to succeed.")
			},
			expected: map[string]interface{}{"foo": 5},
		},
		{
			desc: "OpenNamespace",
			f: func(e ObjectEncoder) {
				e.OpenNamespace("k")
				e.AddInt("foo", 1)
				e.OpenNamespace("middle")
				e.AddInt("foo", 2)
				e.OpenNamespace("inner")
				e.AddInt("foo", 3)
			},
			expected: map[string]interface{}{
				"foo": 1,
				"middle": map[string]interface{}{
					"foo": 2,
					"inner": map[string]interface{}{
						"foo": 3,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			enc := NewMapObjectEncoder()
			tt.f(enc)
			assert.Equal(t, tt.expected, enc.Fields["k"], "Unexpected encoder output.")
		})
	}
}
func TestSliceArrayEncoderAppend(t *testing.T) {
	tests := []struct {
		desc     string
		f        func(ArrayEncoder)
		expected interface{}
	}{
		// AppendObject and AppendArray are covered by the AddObject (nested) and
		// AddArray (nested) cases above.
		{"AppendBool", func(e ArrayEncoder) { e.AppendBool(true) }, true},
		{"AppendByteString", func(e ArrayEncoder) { e.AppendByteString([]byte("foo")) }, "foo"},
		{"AppendComplex128", func(e ArrayEncoder) { e.AppendComplex128(1 + 2i) }, 1 + 2i},
		{"AppendComplex64", func(e ArrayEncoder) { e.AppendComplex64(1 + 2i) }, complex64(1 + 2i)},
		{"AppendDuration", func(e ArrayEncoder) { e.AppendDuration(time.Second) }, time.Second},
		{"AppendFloat64", func(e ArrayEncoder) { e.AppendFloat64(3.14) }, 3.14},
		{"AppendFloat32", func(e ArrayEncoder) { e.AppendFloat32(3.14) }, float32(3.14)},
		{"AppendInt", func(e ArrayEncoder) { e.AppendInt(42) }, 42},
		{"AppendInt64", func(e ArrayEncoder) { e.AppendInt64(42) }, int64(42)},
		{"AppendInt32", func(e ArrayEncoder) { e.AppendInt32(42) }, int32(42)},
		{"AppendInt16", func(e ArrayEncoder) { e.AppendInt16(42) }, int16(42)},
		{"AppendInt8", func(e ArrayEncoder) { e.AppendInt8(42) }, int8(42)},
		{"AppendString", func(e ArrayEncoder) { e.AppendString("foo") }, "foo"},
		{"AppendTime", func(e ArrayEncoder) { e.AppendTime(time.Unix(0, 100)) }, time.Unix(0, 100)},
		{"AppendUint", func(e ArrayEncoder) { e.AppendUint(42) }, uint(42)},
		{"AppendUint64", func(e ArrayEncoder) { e.AppendUint64(42) }, uint64(42)},
		{"AppendUint32", func(e ArrayEncoder) { e.AppendUint32(42) }, uint32(42)},
		{"AppendUint16", func(e ArrayEncoder) { e.AppendUint16(42) }, uint16(42)},
		{"AppendUint8", func(e ArrayEncoder) { e.AppendUint8(42) }, uint8(42)},
		{"AppendUintptr", func(e ArrayEncoder) { e.AppendUintptr(42) }, uintptr(42)},
		{
			desc:     "AppendReflected",
			f:        func(e ArrayEncoder) { e.AppendReflected(map[string]interface{}{"foo": 5}) },
			expected: map[string]interface{}{"foo": 5},
		},
		{
			desc: "AppendArray (arrays of arrays)",
			f: func(e ArrayEncoder) {
				e.AppendArray(ArrayMarshalerFunc(func(inner ArrayEncoder) error {
					inner.AppendBool(true)
					inner.AppendBool(false)
					return nil
				}))
			},
			expected: []interface{}{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			enc := NewMapObjectEncoder()
			assert.NoError(t, enc.AddArray("k", ArrayMarshalerFunc(func(arr ArrayEncoder) error {
				tt.f(arr)
				tt.f(arr)
				return nil
			})), "Expected AddArray to succeed.")

			arr, ok := enc.Fields["k"].([]interface{})
			require.True(t, ok, "Test case %s didn't encode an array.", tt.desc)
			assert.Equal(t, []interface{}{tt.expected, tt.expected}, arr, "Unexpected encoder output.")
		})
	}
}

func TestMapObjectEncoderReflectionFailures(t *testing.T) {
	enc := NewMapObjectEncoder()
	assert.Error(t, enc.AddObject("object", loggable{false}), "Expected AddObject to fail.")
	assert.Equal(
		t,
		map[string]interface{}{"object": map[string]interface{}{}},
		enc.Fields,
		"Expected encoder to use empty values on errors.",
	)
}
