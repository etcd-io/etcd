// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Null refactoring, which makes no changes to a program.
// It is for testing only (and can be used as a template for building new
// refactorings).

package refactoring

// A Null refactoring makes no changes to a program.
type Null struct {
	RefactoringBase
}

func (r *Null) Description() *Description {
	return &Description{
		Name:      "Null Refactoring",
		Synopsis:  "Refactoring that makes no changes to a program",
		Usage:     "<allow_errors?>",
		HTMLDoc:   "",
		Multifile: false,
		Params: []Parameter{{
			Label:        "Allow Errors",
			Prompt:       "Allow Errors",
			DefaultValue: true,
		}},
		OptionalParams: nil,
		Hidden:         true,
	}
}

func (r *Null) Run(config *Config) *Result {
	r.Init(config, r.Description())

	if config.Args[0].(bool) {
		r.Log.ChangeInitialErrorsToWarnings()
	}

	if r.Log.ContainsErrors() {
		return &r.Result
	}

	r.UpdateLog(config, false)
	return &r.Result
}
