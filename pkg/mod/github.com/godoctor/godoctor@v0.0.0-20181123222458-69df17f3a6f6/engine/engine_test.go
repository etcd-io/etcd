// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package engine_test

import (
	"testing"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/refactoring"
	"github.com/godoctor/godoctor/text"
)

type customRefactoring struct{}

func (*customRefactoring) Description() *refactoring.Description {
	return &refactoring.Description{
		Name:   "Test",
		Params: nil,
		Hidden: false,
	}
}

func (*customRefactoring) Run(config *refactoring.Config) *refactoring.Result {
	return &refactoring.Result{
		Log:   refactoring.NewLog(),
		Edits: map[string]*text.EditSet{},
	}
}

func TestEngine(t *testing.T) {
	engine.AddDefaultRefactorings()

	first := ""
	for _, shortName := range engine.AllRefactoringNames() {
		if first == "" {
			first = shortName
		}
		if engine.GetRefactoring(shortName) == nil {
			t.Fatalf("GetRefactoring return incorrect")
		}
	}

	err := engine.AddRefactoring(first, &customRefactoring{})
	if err == nil {
		t.Fatalf("Should have forbidden adding with existing name")
	}

	err = engine.AddRefactoring("zz_new", &customRefactoring{})
	if err != nil {
		t.Fatalf("The name zz_new should be unique and OK to add (?!)")
	}
}
