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
	"log"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/internal/exit"
	"go.uber.org/zap/internal/ztest"

	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestReplaceGlobals(t *testing.T) {
	initialL := *L()
	initialS := *S()

	withLogger(t, DebugLevel, nil, func(l *Logger, logs *observer.ObservedLogs) {
		L().Info("no-op")
		S().Info("no-op")
		assert.Equal(t, 0, logs.Len(), "Expected initial logs to go to default no-op global.")

		defer ReplaceGlobals(l)()

		L().Info("captured")
		S().Info("captured")
		expected := observer.LoggedEntry{
			Entry:   zapcore.Entry{Message: "captured"},
			Context: []Field{},
		}
		assert.Equal(
			t,
			[]observer.LoggedEntry{expected, expected},
			logs.AllUntimed(),
			"Unexpected global log output.",
		)
	})

	assert.Equal(t, initialL, *L(), "Expected func returned from ReplaceGlobals to restore initial L.")
	assert.Equal(t, initialS, *S(), "Expected func returned from ReplaceGlobals to restore initial S.")
}

func TestGlobalsConcurrentUse(t *testing.T) {
	var (
		stop atomic.Bool
		wg   sync.WaitGroup
	)

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			for !stop.Load() {
				ReplaceGlobals(NewNop())
			}
			wg.Done()
		}()
		go func() {
			for !stop.Load() {
				L().With(Int("foo", 42)).Named("main").WithOptions(Development()).Info("")
				S().Info("")
			}
			wg.Done()
		}()
	}

	ztest.Sleep(100 * time.Millisecond)
	stop.Toggle()
	wg.Wait()
}

func TestNewStdLog(t *testing.T) {
	withLogger(t, DebugLevel, []Option{AddCaller()}, func(l *Logger, logs *observer.ObservedLogs) {
		std := NewStdLog(l)
		std.Print("redirected")
		checkStdLogMessage(t, "redirected", logs)
	})
}

func TestNewStdLogAt(t *testing.T) {
	// include DPanicLevel here, but do not include Development in options
	levels := []zapcore.Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, DPanicLevel}
	for _, level := range levels {
		withLogger(t, DebugLevel, []Option{AddCaller()}, func(l *Logger, logs *observer.ObservedLogs) {
			std, err := NewStdLogAt(l, level)
			require.NoError(t, err, "Unexpected error.")
			std.Print("redirected")
			checkStdLogMessage(t, "redirected", logs)
		})
	}
}

func TestNewStdLogAtPanics(t *testing.T) {
	// include DPanicLevel here and enable Development in options
	levels := []zapcore.Level{DPanicLevel, PanicLevel}
	for _, level := range levels {
		withLogger(t, DebugLevel, []Option{AddCaller(), Development()}, func(l *Logger, logs *observer.ObservedLogs) {
			std, err := NewStdLogAt(l, level)
			require.NoError(t, err, "Unexpected error")
			assert.Panics(t, func() { std.Print("redirected") }, "Expected log to panic.")
			checkStdLogMessage(t, "redirected", logs)
		})
	}
}

func TestNewStdLogAtFatal(t *testing.T) {
	withLogger(t, DebugLevel, []Option{AddCaller()}, func(l *Logger, logs *observer.ObservedLogs) {
		stub := exit.WithStub(func() {
			std, err := NewStdLogAt(l, FatalLevel)
			require.NoError(t, err, "Unexpected error.")
			std.Print("redirected")
			checkStdLogMessage(t, "redirected", logs)
		})
		assert.True(t, true, stub.Exited, "Expected Fatal logger call to terminate process.")
		stub.Unstub()
	})
}

func TestNewStdLogAtInvalid(t *testing.T) {
	_, err := NewStdLogAt(NewNop(), zapcore.Level(99))
	assert.Error(t, err, "Expected to get error.")
	assert.Contains(t, err.Error(), "99", "Expected level code in error message")
}

func TestRedirectStdLog(t *testing.T) {
	initialFlags := log.Flags()
	initialPrefix := log.Prefix()

	withLogger(t, DebugLevel, nil, func(l *Logger, logs *observer.ObservedLogs) {
		defer RedirectStdLog(l)()
		log.Print("redirected")

		assert.Equal(t, []observer.LoggedEntry{{
			Entry:   zapcore.Entry{Message: "redirected"},
			Context: []Field{},
		}}, logs.AllUntimed(), "Unexpected global log output.")
	})

	assert.Equal(t, initialFlags, log.Flags(), "Expected to reset initial flags.")
	assert.Equal(t, initialPrefix, log.Prefix(), "Expected to reset initial prefix.")
}

func TestRedirectStdLogCaller(t *testing.T) {
	withLogger(t, DebugLevel, []Option{AddCaller()}, func(l *Logger, logs *observer.ObservedLogs) {
		defer RedirectStdLog(l)()
		log.Print("redirected")
		entries := logs.All()
		require.Len(t, entries, 1, "Unexpected number of logs.")
		assert.Contains(t, entries[0].Entry.Caller.File, "global_test.go", "Unexpected caller annotation.")
	})
}

func TestRedirectStdLogAt(t *testing.T) {
	initialFlags := log.Flags()
	initialPrefix := log.Prefix()

	// include DPanicLevel here, but do not include Development in options
	levels := []zapcore.Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, DPanicLevel}
	for _, level := range levels {
		withLogger(t, DebugLevel, nil, func(l *Logger, logs *observer.ObservedLogs) {
			restore, err := RedirectStdLogAt(l, level)
			require.NoError(t, err, "Unexpected error.")
			defer restore()
			log.Print("redirected")

			assert.Equal(t, []observer.LoggedEntry{{
				Entry:   zapcore.Entry{Level: level, Message: "redirected"},
				Context: []Field{},
			}}, logs.AllUntimed(), "Unexpected global log output.")
		})
	}

	assert.Equal(t, initialFlags, log.Flags(), "Expected to reset initial flags.")
	assert.Equal(t, initialPrefix, log.Prefix(), "Expected to reset initial prefix.")
}

func TestRedirectStdLogAtCaller(t *testing.T) {
	// include DPanicLevel here, but do not include Development in options
	levels := []zapcore.Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel, DPanicLevel}
	for _, level := range levels {
		withLogger(t, DebugLevel, []Option{AddCaller()}, func(l *Logger, logs *observer.ObservedLogs) {
			restore, err := RedirectStdLogAt(l, level)
			require.NoError(t, err, "Unexpected error.")
			defer restore()
			log.Print("redirected")
			entries := logs.All()
			require.Len(t, entries, 1, "Unexpected number of logs.")
			assert.Contains(t, entries[0].Entry.Caller.File, "global_test.go", "Unexpected caller annotation.")
		})
	}
}

func TestRedirectStdLogAtPanics(t *testing.T) {
	initialFlags := log.Flags()
	initialPrefix := log.Prefix()

	// include DPanicLevel here and enable Development in options
	levels := []zapcore.Level{DPanicLevel, PanicLevel}
	for _, level := range levels {
		withLogger(t, DebugLevel, []Option{AddCaller(), Development()}, func(l *Logger, logs *observer.ObservedLogs) {
			restore, err := RedirectStdLogAt(l, level)
			require.NoError(t, err, "Unexpected error.")
			defer restore()
			assert.Panics(t, func() { log.Print("redirected") }, "Expected log to panic.")
			checkStdLogMessage(t, "redirected", logs)
		})
	}

	assert.Equal(t, initialFlags, log.Flags(), "Expected to reset initial flags.")
	assert.Equal(t, initialPrefix, log.Prefix(), "Expected to reset initial prefix.")
}

func TestRedirectStdLogAtFatal(t *testing.T) {
	initialFlags := log.Flags()
	initialPrefix := log.Prefix()

	withLogger(t, DebugLevel, []Option{AddCaller()}, func(l *Logger, logs *observer.ObservedLogs) {
		stub := exit.WithStub(func() {
			restore, err := RedirectStdLogAt(l, FatalLevel)
			require.NoError(t, err, "Unexpected error.")
			defer restore()
			log.Print("redirected")
			checkStdLogMessage(t, "redirected", logs)
		})
		assert.True(t, true, stub.Exited, "Expected Fatal logger call to terminate process.")
		stub.Unstub()
	})

	assert.Equal(t, initialFlags, log.Flags(), "Expected to reset initial flags.")
	assert.Equal(t, initialPrefix, log.Prefix(), "Expected to reset initial prefix.")
}

func TestRedirectStdLogAtInvalid(t *testing.T) {
	restore, err := RedirectStdLogAt(NewNop(), zapcore.Level(99))
	defer func() {
		if restore != nil {
			restore()
		}
	}()
	require.Error(t, err, "Expected to get error.")
	assert.Contains(t, err.Error(), "99", "Expected level code in error message")
}

func checkStdLogMessage(t *testing.T, msg string, logs *observer.ObservedLogs) {
	require.Equal(t, 1, logs.Len(), "Expected exactly one entry to be logged")
	entry := logs.AllUntimed()[0]
	assert.Equal(t, []Field{}, entry.Context, "Unexpected entry context.")
	assert.Equal(t, "redirected", entry.Entry.Message, "Unexpected entry message.")
	assert.Regexp(
		t,
		`go.uber.org/zap/global_test.go:\d+$`,
		entry.Entry.Caller.String(),
		"Unexpected caller annotation.",
	)
}
