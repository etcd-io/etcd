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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.uber.org/zap/zapcore"
)

var (
	_epoch     = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	_testEntry = Entry{
		LoggerName: "main",
		Level:      InfoLevel,
		Message:    `hello`,
		Time:       _epoch,
		Stack:      "fake-stack",
		Caller:     EntryCaller{Defined: true, File: "foo.go", Line: 42},
	}
)

func testEncoderConfig() EncoderConfig {
	return EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "name",
		TimeKey:        "ts",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		LineEnding:     "\n",
		EncodeTime:     EpochTimeEncoder,
		EncodeLevel:    LowercaseLevelEncoder,
		EncodeDuration: SecondsDurationEncoder,
		EncodeCaller:   ShortCallerEncoder,
	}
}

func humanEncoderConfig() EncoderConfig {
	cfg := testEncoderConfig()
	cfg.EncodeTime = ISO8601TimeEncoder
	cfg.EncodeLevel = CapitalLevelEncoder
	cfg.EncodeDuration = StringDurationEncoder
	return cfg
}

func withJSONEncoder(f func(Encoder)) {
	f(NewJSONEncoder(testEncoderConfig()))
}

func withConsoleEncoder(f func(Encoder)) {
	f(NewConsoleEncoder(humanEncoderConfig()))
}

func capitalNameEncoder(loggerName string, enc PrimitiveArrayEncoder) {
	enc.AppendString(strings.ToUpper(loggerName))
}

func TestEncoderConfiguration(t *testing.T) {
	base := testEncoderConfig()

	tests := []struct {
		desc            string
		cfg             EncoderConfig
		amendEntry      func(Entry) Entry
		extra           func(Encoder)
		expectedJSON    string
		expectedConsole string
	}{
		{
			desc: "messages to be escaped",
			cfg:  base,
			amendEntry: func(ent Entry) Entry {
				ent.Message = `hello\`
				return ent
			},
			expectedJSON:    `{"level":"info","ts":0,"name":"main","caller":"foo.go:42","msg":"hello\\","stacktrace":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\\\nfake-stack\n",
		},
		{
			desc: "use custom entry keys in JSON output and ignore them in console output",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "skip level if LevelKey is omitted",
			cfg: EncoderConfig{
				LevelKey:       "",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tmain\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "skip timestamp if TimeKey is omitted",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "info\tmain\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "skip message if MessageKey is omitted",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\nfake-stack\n",
		},
		{
			desc: "skip name if NameKey is omitted",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "skip caller if CallerKey is omitted",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\thello\nfake-stack\n",
		},
		{
			desc: "skip stacktrace if StacktraceKey is omitted",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello"}` + "\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\n",
		},
		{
			desc: "use the supplied EncodeTime, for both the entry and any times added",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     func(t time.Time, enc PrimitiveArrayEncoder) { enc.AppendString(t.String()) },
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			extra: func(enc Encoder) {
				enc.AddTime("extra", _epoch)
				enc.AddArray("extras", ArrayMarshalerFunc(func(enc ArrayEncoder) error {
					enc.AppendTime(_epoch)
					return nil
				}))
			},
			expectedJSON: `{"L":"info","T":"1970-01-01 00:00:00 +0000 UTC","N":"main","C":"foo.go:42","M":"hello","extra":"1970-01-01 00:00:00 +0000 UTC","extras":["1970-01-01 00:00:00 +0000 UTC"],"S":"fake-stack"}` + "\n",
			expectedConsole: "1970-01-01 00:00:00 +0000 UTC\tinfo\tmain\tfoo.go:42\thello\t" + // plain-text preamble
				`{"extra": "1970-01-01 00:00:00 +0000 UTC", "extras": ["1970-01-01 00:00:00 +0000 UTC"]}` + // JSON context
				"\nfake-stack\n", // stacktrace after newline
		},
		{
			desc: "use the supplied EncodeDuration for any durations added",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: StringDurationEncoder,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			extra: func(enc Encoder) {
				enc.AddDuration("extra", time.Second)
				enc.AddArray("extras", ArrayMarshalerFunc(func(enc ArrayEncoder) error {
					enc.AppendDuration(time.Minute)
					return nil
				}))
			},
			expectedJSON: `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","extra":"1s","extras":["1m0s"],"S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\t" + // preamble
				`{"extra": "1s", "extras": ["1m0s"]}` + // context
				"\nfake-stack\n", // stacktrace
		},
		{
			desc: "use the supplied EncodeLevel",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    CapitalLevelEncoder,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"INFO","T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tINFO\tmain\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "use the supplied EncodeName",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
				EncodeName:     capitalNameEncoder,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"MAIN","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tMAIN\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "close all open namespaces",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			extra: func(enc Encoder) {
				enc.OpenNamespace("outer")
				enc.OpenNamespace("inner")
				enc.AddString("foo", "bar")
				enc.OpenNamespace("innermost")
			},
			expectedJSON: `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","outer":{"inner":{"foo":"bar","innermost":{}}},"S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\t" +
				`{"outer": {"inner": {"foo": "bar", "innermost": {}}}}` +
				"\nfake-stack\n",
		},
		{
			desc: "handle no-op EncodeTime",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     func(time.Time, PrimitiveArrayEncoder) {},
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			extra:           func(enc Encoder) { enc.AddTime("sometime", time.Unix(0, 100)) },
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","sometime":100,"S":"fake-stack"}` + "\n",
			expectedConsole: "info\tmain\tfoo.go:42\thello\t" + `{"sometime": 100}` + "\nfake-stack\n",
		},
		{
			desc: "handle no-op EncodeDuration",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: func(time.Duration, PrimitiveArrayEncoder) {},
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			extra:           func(enc Encoder) { enc.AddDuration("someduration", time.Microsecond) },
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","someduration":1000,"S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\t" + `{"someduration": 1000}` + "\nfake-stack\n",
		},
		{
			desc: "handle no-op EncodeLevel",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    func(Level, PrimitiveArrayEncoder) {},
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tmain\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "handle no-op EncodeCaller",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   func(EntryCaller, PrimitiveArrayEncoder) {},
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tmain\thello\nfake-stack\n",
		},
		{
			desc: "handle no-op EncodeName",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     base.LineEnding,
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
				EncodeName:     func(string, PrimitiveArrayEncoder) {},
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\n",
			expectedConsole: "0\tinfo\tfoo.go:42\thello\nfake-stack\n",
		},
		{
			desc: "use custom line separator",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				LineEnding:     "\r\n",
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + "\r\n",
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\nfake-stack\r\n",
		},
		{
			desc: "omit line separator definition - fall back to default",
			cfg: EncoderConfig{
				LevelKey:       "L",
				TimeKey:        "T",
				MessageKey:     "M",
				NameKey:        "N",
				CallerKey:      "C",
				StacktraceKey:  "S",
				EncodeTime:     base.EncodeTime,
				EncodeDuration: base.EncodeDuration,
				EncodeLevel:    base.EncodeLevel,
				EncodeCaller:   base.EncodeCaller,
			},
			expectedJSON:    `{"L":"info","T":0,"N":"main","C":"foo.go:42","M":"hello","S":"fake-stack"}` + DefaultLineEnding,
			expectedConsole: "0\tinfo\tmain\tfoo.go:42\thello\nfake-stack" + DefaultLineEnding,
		},
	}

	for i, tt := range tests {
		json := NewJSONEncoder(tt.cfg)
		console := NewConsoleEncoder(tt.cfg)
		if tt.extra != nil {
			tt.extra(json)
			tt.extra(console)
		}
		entry := _testEntry
		if tt.amendEntry != nil {
			entry = tt.amendEntry(_testEntry)
		}
		jsonOut, jsonErr := json.EncodeEntry(entry, nil)
		if assert.NoError(t, jsonErr, "Unexpected error JSON-encoding entry in case #%d.", i) {
			assert.Equal(
				t,
				tt.expectedJSON,
				jsonOut.String(),
				"Unexpected JSON output: expected to %v.", tt.desc,
			)
		}
		consoleOut, consoleErr := console.EncodeEntry(entry, nil)
		if assert.NoError(t, consoleErr, "Unexpected error console-encoding entry in case #%d.", i) {
			assert.Equal(
				t,
				tt.expectedConsole,
				consoleOut.String(),
				"Unexpected console output: expected to %v.", tt.desc,
			)
		}
	}
}

func TestLevelEncoders(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{} // output of encoding InfoLevel
	}{
		{"capital", "INFO"},
		{"lower", "info"},
		{"", "info"},
		{"something-random", "info"},
	}

	for _, tt := range tests {
		var le LevelEncoder
		require.NoError(t, le.UnmarshalText([]byte(tt.name)), "Unexpected error unmarshaling %q.", tt.name)
		assertAppended(
			t,
			tt.expected,
			func(arr ArrayEncoder) { le(InfoLevel, arr) },
			"Unexpected output serializing InfoLevel with %q.", tt.name,
		)
	}
}

func TestTimeEncoders(t *testing.T) {
	moment := time.Unix(100, 50005000).UTC()
	tests := []struct {
		name     string
		expected interface{} // output of serializing moment
	}{
		{"iso8601", "1970-01-01T00:01:40.050Z"},
		{"ISO8601", "1970-01-01T00:01:40.050Z"},
		{"millis", 100050.005},
		{"nanos", int64(100050005000)},
		{"", 100.050005},
		{"something-random", 100.050005},
	}

	for _, tt := range tests {
		var te TimeEncoder
		require.NoError(t, te.UnmarshalText([]byte(tt.name)), "Unexpected error unmarshaling %q.", tt.name)
		assertAppended(
			t,
			tt.expected,
			func(arr ArrayEncoder) { te(moment, arr) },
			"Unexpected output serializing %v with %q.", moment, tt.name,
		)
	}
}

func TestDurationEncoders(t *testing.T) {
	elapsed := time.Second + 500*time.Nanosecond
	tests := []struct {
		name     string
		expected interface{} // output of serializing elapsed
	}{
		{"string", "1.0000005s"},
		{"nanos", int64(1000000500)},
		{"", 1.0000005},
		{"something-random", 1.0000005},
	}

	for _, tt := range tests {
		var de DurationEncoder
		require.NoError(t, de.UnmarshalText([]byte(tt.name)), "Unexpected error unmarshaling %q.", tt.name)
		assertAppended(
			t,
			tt.expected,
			func(arr ArrayEncoder) { de(elapsed, arr) },
			"Unexpected output serializing %v with %q.", elapsed, tt.name,
		)
	}
}

func TestCallerEncoders(t *testing.T) {
	caller := EntryCaller{Defined: true, File: "/home/jack/src/github.com/foo/foo.go", Line: 42}
	tests := []struct {
		name     string
		expected interface{} // output of serializing caller
	}{
		{"", "foo/foo.go:42"},
		{"something-random", "foo/foo.go:42"},
		{"short", "foo/foo.go:42"},
		{"full", "/home/jack/src/github.com/foo/foo.go:42"},
	}

	for _, tt := range tests {
		var ce CallerEncoder
		require.NoError(t, ce.UnmarshalText([]byte(tt.name)), "Unexpected error unmarshaling %q.", tt.name)
		assertAppended(
			t,
			tt.expected,
			func(arr ArrayEncoder) { ce(caller, arr) },
			"Unexpected output serializing file name as %v with %q.", tt.expected, tt.name,
		)
	}
}

func TestNameEncoders(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{} // output of encoding InfoLevel
	}{
		{"", "main"},
		{"full", "main"},
		{"something-random", "main"},
	}

	for _, tt := range tests {
		var ne NameEncoder
		require.NoError(t, ne.UnmarshalText([]byte(tt.name)), "Unexpected error unmarshaling %q.", tt.name)
		assertAppended(
			t,
			tt.expected,
			func(arr ArrayEncoder) { ne("main", arr) },
			"Unexpected output serializing logger name with %q.", tt.name,
		)
	}
}

func assertAppended(t testing.TB, expected interface{}, f func(ArrayEncoder), msgAndArgs ...interface{}) {
	mem := NewMapObjectEncoder()
	mem.AddArray("k", ArrayMarshalerFunc(func(arr ArrayEncoder) error {
		f(arr)
		return nil
	}))
	arr := mem.Fields["k"].([]interface{})
	require.Equal(t, 1, len(arr), "Expected to append exactly one element to array.")
	assert.Equal(t, expected, arr[0], msgAndArgs...)
}
