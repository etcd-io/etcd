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
	"sync"
	"testing"

	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
)

func TestLevelEnablerFunc(t *testing.T) {
	enab := LevelEnablerFunc(func(l zapcore.Level) bool { return l == zapcore.InfoLevel })
	tests := []struct {
		level   zapcore.Level
		enabled bool
	}{
		{DebugLevel, false},
		{InfoLevel, true},
		{WarnLevel, false},
		{ErrorLevel, false},
		{DPanicLevel, false},
		{PanicLevel, false},
		{FatalLevel, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.enabled, enab.Enabled(tt.level), "Unexpected result applying LevelEnablerFunc to %s", tt.level)
	}
}

func TestNewAtomicLevel(t *testing.T) {
	lvl := NewAtomicLevel()
	assert.Equal(t, InfoLevel, lvl.Level(), "Unexpected initial level.")
	lvl.SetLevel(ErrorLevel)
	assert.Equal(t, ErrorLevel, lvl.Level(), "Unexpected level after SetLevel.")
	lvl = NewAtomicLevelAt(WarnLevel)
	assert.Equal(t, WarnLevel, lvl.Level(), "Unexpected level after SetLevel.")
}

func TestAtomicLevelMutation(t *testing.T) {
	lvl := NewAtomicLevel()
	lvl.SetLevel(WarnLevel)
	// Trigger races for non-atomic level mutations.
	proceed := make(chan struct{})
	wg := &sync.WaitGroup{}
	runConcurrently(10, 100, wg, func() {
		<-proceed
		assert.Equal(t, WarnLevel, lvl.Level())
	})
	runConcurrently(10, 100, wg, func() {
		<-proceed
		lvl.SetLevel(WarnLevel)
	})
	close(proceed)
	wg.Wait()
}

func TestAtomicLevelText(t *testing.T) {
	tests := []struct {
		text   string
		expect zapcore.Level
		err    bool
	}{
		{"debug", DebugLevel, false},
		{"info", InfoLevel, false},
		{"", InfoLevel, false},
		{"warn", WarnLevel, false},
		{"error", ErrorLevel, false},
		{"dpanic", DPanicLevel, false},
		{"panic", PanicLevel, false},
		{"fatal", FatalLevel, false},
		{"foobar", InfoLevel, true},
	}

	for _, tt := range tests {
		var lvl AtomicLevel
		// Test both initial unmarshaling and overwriting existing value.
		for i := 0; i < 2; i++ {
			if tt.err {
				assert.Error(t, lvl.UnmarshalText([]byte(tt.text)), "Expected unmarshaling %q to fail.", tt.text)
			} else {
				assert.NoError(t, lvl.UnmarshalText([]byte(tt.text)), "Expected unmarshaling %q to succeed.", tt.text)
			}
			assert.Equal(t, tt.expect, lvl.Level(), "Unexpected level after unmarshaling.")
			lvl.SetLevel(InfoLevel)
		}

		// Test marshalling
		if tt.text != "" && !tt.err {
			lvl.SetLevel(tt.expect)
			marshaled, err := lvl.MarshalText()
			assert.NoError(t, err, `Unexpected error marshalling level "%v" to text.`, tt.expect)
			assert.Equal(t, tt.text, string(marshaled), "Expected marshaled text to match")
			assert.Equal(t, tt.text, lvl.String(), "Expected Stringer call to match")
		}
	}
}
