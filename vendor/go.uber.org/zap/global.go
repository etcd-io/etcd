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
	"bytes"
	"fmt"
	"log"
	"os"
	"sync"

	"go.uber.org/zap/zapcore"
)

const (
	_stdLogDefaultDepth = 2
	_loggerWriterDepth  = 2
)

var (
	_globalMu sync.RWMutex
	_globalL  = NewNop()
	_globalS  = _globalL.Sugar()
)

// L returns the global Logger, which can be reconfigured with ReplaceGlobals.
// It's safe for concurrent use.
func L() *Logger {
	_globalMu.RLock()
	l := _globalL
	_globalMu.RUnlock()
	return l
}

// S returns the global SugaredLogger, which can be reconfigured with
// ReplaceGlobals. It's safe for concurrent use.
func S() *SugaredLogger {
	_globalMu.RLock()
	s := _globalS
	_globalMu.RUnlock()
	return s
}

// ReplaceGlobals replaces the global Logger and SugaredLogger, and returns a
// function to restore the original values. It's safe for concurrent use.
func ReplaceGlobals(logger *Logger) func() {
	_globalMu.Lock()
	prev := _globalL
	_globalL = logger
	_globalS = logger.Sugar()
	_globalMu.Unlock()
	return func() { ReplaceGlobals(prev) }
}

// NewStdLog returns a *log.Logger which writes to the supplied zap Logger at
// InfoLevel. To redirect the standard library's package-global logging
// functions, use RedirectStdLog instead.
func NewStdLog(l *Logger) *log.Logger {
	logger := l.WithOptions(AddCallerSkip(_stdLogDefaultDepth + _loggerWriterDepth))
	f := logger.Info
	return log.New(&loggerWriter{f}, "" /* prefix */, 0 /* flags */)
}

// NewStdLogAt returns *log.Logger which writes to supplied zap logger at
// required level.
func NewStdLogAt(l *Logger, level zapcore.Level) (*log.Logger, error) {
	logger := l.WithOptions(AddCallerSkip(_stdLogDefaultDepth + _loggerWriterDepth))
	var logFunc func(string, ...zapcore.Field)
	switch level {
	case DebugLevel:
		logFunc = logger.Debug
	case InfoLevel:
		logFunc = logger.Info
	case WarnLevel:
		logFunc = logger.Warn
	case ErrorLevel:
		logFunc = logger.Error
	case DPanicLevel:
		logFunc = logger.DPanic
	case PanicLevel:
		logFunc = logger.Panic
	case FatalLevel:
		logFunc = logger.Fatal
	default:
		return nil, fmt.Errorf("unrecognized level: %q", level)
	}
	return log.New(&loggerWriter{logFunc}, "" /* prefix */, 0 /* flags */), nil
}

// RedirectStdLog redirects output from the standard library's package-global
// logger to the supplied logger at InfoLevel. Since zap already handles caller
// annotations, timestamps, etc., it automatically disables the standard
// library's annotations and prefixing.
//
// It returns a function to restore the original prefix and flags and reset the
// standard library's output to os.Stdout.
func RedirectStdLog(l *Logger) func() {
	flags := log.Flags()
	prefix := log.Prefix()
	log.SetFlags(0)
	log.SetPrefix("")
	logFunc := l.WithOptions(
		AddCallerSkip(_stdLogDefaultDepth + _loggerWriterDepth),
	).Info
	log.SetOutput(&loggerWriter{logFunc})
	return func() {
		log.SetFlags(flags)
		log.SetPrefix(prefix)
		log.SetOutput(os.Stderr)
	}
}

type loggerWriter struct {
	logFunc func(msg string, fields ...zapcore.Field)
}

func (l *loggerWriter) Write(p []byte) (int, error) {
	p = bytes.TrimSpace(p)
	l.logFunc(string(p))
	return len(p), nil
}
