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

package zap

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

type username string

func (n username) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("username", string(n))
	return nil
}

func assertCanBeReused(t testing.TB, field Field) {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		enc := zapcore.NewMapObjectEncoder()

		// Ensure using the field in multiple encoders in separate goroutines
		// does not cause any races or panics.
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotPanics(t, func() {
				field.AddTo(enc)
			}, "Reusing a field should not cause issues")
		}()
	}

	wg.Wait()
}

func TestFieldConstructors(t *testing.T) {
	// Interface types.
	addr := net.ParseIP("1.2.3.4")
	name := username("phil")
	ints := []int{5, 6}

	tests := []struct {
		name   string
		field  Field
		expect Field
	}{
		{"Skip", Field{Type: zapcore.SkipType}, Skip()},
		{"Binary", Field{Key: "k", Type: zapcore.BinaryType, Interface: []byte("ab12")}, Binary("k", []byte("ab12"))},
		{"Bool", Field{Key: "k", Type: zapcore.BoolType, Integer: 1}, Bool("k", true)},
		{"Bool", Field{Key: "k", Type: zapcore.BoolType, Integer: 1}, Bool("k", true)},
		{"ByteString", Field{Key: "k", Type: zapcore.ByteStringType, Interface: []byte("ab12")}, ByteString("k", []byte("ab12"))},
		{"Complex128", Field{Key: "k", Type: zapcore.Complex128Type, Interface: 1 + 2i}, Complex128("k", 1+2i)},
		{"Complex64", Field{Key: "k", Type: zapcore.Complex64Type, Interface: complex64(1 + 2i)}, Complex64("k", 1+2i)},
		{"Duration", Field{Key: "k", Type: zapcore.DurationType, Integer: 1}, Duration("k", 1)},
		{"Int", Field{Key: "k", Type: zapcore.Int64Type, Integer: 1}, Int("k", 1)},
		{"Int64", Field{Key: "k", Type: zapcore.Int64Type, Integer: 1}, Int64("k", 1)},
		{"Int32", Field{Key: "k", Type: zapcore.Int32Type, Integer: 1}, Int32("k", 1)},
		{"Int16", Field{Key: "k", Type: zapcore.Int16Type, Integer: 1}, Int16("k", 1)},
		{"Int8", Field{Key: "k", Type: zapcore.Int8Type, Integer: 1}, Int8("k", 1)},
		{"String", Field{Key: "k", Type: zapcore.StringType, String: "foo"}, String("k", "foo")},
		{"Time", Field{Key: "k", Type: zapcore.TimeType, Integer: 0, Interface: time.UTC}, Time("k", time.Unix(0, 0).In(time.UTC))},
		{"Time", Field{Key: "k", Type: zapcore.TimeType, Integer: 1000, Interface: time.UTC}, Time("k", time.Unix(0, 1000).In(time.UTC))},
		{"Uint", Field{Key: "k", Type: zapcore.Uint64Type, Integer: 1}, Uint("k", 1)},
		{"Uint64", Field{Key: "k", Type: zapcore.Uint64Type, Integer: 1}, Uint64("k", 1)},
		{"Uint32", Field{Key: "k", Type: zapcore.Uint32Type, Integer: 1}, Uint32("k", 1)},
		{"Uint16", Field{Key: "k", Type: zapcore.Uint16Type, Integer: 1}, Uint16("k", 1)},
		{"Uint8", Field{Key: "k", Type: zapcore.Uint8Type, Integer: 1}, Uint8("k", 1)},
		{"Uintptr", Field{Key: "k", Type: zapcore.UintptrType, Integer: 10}, Uintptr("k", 0xa)},
		{"Reflect", Field{Key: "k", Type: zapcore.ReflectType, Interface: ints}, Reflect("k", ints)},
		{"Stringer", Field{Key: "k", Type: zapcore.StringerType, Interface: addr}, Stringer("k", addr)},
		{"Object", Field{Key: "k", Type: zapcore.ObjectMarshalerType, Interface: name}, Object("k", name)},
		{"Any:ObjectMarshaler", Any("k", name), Object("k", name)},
		{"Any:ArrayMarshaler", Any("k", bools([]bool{true})), Array("k", bools([]bool{true}))},
		{"Any:Stringer", Any("k", addr), Stringer("k", addr)},
		{"Any:Bool", Any("k", true), Bool("k", true)},
		{"Any:Bools", Any("k", []bool{true}), Bools("k", []bool{true})},
		{"Any:Byte", Any("k", byte(1)), Uint8("k", 1)},
		{"Any:Bytes", Any("k", []byte{1}), Binary("k", []byte{1})},
		{"Any:Complex128", Any("k", 1+2i), Complex128("k", 1+2i)},
		{"Any:Complex128s", Any("k", []complex128{1 + 2i}), Complex128s("k", []complex128{1 + 2i})},
		{"Any:Complex64", Any("k", complex64(1+2i)), Complex64("k", 1+2i)},
		{"Any:Complex64s", Any("k", []complex64{1 + 2i}), Complex64s("k", []complex64{1 + 2i})},
		{"Any:Float64", Any("k", 3.14), Float64("k", 3.14)},
		{"Any:Float64s", Any("k", []float64{3.14}), Float64s("k", []float64{3.14})},
		{"Any:Float32", Any("k", float32(3.14)), Float32("k", 3.14)},
		{"Any:Float32s", Any("k", []float32{3.14}), Float32s("k", []float32{3.14})},
		{"Any:Int", Any("k", 1), Int("k", 1)},
		{"Any:Ints", Any("k", []int{1}), Ints("k", []int{1})},
		{"Any:Int64", Any("k", int64(1)), Int64("k", 1)},
		{"Any:Int64s", Any("k", []int64{1}), Int64s("k", []int64{1})},
		{"Any:Int32", Any("k", int32(1)), Int32("k", 1)},
		{"Any:Int32s", Any("k", []int32{1}), Int32s("k", []int32{1})},
		{"Any:Int16", Any("k", int16(1)), Int16("k", 1)},
		{"Any:Int16s", Any("k", []int16{1}), Int16s("k", []int16{1})},
		{"Any:Int8", Any("k", int8(1)), Int8("k", 1)},
		{"Any:Int8s", Any("k", []int8{1}), Int8s("k", []int8{1})},
		{"Any:Rune", Any("k", rune(1)), Int32("k", 1)},
		{"Any:Runes", Any("k", []rune{1}), Int32s("k", []int32{1})},
		{"Any:String", Any("k", "v"), String("k", "v")},
		{"Any:Strings", Any("k", []string{"v"}), Strings("k", []string{"v"})},
		{"Any:Uint", Any("k", uint(1)), Uint("k", 1)},
		{"Any:Uints", Any("k", []uint{1}), Uints("k", []uint{1})},
		{"Any:Uint64", Any("k", uint64(1)), Uint64("k", 1)},
		{"Any:Uint64s", Any("k", []uint64{1}), Uint64s("k", []uint64{1})},
		{"Any:Uint32", Any("k", uint32(1)), Uint32("k", 1)},
		{"Any:Uint32s", Any("k", []uint32{1}), Uint32s("k", []uint32{1})},
		{"Any:Uint16", Any("k", uint16(1)), Uint16("k", 1)},
		{"Any:Uint16s", Any("k", []uint16{1}), Uint16s("k", []uint16{1})},
		{"Any:Uint8", Any("k", uint8(1)), Uint8("k", 1)},
		{"Any:Uint8s", Any("k", []uint8{1}), Binary("k", []uint8{1})},
		{"Any:Uintptr", Any("k", uintptr(1)), Uintptr("k", 1)},
		{"Any:Uintptrs", Any("k", []uintptr{1}), Uintptrs("k", []uintptr{1})},
		{"Any:Time", Any("k", time.Unix(0, 0)), Time("k", time.Unix(0, 0))},
		{"Any:Times", Any("k", []time.Time{time.Unix(0, 0)}), Times("k", []time.Time{time.Unix(0, 0)})},
		{"Any:Duration", Any("k", time.Second), Duration("k", time.Second)},
		{"Any:Durations", Any("k", []time.Duration{time.Second}), Durations("k", []time.Duration{time.Second})},
		{"Any:Fallback", Any("k", struct{}{}), Reflect("k", struct{}{})},
		{"Namespace", Namespace("k"), Field{Key: "k", Type: zapcore.NamespaceType}},
	}

	for _, tt := range tests {
		if !assert.Equal(t, tt.expect, tt.field, "Unexpected output from convenience field constructor %s.", tt.name) {
			t.Logf("type expected: %T\nGot: %T", tt.expect.Interface, tt.field.Interface)
		}
		assertCanBeReused(t, tt.field)
	}
}

func TestStackField(t *testing.T) {
	f := Stack("stacktrace")
	assert.Equal(t, "stacktrace", f.Key, "Unexpected field key.")
	assert.Equal(t, zapcore.StringType, f.Type, "Unexpected field type.")
	assert.Equal(t, takeStacktrace(), f.String, "Unexpected stack trace")
	assertCanBeReused(t, f)
}
