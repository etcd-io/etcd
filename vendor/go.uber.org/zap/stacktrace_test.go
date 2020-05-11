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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTakeStacktrace(t *testing.T) {
	trace := takeStacktrace()
	lines := strings.Split(trace, "\n")
	require.True(t, len(lines) > 0, "Expected stacktrace to have at least one frame.")
	assert.Contains(
		t,
		lines[0],
		"testing.",
		"Expected stacktrace to start with the test runner (zap frames are filtered out) %s.", lines[0],
	)
}

func TestIsZapFrame(t *testing.T) {
	zapFrames := []string{
		"go.uber.org/zap.Stack",
		"go.uber.org/zap.(*SugaredLogger).log",
		"go.uber.org/zap/zapcore.(ArrayMarshalerFunc).MarshalLogArray",
		"github.com/uber/tchannel-go/vendor/go.uber.org/zap.Stack",
		"github.com/uber/tchannel-go/vendor/go.uber.org/zap.(*SugaredLogger).log",
		"github.com/uber/tchannel-go/vendor/go.uber.org/zap/zapcore.(ArrayMarshalerFunc).MarshalLogArray",
	}
	nonZapFrames := []string{
		"github.com/uber/tchannel-go.NewChannel",
		"go.uber.org/not-zap.New",
		"go.uber.org/zapext.ctx",
		"go.uber.org/zap_ext/ctx.New",
	}

	t.Run("zap frames", func(t *testing.T) {
		for _, f := range zapFrames {
			require.True(t, isZapFrame(f), f)
		}
	})
	t.Run("non-zap frames", func(t *testing.T) {
		for _, f := range nonZapFrames {
			require.False(t, isZapFrame(f), f)
		}
	})
}

func BenchmarkTakeStacktrace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		takeStacktrace()
	}
}
