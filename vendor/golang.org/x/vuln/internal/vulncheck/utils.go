// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"bytes"
	"context"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
	"golang.org/x/vuln/internal"
	"golang.org/x/vuln/internal/osv"
	"golang.org/x/vuln/internal/semver"

	"golang.org/x/tools/go/ssa"
)

// buildSSA creates an ssa representation for pkgs. Returns
// the ssa program encapsulating the packages and top level
// ssa packages corresponding to pkgs.
func buildSSA(pkgs []*packages.Package, fset *token.FileSet) (*ssa.Program, []*ssa.Package) {
	prog := ssa.NewProgram(fset, ssa.InstantiateGenerics)

	imports := make(map[*packages.Package]*ssa.Package)
	var createImports func(map[string]*packages.Package)
	createImports = func(pkgs map[string]*packages.Package) {
		for _, p := range pkgs {
			if _, ok := imports[p]; !ok {
				i := prog.CreatePackage(p.Types, p.Syntax, p.TypesInfo, true)
				imports[p] = i
				createImports(p.Imports)
			}
		}
	}

	for _, tp := range pkgs {
		createImports(tp.Imports)
	}

	var ssaPkgs []*ssa.Package
	for _, tp := range pkgs {
		if sp, ok := imports[tp]; ok {
			ssaPkgs = append(ssaPkgs, sp)
		} else {
			sp := prog.CreatePackage(tp.Types, tp.Syntax, tp.TypesInfo, false)
			ssaPkgs = append(ssaPkgs, sp)
		}
	}
	prog.Build()
	return prog, ssaPkgs
}

// callGraph builds a call graph of prog based on VTA analysis.
func callGraph(ctx context.Context, prog *ssa.Program, entries []*ssa.Function) (*callgraph.Graph, error) {
	entrySlice := make(map[*ssa.Function]bool)
	for _, e := range entries {
		entrySlice[e] = true
	}

	if err := ctx.Err(); err != nil { // cancelled?
		return nil, err
	}
	initial := cha.CallGraph(prog)

	fslice := forwardSlice(entrySlice, initial)
	if err := ctx.Err(); err != nil { // cancelled?
		return nil, err
	}
	vtaCg := vta.CallGraph(fslice, initial)

	// Repeat the process once more, this time using
	// the produced VTA call graph as the base graph.
	fslice = forwardSlice(entrySlice, vtaCg)
	if err := ctx.Err(); err != nil { // cancelled?
		return nil, err
	}
	cg := vta.CallGraph(fslice, vtaCg)
	cg.DeleteSyntheticNodes()
	return cg, nil
}

// dbTypeFormat formats the name of t according how types
// are encoded in vulnerability database:
//   - pointer designation * is skipped
//   - full path prefix is skipped as well
func dbTypeFormat(t types.Type) string {
	switch tt := types.Unalias(t).(type) {
	case *types.Pointer:
		return dbTypeFormat(tt.Elem())
	case *types.Named:
		return tt.Obj().Name()
	default:
		return types.TypeString(t, func(p *types.Package) string { return "" })
	}
}

// dbFuncName computes a function name consistent with the namings used in vulnerability
// databases. Effectively, a qualified name of a function local to its enclosing package.
// If a receiver is a pointer, this information is not encoded in the resulting name. If
// a function has type argument/parameter, this information is omitted. The name of
// anonymous functions is simply "". The function names are unique subject to the enclosing
// package, but not globally.
//
// Examples:
//
//	func (a A) foo (...) {...}  -> A.foo
//	func foo(...) {...}         -> foo
//	func (b *B) bar (...) {...} -> B.bar
//	func (c C[T]) do(...) {...} -> C.do
func dbFuncName(f *ssa.Function) string {
	selectBound := func(f *ssa.Function) types.Type {
		// If f is a "bound" function introduced by ssa for a given type, return the type.
		// When "f" is a "bound" function, it will have 1 free variable of that type within
		// the function. This is subject to change when ssa changes.
		if len(f.FreeVars) == 1 && strings.HasPrefix(f.Synthetic, "bound ") {
			return f.FreeVars[0].Type()
		}
		return nil
	}
	selectThunk := func(f *ssa.Function) types.Type {
		// If f is a "thunk" function introduced by ssa for a given type, return the type.
		// When "f" is a "thunk" function, the first parameter will have that type within
		// the function. This is subject to change when ssa changes.
		params := f.Signature.Params() // params.Len() == 1 then params != nil.
		if strings.HasPrefix(f.Synthetic, "thunk ") && params.Len() >= 1 {
			if first := params.At(0); first != nil {
				return first.Type()
			}
		}
		return nil
	}
	var qprefix string
	if recv := f.Signature.Recv(); recv != nil {
		qprefix = dbTypeFormat(recv.Type())
	} else if btype := selectBound(f); btype != nil {
		qprefix = dbTypeFormat(btype)
	} else if ttype := selectThunk(f); ttype != nil {
		qprefix = dbTypeFormat(ttype)
	}

	if qprefix == "" {
		return funcName(f)
	}
	return qprefix + "." + funcName(f)
}

// funcName returns the name of the ssa function f.
// It is f.Name() without additional type argument
// information in case of generics.
func funcName(f *ssa.Function) string {
	n, _, _ := strings.Cut(f.Name(), "[")
	return n
}

// memberFuncs returns functions associated with the `member`:
// 1) `member` itself if `member` is a function
// 2) `member` methods if `member` is a type
// 3) empty list otherwise
func memberFuncs(member ssa.Member, prog *ssa.Program) []*ssa.Function {
	switch t := member.(type) {
	case *ssa.Type:
		methods := typeutil.IntuitiveMethodSet(t.Type(), &prog.MethodSets)
		var funcs []*ssa.Function
		for _, m := range methods {
			if f := prog.MethodValue(m); f != nil {
				funcs = append(funcs, f)
			}
		}
		return funcs
	case *ssa.Function:
		return []*ssa.Function{t}
	default:
		return nil
	}
}

// funcPosition gives the position of `f`. Returns empty token.Position
// if no file information on `f` is available.
func funcPosition(f *ssa.Function) *token.Position {
	pos := f.Prog.Fset.Position(f.Pos())
	return &pos
}

// instrPosition gives the position of `instr`. Returns empty token.Position
// if no file information on `instr` is available.
func instrPosition(instr ssa.Instruction) *token.Position {
	pos := instr.Parent().Prog.Fset.Position(instr.Pos())
	return &pos
}

func resolved(call ssa.CallInstruction) bool {
	if call == nil {
		return true
	}
	return call.Common().StaticCallee() != nil
}

func callRecvType(call ssa.CallInstruction) string {
	if !call.Common().IsInvoke() {
		return ""
	}
	buf := new(bytes.Buffer)
	types.WriteType(buf, call.Common().Value.Type(), nil)
	return buf.String()
}

func funcRecvType(f *ssa.Function) string {
	v := f.Signature.Recv()
	if v == nil {
		return ""
	}
	buf := new(bytes.Buffer)
	types.WriteType(buf, v.Type(), nil)
	return buf.String()
}

func FixedVersion(modulePath, version string, affected []osv.Affected) string {
	fixed := earliestValidFix(modulePath, version, affected)
	// Add "v" prefix if one does not exist. moduleVersionString
	// will later on replace it with "go" if needed.
	if fixed != "" && !strings.HasPrefix(fixed, "v") {
		fixed = "v" + fixed
	}
	return fixed
}

// earliestValidFix returns the earliest fix for version of modulePath that
// itself is not vulnerable in affected.
//
// Suppose we have a version "v1.0.0" and we use {...} to denote different
// affected regions. Assume for simplicity that all affected apply to the
// same input modulePath.
//
//	{[v0.1.0, v0.1.9), [v1.0.0, v2.0.0)} -> v2.0.0
//	{[v1.0.0, v1.5.0), [v2.0.0, v2.1.0}, {[v1.4.0, v1.6.0)} -> v2.1.0
func earliestValidFix(modulePath, version string, affected []osv.Affected) string {
	var moduleAffected []osv.Affected
	for _, a := range affected {
		if a.Module.Path == modulePath {
			moduleAffected = append(moduleAffected, a)
		}
	}

	vFixes := validFixes(version, moduleAffected)
	for _, fix := range vFixes {
		if !fixNegated(fix, moduleAffected) {
			return fix
		}
	}
	return ""

}

// validFixes computes all fixes for version in affected and
// returns them sorted increasingly. Assumes that all affected
// apply to the same module.
func validFixes(version string, affected []osv.Affected) []string {
	var fixes []string
	for _, a := range affected {
		for _, r := range a.Ranges {
			if r.Type != osv.RangeTypeSemver {
				continue
			}
			for _, e := range r.Events {
				fix := e.Fixed
				if fix != "" && semver.Less(version, fix) {
					fixes = append(fixes, fix)
				}
			}
		}
	}
	sort.SliceStable(fixes, func(i, j int) bool { return semver.Less(fixes[i], fixes[j]) })
	return fixes
}

// fixNegated checks if fix is negated to by a re-introduction
// of a vulnerability in affected. Assumes that all affected apply
// to the same module.
func fixNegated(fix string, affected []osv.Affected) bool {
	for _, a := range affected {
		for _, r := range a.Ranges {
			if semver.ContainsSemver(r, fix) {
				return true
			}
		}
	}
	return false
}

func modPath(mod *packages.Module) string {
	if mod.Replace != nil {
		return mod.Replace.Path
	}
	return mod.Path
}

func modVersion(mod *packages.Module) string {
	if mod.Replace != nil {
		return mod.Replace.Version
	}
	return mod.Version
}

// pkgPath returns the path of the f's enclosing package, if any.
// Otherwise, returns internal.UnknownPackagePath.
func pkgPath(f *ssa.Function) string {
	g := f
	if f.Origin() != nil {
		// Instantiations of generics do not have
		// an associated package. We hence look up
		// the original function for the package.
		g = f.Origin()
	}
	if g.Package() != nil && g.Package().Pkg != nil {
		return g.Package().Pkg.Path()
	}
	return internal.UnknownPackagePath
}

func pkgModPath(pkg *packages.Package) string {
	if pkg != nil && pkg.Module != nil {
		return pkg.Module.Path
	}
	return internal.UnknownModulePath
}

func IsStdPackage(pkg string) bool {
	if pkg == "" || pkg == internal.UnknownPackagePath {
		return false
	}
	// std packages do not have a "." in their path. For instance, see
	// Contains in pkgsite/+/refs/heads/master/internal/stdlbib/stdlib.go.
	if i := strings.IndexByte(pkg, '/'); i != -1 {
		pkg = pkg[:i]
	}
	return !strings.Contains(pkg, ".")
}
