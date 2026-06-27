// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"strings"

	"golang.org/x/tools/go/ssa"
)

// entryPoints returns functions of topPackages considered entry
// points of govulncheck analysis: main, inits, and exported methods
// and functions.
//
// TODO(https://go.dev/issue/57221): currently, entry functions
// that are generics are not considered an entry point.
func entryPoints(topPackages []*ssa.Package) []*ssa.Function {
	var entries []*ssa.Function
	for _, pkg := range topPackages {
		if pkg.Pkg.Name() == "main" {
			// for "main" packages the only valid entry points are the "main"
			// function and any "init#" functions, even if there are other
			// exported functions or types. similarly to isEntry it should be
			// safe to ignore the validity of the main or init# signatures,
			// since the compiler will reject malformed definitions,
			// and the init function is synthetic
			entries = append(entries, memberFuncs(pkg.Members["main"], pkg.Prog)...)
			for name, member := range pkg.Members {
				if strings.HasPrefix(name, "init#") || name == "init" {
					entries = append(entries, memberFuncs(member, pkg.Prog)...)
				}
			}
			continue
		}
		for _, member := range pkg.Members {
			for _, f := range memberFuncs(member, pkg.Prog) {
				if isEntry(f) {
					entries = append(entries, f)
				}
			}
		}
	}
	return entries
}

func isEntry(f *ssa.Function) bool {
	// it should be safe to ignore checking that the signature of the "init" function
	// is valid, since it is synthetic
	if f.Name() == "init" && f.Synthetic == "package initializer" {
		return true
	}

	return f.Synthetic == "" && f.Object() != nil && f.Object().Exported()
}
