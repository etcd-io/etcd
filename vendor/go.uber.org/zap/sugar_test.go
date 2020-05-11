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
	"testing"

	"go.uber.org/zap/internal/exit"
	"go.uber.org/zap/internal/ztest"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSugarWith(t *testing.T) {
	// Convenience functions to create expected error logs.
	ignored := func(msg interface{}) observer.LoggedEntry {
		return observer.LoggedEntry{
			Entry:   zapcore.Entry{Level: DPanicLevel, Message: _oddNumberErrMsg},
			Context: []Field{Any("ignored", msg)},
		}
	}
	nonString := func(pairs ...invalidPair) observer.LoggedEntry {
		return observer.LoggedEntry{
			Entry:   zapcore.Entry{Level: DPanicLevel, Message: _nonStringKeyErrMsg},
			Context: []Field{Array("invalid", invalidPairs(pairs))},
		}
	}

	tests := []struct {
		desc     string
		args     []interface{}
		expected []Field
		errLogs  []observer.LoggedEntry
	}{
		{
			desc:     "nil args",
			args:     nil,
			expected: []Field{},
			errLogs:  nil,
		},
		{
			desc:     "empty slice of args",
			args:     []interface{}{},
			expected: []Field{},
			errLogs:  nil,
		},
		{
			desc:     "just a dangling key",
			args:     []interface{}{"should ignore"},
			expected: []Field{},
			errLogs:  []observer.LoggedEntry{ignored("should ignore")},
		},
		{
			desc:     "well-formed key-value pairs",
			args:     []interface{}{"foo", 42, "true", "bar"},
			expected: []Field{Int("foo", 42), String("true", "bar")},
			errLogs:  nil,
		},
		{
			desc:     "just a structured field",
			args:     []interface{}{Int("foo", 42)},
			expected: []Field{Int("foo", 42)},
			errLogs:  nil,
		},
		{
			desc:     "structured field and a dangling key",
			args:     []interface{}{Int("foo", 42), "dangling"},
			expected: []Field{Int("foo", 42)},
			errLogs:  []observer.LoggedEntry{ignored("dangling")},
		},
		{
			desc:     "structured field and a dangling non-string key",
			args:     []interface{}{Int("foo", 42), 13},
			expected: []Field{Int("foo", 42)},
			errLogs:  []observer.LoggedEntry{ignored(13)},
		},
		{
			desc:     "key-value pair and a dangling key",
			args:     []interface{}{"foo", 42, "dangling"},
			expected: []Field{Int("foo", 42)},
			errLogs:  []observer.LoggedEntry{ignored("dangling")},
		},
		{
			desc:     "pairs, a structured field, and a dangling key",
			args:     []interface{}{"first", "field", Int("foo", 42), "baz", "quux", "dangling"},
			expected: []Field{String("first", "field"), Int("foo", 42), String("baz", "quux")},
			errLogs:  []observer.LoggedEntry{ignored("dangling")},
		},
		{
			desc:     "one non-string key",
			args:     []interface{}{"foo", 42, true, "bar"},
			expected: []Field{Int("foo", 42)},
			errLogs:  []observer.LoggedEntry{nonString(invalidPair{2, true, "bar"})},
		},
		{
			desc:     "pairs, structured fields, non-string keys, and a dangling key",
			args:     []interface{}{"foo", 42, true, "bar", Int("structure", 11), 42, "reversed", "baz", "quux", "dangling"},
			expected: []Field{Int("foo", 42), Int("structure", 11), String("baz", "quux")},
			errLogs: []observer.LoggedEntry{
				ignored("dangling"),
				nonString(invalidPair{2, true, "bar"}, invalidPair{5, 42, "reversed"}),
			},
		},
	}

	for _, tt := range tests {
		withSugar(t, DebugLevel, nil, func(logger *SugaredLogger, logs *observer.ObservedLogs) {
			logger.With(tt.args...).Info("")
			output := logs.AllUntimed()
			if len(tt.errLogs) > 0 {
				for i := range tt.errLogs {
					assert.Equal(t, tt.errLogs[i], output[i], "Unexpected error log at position %d for scenario %s.", i, tt.desc)
				}
			}
			assert.Equal(t, len(tt.errLogs)+1, len(output), "Expected only one non-error message to be logged in scenario %s.", tt.desc)
			assert.Equal(t, tt.expected, output[len(tt.errLogs)].Context, "Unexpected message context in scenario %s.", tt.desc)
		})
	}
}

func TestSugarFieldsInvalidPairs(t *testing.T) {
	withSugar(t, DebugLevel, nil, func(logger *SugaredLogger, logs *observer.ObservedLogs) {
		logger.With(42, "foo", []string{"bar"}, "baz").Info("")
		output := logs.AllUntimed()

		// Double-check that the actual message was logged.
		require.Equal(t, 2, len(output), "Unexpected number of entries logged.")
		require.Equal(t, observer.LoggedEntry{Context: []Field{}}, output[1], "Unexpected non-error log entry.")

		// Assert that the error message's structured fields serialize properly.
		require.Equal(t, 1, len(output[0].Context), "Expected one field in error entry context.")
		enc := zapcore.NewMapObjectEncoder()
		output[0].Context[0].AddTo(enc)
		assert.Equal(t, []interface{}{
			map[string]interface{}{"position": int64(0), "key": int64(42), "value": "foo"},
			map[string]interface{}{"position": int64(2), "key": []interface{}{"bar"}, "value": "baz"},
		}, enc.Fields["invalid"], "Unexpected output when logging invalid key-value pairs.")
	})
}

type stringerF func() string

func (f stringerF) String() string { return f() }

func TestSugarStructuredLogging(t *testing.T) {
	tests := []struct {
		msg       string
		expectMsg string
	}{
		{"foo", "foo"},
		{"", ""},
	}

	// Common to all test cases.
	context := []interface{}{"foo", "bar"}
	extra := []interface{}{"baz", false}
	expectedFields := []Field{String("foo", "bar"), Bool("baz", false)}

	for _, tt := range tests {
		withSugar(t, DebugLevel, nil, func(logger *SugaredLogger, logs *observer.ObservedLogs) {
			logger.With(context...).Debugw(tt.msg, extra...)
			logger.With(context...).Infow(tt.msg, extra...)
			logger.With(context...).Warnw(tt.msg, extra...)
			logger.With(context...).Errorw(tt.msg, extra...)
			logger.With(context...).DPanicw(tt.msg, extra...)

			expected := make([]observer.LoggedEntry, 5)
			for i, lvl := range []zapcore.Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, DPanicLevel} {
				expected[i] = observer.LoggedEntry{
					Entry:   zapcore.Entry{Message: tt.expectMsg, Level: lvl},
					Context: expectedFields,
				}
			}
			assert.Equal(t, expected, logs.AllUntimed(), "Unexpected log output.")
		})
	}
}

func TestSugarConcatenatingLogging(t *testing.T) {
	tests := []struct {
		args   []interface{}
		expect string
	}{
		{[]interface{}{nil}, "<nil>"},
	}

	// Common to all test cases.
	context := []interface{}{"foo", "bar"}
	expectedFields := []Field{String("foo", "bar")}

	for _, tt := range tests {
		withSugar(t, DebugLevel, nil, func(logger *SugaredLogger, logs *observer.ObservedLogs) {
			logger.With(context...).Debug(tt.args...)
			logger.With(context...).Info(tt.args...)
			logger.With(context...).Warn(tt.args...)
			logger.With(context...).Error(tt.args...)
			logger.With(context...).DPanic(tt.args...)

			expected := make([]observer.LoggedEntry, 5)
			for i, lvl := range []zapcore.Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, DPanicLevel} {
				expected[i] = observer.LoggedEntry{
					Entry:   zapcore.Entry{Message: tt.expect, Level: lvl},
					Context: expectedFields,
				}
			}
			assert.Equal(t, expected, logs.AllUntimed(), "Unexpected log output.")
		})
	}
}

func TestSugarTemplatedLogging(t *testing.T) {
	tests := []struct {
		format string
		args   []interface{}
		expect string
	}{
		{"", nil, ""},
		{"foo", nil, "foo"},
		// If the user fails to pass a template, degrade to fmt.Sprint.
		{"", []interface{}{"foo"}, "foo"},
	}

	// Common to all test cases.
	context := []interface{}{"foo", "bar"}
	expectedFields := []Field{String("foo", "bar")}

	for _, tt := range tests {
		withSugar(t, DebugLevel, nil, func(logger *SugaredLogger, logs *observer.ObservedLogs) {
			logger.With(context...).Debugf(tt.format, tt.args...)
			logger.With(context...).Infof(tt.format, tt.args...)
			logger.With(context...).Warnf(tt.format, tt.args...)
			logger.With(context...).Errorf(tt.format, tt.args...)
			logger.With(context...).DPanicf(tt.format, tt.args...)

			expected := make([]observer.LoggedEntry, 5)
			for i, lvl := range []zapcore.Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, DPanicLevel} {
				expected[i] = observer.LoggedEntry{
					Entry:   zapcore.Entry{Message: tt.expect, Level: lvl},
					Context: expectedFields,
				}
			}
			assert.Equal(t, expected, logs.AllUntimed(), "Unexpected log output.")
		})
	}
}

func TestSugarPanicLogging(t *testing.T) {
	tests := []struct {
		loggerLevel zapcore.Level
		f           func(*SugaredLogger)
		expectedMsg string
	}{
		{FatalLevel, func(s *SugaredLogger) { s.Panic("foo") }, ""},
		{PanicLevel, func(s *SugaredLogger) { s.Panic("foo") }, "foo"},
		{DebugLevel, func(s *SugaredLogger) { s.Panic("foo") }, "foo"},
		{FatalLevel, func(s *SugaredLogger) { s.Panicf("%s", "foo") }, ""},
		{PanicLevel, func(s *SugaredLogger) { s.Panicf("%s", "foo") }, "foo"},
		{DebugLevel, func(s *SugaredLogger) { s.Panicf("%s", "foo") }, "foo"},
		{FatalLevel, func(s *SugaredLogger) { s.Panicw("foo") }, ""},
		{PanicLevel, func(s *SugaredLogger) { s.Panicw("foo") }, "foo"},
		{DebugLevel, func(s *SugaredLogger) { s.Panicw("foo") }, "foo"},
	}

	for _, tt := range tests {
		withSugar(t, tt.loggerLevel, nil, func(sugar *SugaredLogger, logs *observer.ObservedLogs) {
			assert.Panics(t, func() { tt.f(sugar) }, "Expected panic-level logger calls to panic.")
			if tt.expectedMsg != "" {
				assert.Equal(t, []observer.LoggedEntry{{
					Context: []Field{},
					Entry:   zapcore.Entry{Message: tt.expectedMsg, Level: PanicLevel},
				}}, logs.AllUntimed(), "Unexpected log output.")
			} else {
				assert.Equal(t, 0, logs.Len(), "Didn't expect any log output.")
			}
		})
	}
}

func TestSugarFatalLogging(t *testing.T) {
	tests := []struct {
		loggerLevel zapcore.Level
		f           func(*SugaredLogger)
		expectedMsg string
	}{
		{FatalLevel + 1, func(s *SugaredLogger) { s.Fatal("foo") }, ""},
		{FatalLevel, func(s *SugaredLogger) { s.Fatal("foo") }, "foo"},
		{DebugLevel, func(s *SugaredLogger) { s.Fatal("foo") }, "foo"},
		{FatalLevel + 1, func(s *SugaredLogger) { s.Fatalf("%s", "foo") }, ""},
		{FatalLevel, func(s *SugaredLogger) { s.Fatalf("%s", "foo") }, "foo"},
		{DebugLevel, func(s *SugaredLogger) { s.Fatalf("%s", "foo") }, "foo"},
		{FatalLevel + 1, func(s *SugaredLogger) { s.Fatalw("foo") }, ""},
		{FatalLevel, func(s *SugaredLogger) { s.Fatalw("foo") }, "foo"},
		{DebugLevel, func(s *SugaredLogger) { s.Fatalw("foo") }, "foo"},
	}

	for _, tt := range tests {
		withSugar(t, tt.loggerLevel, nil, func(sugar *SugaredLogger, logs *observer.ObservedLogs) {
			stub := exit.WithStub(func() { tt.f(sugar) })
			assert.True(t, stub.Exited, "Expected all calls to fatal logger methods to exit process.")
			if tt.expectedMsg != "" {
				assert.Equal(t, []observer.LoggedEntry{{
					Context: []Field{},
					Entry:   zapcore.Entry{Message: tt.expectedMsg, Level: FatalLevel},
				}}, logs.AllUntimed(), "Unexpected log output.")
			} else {
				assert.Equal(t, 0, logs.Len(), "Didn't expect any log output.")
			}
		})
	}
}

func TestSugarAddCaller(t *testing.T) {
	tests := []struct {
		options []Option
		pat     string
	}{
		{opts(AddCaller()), `.+/sugar_test.go:[\d]+$`},
		{opts(AddCaller(), AddCallerSkip(1), AddCallerSkip(-1)), `.+/zap/sugar_test.go:[\d]+$`},
		{opts(AddCaller(), AddCallerSkip(1)), `.+/zap/common_test.go:[\d]+$`},
		{opts(AddCaller(), AddCallerSkip(1), AddCallerSkip(5)), `.+/src/runtime/.*:[\d]+$`},
	}
	for _, tt := range tests {
		withSugar(t, DebugLevel, tt.options, func(logger *SugaredLogger, logs *observer.ObservedLogs) {
			logger.Info("")
			output := logs.AllUntimed()
			assert.Equal(t, 1, len(output), "Unexpected number of logs written out.")
			assert.Regexp(
				t,
				tt.pat,
				output[0].Entry.Caller,
				"Expected to find package name and file name in output.",
			)
		})
	}
}

func TestSugarAddCallerFail(t *testing.T) {
	errBuf := &ztest.Buffer{}
	withSugar(t, DebugLevel, opts(AddCaller(), AddCallerSkip(1e3), ErrorOutput(errBuf)), func(log *SugaredLogger, logs *observer.ObservedLogs) {
		log.Info("Failure.")
		assert.Regexp(
			t,
			`Logger.check error: failed to get caller`,
			errBuf.String(),
			"Didn't find expected failure message.",
		)
		assert.Equal(
			t,
			logs.AllUntimed()[0].Entry.Message,
			"Failure.",
			"Expected original message to survive failures in runtime.Caller.")
	})
}
