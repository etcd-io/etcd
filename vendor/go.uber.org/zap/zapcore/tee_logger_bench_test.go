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

package zapcore_test

import (
	"testing"

	"go.uber.org/zap/internal/ztest"
	. "go.uber.org/zap/zapcore"
)

func withBenchedTee(b *testing.B, f func(Core)) {
	fac := NewTee(
		NewCore(NewJSONEncoder(testEncoderConfig()), &ztest.Discarder{}, DebugLevel),
		NewCore(NewJSONEncoder(testEncoderConfig()), &ztest.Discarder{}, InfoLevel),
	)
	b.ResetTimer()
	f(fac)
}

func BenchmarkTeeCheck(b *testing.B) {
	cases := []struct {
		lvl Level
		msg string
	}{
		{DebugLevel, "foo"},
		{InfoLevel, "bar"},
		{WarnLevel, "baz"},
		{ErrorLevel, "babble"},
	}
	withBenchedTee(b, func(core Core) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				tt := cases[i]
				entry := Entry{Level: tt.lvl, Message: tt.msg}
				if cm := core.Check(entry, nil); cm != nil {
					cm.Write(Field{Key: "i", Integer: int64(i), Type: Int64Type})
				}
				i = (i + 1) % len(cases)
			}
		})
	})
}
