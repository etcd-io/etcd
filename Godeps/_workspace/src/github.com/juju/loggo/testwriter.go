// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo

import (
	"path"
	"sync"
	"time"
)

// TestLogValues represents a single logging call.
type TestLogValues struct {
	Level     Level
	Module    string
	Filename  string
	Line      int
	Timestamp time.Time
	Message   string
}

// TestWriter is a useful Writer for testing purposes.  Each component of the
// logging message is stored in the Log array.
type TestWriter struct {
	mu  sync.Mutex
	log []TestLogValues
}

// Write saves the params as members in the TestLogValues struct appended to the Log array.
func (writer *TestWriter) Write(level Level, module, filename string, line int, timestamp time.Time, message string) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.log = append(writer.log,
		TestLogValues{level, module, path.Base(filename), line, timestamp, message})
}

// Clear removes any saved log messages.
func (writer *TestWriter) Clear() {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.log = nil
}

// Log returns a copy of the current logged values.
func (writer *TestWriter) Log() []TestLogValues {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	v := make([]TestLogValues, len(writer.log))
	copy(v, writer.log)
	return v
}
