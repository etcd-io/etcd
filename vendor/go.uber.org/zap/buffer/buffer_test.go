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

package buffer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferWrites(t *testing.T) {
	buf := NewPool().Get()

	tests := []struct {
		desc string
		f    func()
		want string
	}{
		{"AppendByte", func() { buf.AppendByte('v') }, "v"},
		{"AppendString", func() { buf.AppendString("foo") }, "foo"},
		{"AppendIntPositive", func() { buf.AppendInt(42) }, "42"},
		{"AppendIntNegative", func() { buf.AppendInt(-42) }, "-42"},
		{"AppendUint", func() { buf.AppendUint(42) }, "42"},
		{"AppendBool", func() { buf.AppendBool(true) }, "true"},
		{"AppendFloat64", func() { buf.AppendFloat(3.14, 64) }, "3.14"},
		// Intenationally introduce some floating-point error.
		{"AppendFloat32", func() { buf.AppendFloat(float64(float32(3.14)), 32) }, "3.14"},
		{"AppendWrite", func() { buf.Write([]byte("foo")) }, "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buf.Reset()
			tt.f()
			assert.Equal(t, tt.want, buf.String(), "Unexpected buffer.String().")
			assert.Equal(t, tt.want, string(buf.Bytes()), "Unexpected string(buffer.Bytes()).")
			assert.Equal(t, len(tt.want), buf.Len(), "Unexpected buffer length.")
			// We're not writing more than a kibibyte in tests.
			assert.Equal(t, _size, buf.Cap(), "Expected buffer capacity to remain constant.")
		})
	}
}

func BenchmarkBuffers(b *testing.B) {
	// Because we use the strconv.AppendFoo functions so liberally, we can't
	// use the standard library's bytes.Buffer anyways (without incurring a
	// bunch of extra allocations). Nevertheless, let's make sure that we're
	// not losing any precious nanoseconds.
	str := strings.Repeat("a", 1024)
	slice := make([]byte, 1024)
	buf := bytes.NewBuffer(slice)
	custom := NewPool().Get()
	b.Run("ByteSlice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			slice = append(slice, str...)
			slice = slice[:0]
		}
	})
	b.Run("BytesBuffer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf.WriteString(str)
			buf.Reset()
		}
	})
	b.Run("CustomBuffer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			custom.AppendString(str)
			custom.Reset()
		}
	})
}
