// Copyright 2015-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package engine is the programmatic entrypoint to the Go refactoring engine.
package engine

import (
	"fmt"

	"github.com/godoctor/godoctor/refactoring"
)

// All available refactorings, keyed by a unique, one-word, all-lowercase name
var refactorings map[string]refactoring.Refactoring

// All available refactorings' keys, in the order the refactorings should be
// displayed in a menu presented to the end user
var refactoringsInOrder []string

func init() {
	ClearRefactorings()
}

// AddDefaultRefactorings invokes AddRefactoring on each of the Go Doctor's
// built-in refactorings.
//
// Clients implementing a custom Go Doctor may:
// 1. not invoke this at all,
// 2. invoke it after adding custom refactorings, or
// 3. invoke it before adding custom refactorings.
// The choice between #2 and #3 will determine whether the client's custom
// refactorings are listed before or after the built-in refactorings when
// "godoctor -list" is run.
func AddDefaultRefactorings() {
	AddRefactoring("rename", new(refactoring.Rename))
	AddRefactoring("extract", new(refactoring.ExtractFunc))
	AddRefactoring("var", new(refactoring.ExtractLocal))
	AddRefactoring("toggle", new(refactoring.ToggleVar))
	AddRefactoring("godoc", new(refactoring.AddGoDoc))
	AddRefactoring("debug", new(refactoring.Debug))
	AddRefactoring("null", new(refactoring.Null))
}

// AllRefactoringNames returns the short names of all refactorings in an
// order suitable for display in a menu.
func AllRefactoringNames() []string {
	return refactoringsInOrder
}

// GetRefactoring returns a Refactoring keyed by the given short name.  The
// short name must be one of the keys in the map returned by AllRefactorings.
func GetRefactoring(shortName string) refactoring.Refactoring {
	return refactorings[shortName]
}

// AddRefactoring allows custom refactorings to be added to the refactoring
// engine.  Invoke this method before starting the command line or protocol
// driver.
func AddRefactoring(shortName string, newRefac refactoring.Refactoring) error {
	if r, ok := refactorings[shortName]; ok {
		return fmt.Errorf("The short name \"%s\" is already "+
			"associated with a refactoring (%s)",
			shortName,
			r.Description().Name)
	}
	refactorings[shortName] = newRefac
	refactoringsInOrder = append(refactoringsInOrder, shortName)
	return nil
}

// ClearRefactorings removes all registered refactorings from the engine.
// This should only be used for testing.
func ClearRefactorings() {
	refactorings = map[string]refactoring.Refactoring{}
	refactoringsInOrder = []string{}
}
