// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Writer is implemented by any recipient of log messages.
type Writer interface {
	// Write writes a message to the Writer with the given
	// level and module name. The filename and line hold
	// the file name and line number of the code that is
	// generating the log message; the time stamp holds
	// the time the log message was generated, and
	// message holds the log message itself.
	Write(level Level, name, filename string, line int, timestamp time.Time, message string)
}

type registeredWriter struct {
	writer Writer
	level  Level
}

// defaultName is the name of a writer that is registered
// by default that writes to stderr.
const defaultName = "default"

var (
	writerMutex sync.Mutex
	writers     = map[string]*registeredWriter{
		defaultName: &registeredWriter{
			writer: NewSimpleWriter(os.Stderr, &DefaultFormatter{}),
			level:  TRACE,
		},
	}
	globalMinLevel = TRACE
)

// ResetWriters puts the list of writers back into the initial state.
func ResetWriters() {
	writerMutex.Lock()
	defer writerMutex.Unlock()
	writers = map[string]*registeredWriter{
		"default": &registeredWriter{
			writer: NewSimpleWriter(os.Stderr, &DefaultFormatter{}),
			level:  TRACE,
		},
	}
	findMinLevel()
}

// ReplaceDefaultWriter is a convenience method that does the equivalent of
// RemoveWriter and then RegisterWriter with the name "default".  The previous
// default writer, if any is returned.
func ReplaceDefaultWriter(writer Writer) (Writer, error) {
	if writer == nil {
		return nil, fmt.Errorf("Writer cannot be nil")
	}
	writerMutex.Lock()
	defer writerMutex.Unlock()
	reg, found := writers[defaultName]
	if !found {
		return nil, fmt.Errorf("there is no %q writer", defaultName)
	}
	oldWriter := reg.writer
	reg.writer = writer
	return oldWriter, nil

}

// RegisterWriter adds the writer to the list of writers that get notified
// when logging.  When registering, the caller specifies the minimum logging
// level that will be written, and a name for the writer.  If there is already
// a registered writer with that name, an error is returned.
func RegisterWriter(name string, writer Writer, minLevel Level) error {
	if writer == nil {
		return fmt.Errorf("Writer cannot be nil")
	}
	writerMutex.Lock()
	defer writerMutex.Unlock()
	if _, found := writers[name]; found {
		return fmt.Errorf("there is already a Writer registered with the name %q", name)
	}
	writers[name] = &registeredWriter{writer: writer, level: minLevel}
	findMinLevel()
	return nil
}

// RemoveWriter removes the Writer identified by 'name' and returns it.
// If the Writer is not found, an error is returned.
func RemoveWriter(name string) (Writer, Level, error) {
	writerMutex.Lock()
	defer writerMutex.Unlock()
	registered, found := writers[name]
	if !found {
		return nil, UNSPECIFIED, fmt.Errorf("Writer %q is not registered", name)
	}
	delete(writers, name)
	findMinLevel()
	return registered.writer, registered.level, nil
}

func findMinLevel() {
	// We assume the lock is already held
	minLevel := CRITICAL
	for _, registered := range writers {
		if registered.level < minLevel {
			minLevel = registered.level
		}
	}
	globalMinLevel.set(minLevel)
}

// WillWrite returns whether there are any writers registered
// at or above the given severity level. If it returns
// false, a log message at the given level will be discarded.
func WillWrite(level Level) bool {
	return level >= globalMinLevel.get()
}

func writeToWriters(level Level, module, filename string, line int, timestamp time.Time, message string) {
	writerMutex.Lock()
	defer writerMutex.Unlock()
	for _, registered := range writers {
		if level >= registered.level {
			registered.writer.Write(level, module, filename, line, timestamp, message)
		}
	}
}

type simpleWriter struct {
	writer    io.Writer
	formatter Formatter
}

// NewSimpleWriter returns a new writer that writes
// log messages to the given io.Writer formatting the
// messages with the given formatter.
func NewSimpleWriter(writer io.Writer, formatter Formatter) Writer {
	return &simpleWriter{writer, formatter}
}

func (simple *simpleWriter) Write(level Level, module, filename string, line int, timestamp time.Time, message string) {
	logLine := simple.formatter.Format(level, module, filename, line, timestamp, message)
	fmt.Fprintln(simple.writer, logLine)
}
