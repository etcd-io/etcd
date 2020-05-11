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
	"go.uber.org/zap/zaptest/observer"
)

func opts(opts ...Option) []Option {
	return opts
}

// Here specifically to introduce an easily-identifiable filename for testing
// stacktraces and caller skips.
func withLogger(t testing.TB, e zapcore.LevelEnabler, opts []Option, f func(*Logger, *observer.ObservedLogs)) {
	fac, logs := observer.New(e)
	log := New(fac, opts...)
	f(log, logs)
}

func withSugar(t testing.TB, e zapcore.LevelEnabler, opts []Option, f func(*SugaredLogger, *observer.ObservedLogs)) {
	withLogger(t, e, opts, func(logger *Logger, logs *observer.ObservedLogs) { f(logger.Sugar(), logs) })
}

func runConcurrently(goroutines, iterations int, wg *sync.WaitGroup, f func()) {
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				f()
			}
		}()
	}
}
