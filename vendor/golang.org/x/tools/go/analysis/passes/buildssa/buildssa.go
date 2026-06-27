// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buildssa defines an Analyzer that constructs the SSA
// representation of an error-free package and returns the set of all
// functions within it. It does not report any diagnostics itself but
// may be used as an input to other analyzers.
package buildssa

import (
	"go/ast"
	"go/types"
	"iter"
	"reflect"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:       "buildssa",
	Doc:        "build SSA-form IR for later passes",
	URL:        "https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/buildssa",
	Run:        run,
	Requires:   []*analysis.Analyzer{ctrlflow.Analyzer},
	ResultType: reflect.TypeFor[*SSA](),
	// Do not add FactTypes here: SSA construction of P must not
	// require SSA construction of all of P's dependencies.
	// (That's why we enlist the cheaper ctrlflow pass to compute
	// noreturn instead of having go/ssa + buildssa do it.)
	FactTypes: nil,
}

// SSA provides SSA-form intermediate representation for all the
// source functions in the current package.
type SSA struct {
	Pkg      *ssa.Package
	SrcFuncs []*ssa.Function
}

func run(pass *analysis.Pass) (any, error) {
	cfgs := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)

	// We must create a new Program for each Package because the
	// analysis API provides no place to hang a Program shared by
	// all Packages. Consequently, SSA Packages and Functions do not
	// have a canonical representation across an analysis session of
	// multiple packages. This is unlikely to be a problem in
	// practice because the analysis API essentially forces all
	// packages to be analysed independently, so any given call to
	// Analysis.Run on a package will see only SSA objects belonging
	// to a single Program.

	// Some Analyzers may need GlobalDebug, in which case we'll have
	// to set it globally, but let's wait till we need it.
	mode := ssa.BuilderMode(0)

	prog := ssa.NewProgram(pass.Fset, mode)

	// Use the result of the ctrlflow analysis to improve the SSA CFG.
	prog.SetNoReturn(cfgs.NoReturn)

	// Create SSA packages for direct imports.
	for _, p := range pass.Pkg.Imports() {
		prog.CreatePackage(p, nil, nil, true)
	}

	// Create and build the primary package.
	ssapkg := prog.CreatePackage(pass.Pkg, pass.Files, pass.TypesInfo, false)
	ssapkg.Build()

	// Compute list of source functions, including literals,
	// in source order.
	var funcs []*ssa.Function
	for _, fn := range allFunctions(pass) {
		// (init functions have distinct Func
		// objects named "init" and distinct
		// ssa.Functions named "init#1", ...)

		f := ssapkg.Prog.FuncValue(fn)
		if f == nil {
			panic(fn)
		}

		var addAnons func(f *ssa.Function)
		addAnons = func(f *ssa.Function) {
			funcs = append(funcs, f)
			for _, anon := range f.AnonFuncs {
				addAnons(anon)
			}
		}
		addAnons(f)
	}

	return &SSA{Pkg: ssapkg, SrcFuncs: funcs}, nil
}

// allFunctions returns an iterator over all named functions.
func allFunctions(pass *analysis.Pass) iter.Seq2[*ast.FuncDecl, *types.Func] {
	return func(yield func(*ast.FuncDecl, *types.Func) bool) {
		for _, file := range pass.Files {
			for _, decl := range file.Decls {
				if decl, ok := decl.(*ast.FuncDecl); ok {
					fn := pass.TypesInfo.Defs[decl.Name].(*types.Func)
					if !yield(decl, fn) {
						return
					}
				}
			}
		}
	}
}
