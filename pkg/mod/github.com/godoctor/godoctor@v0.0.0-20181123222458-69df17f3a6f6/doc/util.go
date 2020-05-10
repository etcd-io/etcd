// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package doc contains functions to generate the HTML User's Guide and the
// man page for the Go Doctor.
package doc

import (
	"flag"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/refactoring"
)

type refacDesc struct {
	Key         string
	Description *refactoring.Description
}

type docContent struct {
	AboutText    string
	Flags        []*flag.Flag
	Refactorings []refacDesc
}

func prepare(aboutText string, flags *flag.FlagSet) docContent {
	content := docContent{aboutText, []*flag.Flag{}, []refacDesc{}}

	flags.VisitAll(func(flag *flag.Flag) {
		content.Flags = append(content.Flags, flag)
	})

	for _, key := range engine.AllRefactoringNames() {
		r := engine.GetRefactoring(key)
		if !r.Description().Hidden {
			content.Refactorings = append(content.Refactorings,
				refacDesc{key, r.Description()})
		}
	}

	return content
}
