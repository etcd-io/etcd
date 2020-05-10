// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Log struct and associated methods.  Every refactoring
// returns a Log, which contains informational messages, warnings, and errors
// generated during the refactoring process.  If the log is nonempty, it should
// be displayed to the user before a refactoring's changes are applied.

// TERMINOLOGY: "Initial entries" are those that are added when the program is
// first loaded, before the refactoring begins.  They are used to record
// semantic errors that are present file before refactoring starts.  Some
// refactorings work in the presence of errors, and others may not.  Therefore,
// there are two methods to modify initial entries: one that converts initial
// errors to warnings, and another that removes initial entries altogether.

package refactoring

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"

	"go/ast"
	"go/token"

	"github.com/godoctor/godoctor/filesystem"
)

// A Severity indicates whether a log entry describes an informational message,
// a warning, or an error.
type Severity int

const (
	Info    Severity = iota // informational message
	Warning                 // warning, something to be cautious of
	Error                   // the refactoring transformation is, or might be, invalid
)

// A Entry constitutes a single entry in a Log.  Every Entry has a
// severity and a message.  If the filename is a nonempty string, the Entry
// is associated with a particular position in the given file.  Some log
// entries are marked as "initial."  These indicate semantic errors that were
// present in the input file (e.g., unresolved identifiers, unnecessary
// imports, etc.) before the refactoring was started.
type Entry struct {
	isInitial bool
	Severity  Severity
	Message   string
	Pos       token.Pos
	End       token.Pos
}

func (entry *Entry) String() string {
	var buffer bytes.Buffer
	switch entry.Severity {
	case Info:
		// No prefix
	case Warning:
		buffer.WriteString("Warning: ")
	case Error:
		buffer.WriteString("Error: ")
	}
	buffer.WriteString(entry.Message)
	return buffer.String()
}

// A Log is used to store informational messages, warnings, and errors that
// will be presented to the user before a refactoring's changes are applied.
type Log struct {
	// FileSet to map log entries' Pos and End fields to file positions
	Fset *token.FileSet
	// Informational messages, warnings, and errors, in the (temporal)
	// order they were added to the log
	Entries []*Entry
}

// NewLog creates an empty Log.  The Log will be unable to associate errors
// with filenames and line/column/offset information until its Fset field is
// set non-nil.
func NewLog() *Log {
	return &Log{
		Fset:    nil,
		Entries: []*Entry{}}
}

// Append adds the given entries to the end of this log, preserving their order.
func (log *Log) Append(entries []*Entry) {
	for _, entry := range entries {
		log.Entries = append(log.Entries, entry)
	}
}

// Clear removes all Entries from the error log.
func (log *Log) Clear() {
	log.Entries = []*Entry{}
}

// Infof adds an informational message (an entry with Info severity) to a log.
func (log *Log) Infof(format string, v ...interface{}) {
	log.log(Info, format, v...)
}

// Info adds an informational message (an entry with Info severity) to a log.
func (log *Log) Info(entry interface{}) {
	log.log(Info, "%v", entry)
}

// Warnf adds an entry with Warning severity to a log.
func (log *Log) Warnf(format string, v ...interface{}) {
	log.log(Warning, format, v...)
}

// Warn adds an entry with Warning severity to a log.
func (log *Log) Warn(entry interface{}) {
	log.log(Warning, "%v", entry)
}

// Errorf adds an entry with Error severity to a log.
func (log *Log) Errorf(format string, v ...interface{}) {
	log.log(Error, format, v...)
}

// Error adds an entry with Error severity to a log.
func (log *Log) Error(entry interface{}) {
	log.log(Error, "%v", entry)
}

func (log *Log) log(severity Severity, format string, v ...interface{}) {
	log.Entries = append(log.Entries, &Entry{
		isInitial: false,
		Severity:  severity,
		Message:   fmt.Sprintf(format, v...),
		Pos:       token.NoPos,
		End:       token.NoPos})
}

/*
// Associate associates the most recently-logged entry with the given filename.
func (log *Log) Associate(filename string) {
	if len(log.Entries) == 0 {
		return
	}
	entry := log.Entries[len(log.Entries)-1]
	entry.Filename = displayablePath(filename)
}
*/
// AssociatePos associates the most recently-logged entry with the file and
// offset denoted by the given Pos.
func (log *Log) AssociatePos(start, end token.Pos) {
	if len(log.Entries) == 0 {
		return
	}
	entry := log.Entries[len(log.Entries)-1]
	entry.Pos = start
	entry.End = end
}

// AssociateNode associates the most recently-logged entry with the region of
// source code corresponding to the given AST Node.
func (log *Log) AssociateNode(node ast.Node) {
	log.AssociatePos(node.Pos(), node.End())
}

// MarkInitial marks all entries that have been logged so far as initial
// entries.  Subsequent entries will not be marked as initial unless this
// method is called again at a later point in time.
func (log *Log) MarkInitial() {
	for _, entry := range log.Entries {
		entry.isInitial = true
	}
}

func (log *Log) String() string {
	var buffer bytes.Buffer
	log.Write(&buffer, "")
	return buffer.String()
}

// Write outputs this log in a GNU-style 'file:line:col: message' format.
// Filenames are displayed relative to the given directory, if possible.
func (log *Log) Write(out io.Writer, cwd string) {
	for _, entry := range log.Entries {
		if log.Fset != nil && entry.Pos.IsValid() {
			pos := log.Fset.Position(entry.Pos)
			fmt.Fprintf(out, "%s:%d:%d: ",
				displayablePath(pos.Filename, cwd),
				pos.Line,
				pos.Column)
		}
		fmt.Fprintf(out, "%s\n", entry.String())
	}
}

// displayablePath returns a path for the given file relative to the given
// current directory.  If a relative path cannot be determined, file is
// returned as-is.  This is intended for use in displaying error messages.
func displayablePath(file, cwd string) string {
	stdin, _ := filesystem.FakeStdinPath()
	if file == stdin {
		return "<stdin>"
	}

	if cwd == "" {
		return file
	}

	absPath, err := filepath.Abs(file)
	if err != nil {
		absPath = file
	}

	relativePath, err := filepath.Rel(cwd, absPath)
	if err != nil || relativePath == "" {
		return file
	}

	return relativePath
}

// ContainsPositions returns true if the log contains at least one entry that
// has position information associated with it.
func (log *Log) ContainsPositions() bool {
	return log.contains(func(entry *Entry) bool {
		return entry.Pos.IsValid()
	})
}

// ContainsInitialErrors returns true if the log contains at least one initial
// entry with Error severity.
func (log *Log) ContainsInitialErrors() bool {
	return log.contains(func(entry *Entry) bool {
		return entry.isInitial && entry.Severity >= Error
	})
}

// ContainsErrors returns true if the log contains at least one error.  The
// error may be an initial entry, or it may not.
func (log *Log) ContainsErrors() bool {
	return log.contains(func(entry *Entry) bool {
		return entry.Severity >= Error
	})
}

func (log *Log) contains(predicate func(*Entry) bool) bool {
	for _, entry := range log.Entries {
		if predicate(entry) {
			return true
		}
	}
	return false
}

// RemoveInitialEntries removes all initial entries from the log.  Entries that
// are not marked as initial are retained.
func (log *Log) RemoveInitialEntries() {
	newEntries := []*Entry{}
	for _, entry := range log.Entries {
		if !entry.isInitial {
			newEntries = append(newEntries, entry)
		}
	}
	log.Entries = newEntries
}

// ChangeInitialErrorsToWarnings changes the severity of any initial errors to
// Warning severity.
func (log *Log) ChangeInitialErrorsToWarnings() {
	newEntries := []*Entry{}
	for _, entry := range log.Entries {
		if entry.isInitial && entry.Severity == Error {
			entry.Severity = Warning
			newEntries = append(newEntries, entry)
		} else {
			newEntries = append(newEntries, entry)
		}
	}
	log.Entries = newEntries
}
