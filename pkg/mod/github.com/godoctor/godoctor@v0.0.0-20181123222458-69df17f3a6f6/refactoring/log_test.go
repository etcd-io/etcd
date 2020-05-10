// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"testing"

	"go/token"
)

func TestEntry(t *testing.T) {
	e := Entry{false, Info, "Message", token.NoPos, token.NoPos}
	assertEquals("Message", e.String(), t)
	e = Entry{false, Warning, "Message", token.NoPos, token.NoPos}
	assertEquals("Warning: Message", e.String(), t)
	e = Entry{false, Error, "Message", token.NoPos, token.NoPos}
	assertEquals("Error: Message", e.String(), t)
}

func TestAppend(t *testing.T) {
	l1 := NewLog()
	l1.Info("")
	l2 := NewLog()
	l2.Warn("")
	l2.Error("")
	l1.Append(l2.Entries)
	if len(l1.Entries) != 3 || len(l2.Entries) != 2 {
		t.Fatal("Wrong number of log entries")
	}
}

func TestLog(t *testing.T) {
	fset := token.NewFileSet()
	file1 := fset.AddFile("file1", fset.Base(), 10)
	file2 := fset.AddFile("file2", fset.Base(), 10)
	file2.AddLine(8)

	var log *Log = NewLog()
	log.Fset = fset
	log.Info("Info")
	log.Info("More info")
	log.AssociatePos(file1.Pos(0), file1.Pos(0))
	log.Warn("A warning")
	log.AssociatePos(file1.Pos(1), file1.Pos(3))
	log.Error("An error")
	log.AssociatePos(file2.Pos(3), file1.Pos(5))
	log.Error("Another error")
	log.AssociatePos(file2.Pos(8), file1.Pos(8))
	var expected string = `Info
file1:1:1: More info
file1:1:2: Warning: A warning
file2:1:4: Error: An error
file2:2:1: Error: Another error
`
	assertEquals(expected, log.String(), t)
}

// assertEquals is a utility method for unit tests that marks a function as
// having failed if expected != actual
// TODO(jeff): Copied from util_test.go
func assertEquals(expected string, actual string, t *testing.T) {
	if expected != actual {
		t.Fatalf("Expected: %s Actual: %s", expected, actual)
	}
}
