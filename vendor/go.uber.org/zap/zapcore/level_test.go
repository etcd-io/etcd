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
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelString(t *testing.T) {
	tests := map[Level]string{
		DebugLevel:  "debug",
		InfoLevel:   "info",
		WarnLevel:   "warn",
		ErrorLevel:  "error",
		DPanicLevel: "dpanic",
		PanicLevel:  "panic",
		FatalLevel:  "fatal",
		Level(-42):  "Level(-42)",
	}

	for lvl, stringLevel := range tests {
		assert.Equal(t, stringLevel, lvl.String(), "Unexpected lowercase level string.")
		assert.Equal(t, strings.ToUpper(stringLevel), lvl.CapitalString(), "Unexpected all-caps level string.")
	}
}

func TestLevelText(t *testing.T) {
	tests := []struct {
		text  string
		level Level
	}{
		{"debug", DebugLevel},
		{"info", InfoLevel},
		{"", InfoLevel}, // make the zero value useful
		{"warn", WarnLevel},
		{"error", ErrorLevel},
		{"dpanic", DPanicLevel},
		{"panic", PanicLevel},
		{"fatal", FatalLevel},
	}
	for _, tt := range tests {
		if tt.text != "" {
			lvl := tt.level
			marshaled, err := lvl.MarshalText()
			assert.NoError(t, err, "Unexpected error marshaling level %v to text.", &lvl)
			assert.Equal(t, tt.text, string(marshaled), "Marshaling level %v to text yielded unexpected result.", &lvl)
		}

		var unmarshaled Level
		err := unmarshaled.UnmarshalText([]byte(tt.text))
		assert.NoError(t, err, `Unexpected error unmarshaling text %q to level.`, tt.text)
		assert.Equal(t, tt.level, unmarshaled, `Text %q unmarshaled to an unexpected level.`, tt.text)
	}
}

func TestCapitalLevelsParse(t *testing.T) {
	tests := []struct {
		text  string
		level Level
	}{
		{"DEBUG", DebugLevel},
		{"INFO", InfoLevel},
		{"WARN", WarnLevel},
		{"ERROR", ErrorLevel},
		{"DPANIC", DPanicLevel},
		{"PANIC", PanicLevel},
		{"FATAL", FatalLevel},
	}
	for _, tt := range tests {
		var unmarshaled Level
		err := unmarshaled.UnmarshalText([]byte(tt.text))
		assert.NoError(t, err, `Unexpected error unmarshaling text %q to level.`, tt.text)
		assert.Equal(t, tt.level, unmarshaled, `Text %q unmarshaled to an unexpected level.`, tt.text)
	}
}

func TestWeirdLevelsParse(t *testing.T) {
	tests := []struct {
		text  string
		level Level
	}{
		// I guess...
		{"Debug", DebugLevel},
		{"Info", InfoLevel},
		{"Warn", WarnLevel},
		{"Error", ErrorLevel},
		{"Dpanic", DPanicLevel},
		{"Panic", PanicLevel},
		{"Fatal", FatalLevel},

		// What even is...
		{"DeBuG", DebugLevel},
		{"InFo", InfoLevel},
		{"WaRn", WarnLevel},
		{"ErRor", ErrorLevel},
		{"DpAnIc", DPanicLevel},
		{"PaNiC", PanicLevel},
		{"FaTaL", FatalLevel},
	}
	for _, tt := range tests {
		var unmarshaled Level
		err := unmarshaled.UnmarshalText([]byte(tt.text))
		assert.NoError(t, err, `Unexpected error unmarshaling text %q to level.`, tt.text)
		assert.Equal(t, tt.level, unmarshaled, `Text %q unmarshaled to an unexpected level.`, tt.text)
	}
}

func TestLevelNils(t *testing.T) {
	var l *Level

	// The String() method will not handle nil level properly.
	assert.Panics(t, func() {
		assert.Equal(t, "Level(nil)", l.String(), "Unexpected result stringifying nil *Level.")
	}, "Level(nil).String() should panic")

	assert.Panics(t, func() {
		l.MarshalText()
	}, "Expected to panic when marshalling a nil level.")

	err := l.UnmarshalText([]byte("debug"))
	assert.Equal(t, errUnmarshalNilLevel, err, "Expected to error unmarshalling into a nil Level.")
}

func TestLevelUnmarshalUnknownText(t *testing.T) {
	var l Level
	err := l.UnmarshalText([]byte("foo"))
	assert.Contains(t, err.Error(), "unrecognized level", "Expected unmarshaling arbitrary text to fail.")
}

func TestLevelAsFlagValue(t *testing.T) {
	var (
		buf bytes.Buffer
		lvl Level
	)
	fs := flag.NewFlagSet("levelTest", flag.ContinueOnError)
	fs.SetOutput(&buf)
	fs.Var(&lvl, "level", "log level")

	for _, expected := range []Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, DPanicLevel, PanicLevel, FatalLevel} {
		assert.NoError(t, fs.Parse([]string{"-level", expected.String()}))
		assert.Equal(t, expected, lvl, "Unexpected level after parsing flag.")
		assert.Equal(t, expected, lvl.Get(), "Unexpected output using flag.Getter API.")
		assert.Empty(t, buf.String(), "Unexpected error output parsing level flag.")
		buf.Reset()
	}

	assert.Error(t, fs.Parse([]string{"-level", "nope"}))
	assert.Equal(
		t,
		`invalid value "nope" for flag -level: unrecognized level: "nope"`,
		strings.Split(buf.String(), "\n")[0], // second line is help message
		"Unexpected error output from invalid flag input.",
	)
}
