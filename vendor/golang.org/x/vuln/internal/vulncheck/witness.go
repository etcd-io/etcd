// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"container/list"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// CallStack is a call stack starting with a client
// function or method and ending with a call to a
// vulnerable symbol.
type CallStack []StackEntry

// StackEntry is an element of a call stack.
type StackEntry struct {
	// Function whose frame is on the stack.
	Function *FuncNode

	// Call is the call site inducing the next stack frame.
	// nil when the frame represents the last frame in the stack.
	Call *CallSite
}

// sourceCallstacks returns representative call stacks for each
// vulnerability in res. The returned call stacks are heuristically
// ordered by how seemingly easy is to understand them: shorter
// call stacks with less dynamic call sites appear earlier in the
// returned slices.
//
// sourceCallstacks performs a breadth-first search of res.CallGraph
// starting at the vulnerable symbol and going up until reaching an entry
// function or method in res.CallGraph.Entries. During this search,
// each function is visited at most once to avoid potential
// exponential explosion. Hence, not all call stacks are analyzed.
func sourceCallstacks(res *Result) map[*Vuln]CallStack {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	stackPerVuln := make(map[*Vuln]CallStack)
	for _, vuln := range res.Vulns {
		vuln := vuln
		wg.Add(1)
		go func() {
			cs := sourceCallstack(vuln, res)
			mu.Lock()
			stackPerVuln[vuln] = cs
			mu.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()

	updateInitPositions(stackPerVuln)
	return stackPerVuln
}

// sourceCallstack finds a representative call stack for vuln.
// This is a shortest unique call stack with the least
// number of dynamic call sites.
func sourceCallstack(vuln *Vuln, res *Result) CallStack {
	vulnSink := vuln.CallSink
	if vulnSink == nil {
		return nil
	}

	entries := make(map[*FuncNode]bool)
	for _, e := range res.EntryFunctions {
		entries[e] = true
	}

	seen := make(map[*FuncNode]bool)

	// Do a BFS from the vuln sink to the entry points
	// and find the representative call stack. This is
	// the shortest call stack that goes through the
	// least number of dynamic call sites. We first
	// collect all candidate call stacks of the shortest
	// length and then pick the best one accordingly.
	var candidates []CallStack
	candDepth := 0
	queue := list.New()
	queue.PushBack(&callChain{f: vulnSink})

	// We want to avoid call stacks that go through
	// other vulnerable symbols of the same package
	// for the same vulnerability. In other words,
	// we want unique call stacks.
	skipSymbols := make(map[*FuncNode]bool)
	for _, v := range res.Vulns {
		if v.CallSink != nil && v != vuln &&
			v.OSV == vuln.OSV && v.Package == vuln.Package {
			skipSymbols[v.CallSink] = true
		}
	}

	for queue.Len() > 0 {
		front := queue.Front()
		c := front.Value.(*callChain)
		queue.Remove(front)

		f := c.f
		if seen[f] {
			continue
		}
		seen[f] = true

		// Pick a single call site for each function in determinstic order.
		// A single call site is sufficient as we visit a function only once.
		for _, cs := range callsites(f.CallSites, seen) {
			nStack := &callChain{f: cs.Parent, call: cs, child: c}
			if !skipSymbols[cs.Parent] {
				queue.PushBack(nStack)
			}

			if entries[cs.Parent] {
				ns := nStack.CallStack()
				if len(candidates) == 0 || len(ns) == candDepth {
					// The case where we either have not identified
					// any call stacks or just found one of the same
					// length as the previous ones.
					candidates = append(candidates, ns)
					candDepth = len(ns)
				} else {
					// We just found a candidate call stack whose
					// length is greater than what we previously
					// found. We can thus safely disregard this
					// call stack and stop searching since we won't
					// be able to find any better candidates.
					queue.Init() // clear the list, effectively exiting the outer loop
				}
			}
		}
	}

	// Sort candidate call stacks by their number of dynamic call
	// sites and return the first one.
	sort.SliceStable(candidates, func(i int, j int) bool {
		s1, s2 := candidates[i], candidates[j]
		if w1, w2 := weight(s1), weight(s2); w1 != w2 {
			return w1 < w2
		}

		// At this point, the stableness/determinism of
		// sorting is guaranteed by the determinism of
		// the underlying call graph and the call stack
		// search algorithm.
		return true
	})
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

// callsites picks a call site from sites for each non-visited function.
// For each such function, the smallest (posLess) call site is chosen. The
// returned slice is sorted by caller functions (funcLess). Assumes callee
// of each call site is the same.
func callsites(sites []*CallSite, visited map[*FuncNode]bool) []*CallSite {
	minCs := make(map[*FuncNode]*CallSite)
	for _, cs := range sites {
		if visited[cs.Parent] {
			continue
		}
		if csLess(cs, minCs[cs.Parent]) {
			minCs[cs.Parent] = cs
		}
	}

	var fs []*FuncNode
	for _, cs := range minCs {
		fs = append(fs, cs.Parent)
	}
	sort.SliceStable(fs, func(i, j int) bool { return funcLess(fs[i], fs[j]) })

	var css []*CallSite
	for _, f := range fs {
		css = append(css, minCs[f])
	}
	return css
}

// callChain models a chain of function calls.
type callChain struct {
	call  *CallSite // nil for entry points
	f     *FuncNode
	child *callChain
}

// CallStack converts callChain to CallStack type.
func (c *callChain) CallStack() CallStack {
	if c == nil {
		return nil
	}
	return append(CallStack{StackEntry{Function: c.f, Call: c.call}}, c.child.CallStack()...)
}

// weight computes an approximate measure of how easy is to understand the call
// stack when presented to the client as a witness. The smaller the value, the more
// understandable the stack is. Currently defined as the number of unresolved
// call sites in the stack.
func weight(stack CallStack) int {
	w := 0
	for _, e := range stack {
		if e.Call != nil && !e.Call.Resolved {
			w += 1
		}
	}
	return w
}

// csLess compares two call sites by their locations and, if needed,
// their string representation.
func csLess(cs1, cs2 *CallSite) bool {
	if cs2 == nil {
		return true
	}

	// fast code path
	if p1, p2 := cs1.Pos, cs2.Pos; p1 != nil && p2 != nil {
		if posLess(*p1, *p2) {
			return true
		}
		if posLess(*p2, *p1) {
			return false
		}
		// for sanity, should not occur in practice
		return fmt.Sprintf("%v.%v", cs1.RecvType, cs2.Name) < fmt.Sprintf("%v.%v", cs2.RecvType, cs2.Name)
	}

	// code path rarely exercised
	if cs2.Pos == nil {
		return true
	}
	if cs1.Pos == nil {
		return false
	}
	// should very rarely occur in practice
	return fmt.Sprintf("%v.%v", cs1.RecvType, cs2.Name) < fmt.Sprintf("%v.%v", cs2.RecvType, cs2.Name)
}

// posLess compares two positions by their line and column number,
// and filename if needed.
func posLess(p1, p2 token.Position) bool {
	if p1.Line < p2.Line {
		return true
	}
	if p2.Line < p1.Line {
		return false
	}

	if p1.Column < p2.Column {
		return true
	}
	if p2.Column < p1.Column {
		return false
	}

	return strings.Compare(p1.Filename, p2.Filename) == -1
}

// funcLess compares two function nodes by locations of
// corresponding functions and, if needed, their string representation.
func funcLess(f1, f2 *FuncNode) bool {
	if p1, p2 := f1.Pos, f2.Pos; p1 != nil && p2 != nil {
		if posLess(*p1, *p2) {
			return true
		}
		if posLess(*p2, *p1) {
			return false
		}
		// for sanity, should not occur in practice
		return f1.String() < f2.String()
	}

	if f2.Pos == nil {
		return true
	}
	if f1.Pos == nil {
		return false
	}
	// should happen only for inits
	return f1.String() < f2.String()
}

// updateInitPositions populates non-existing positions of init functions
// and their respective calls in callStacks (see #51575).
func updateInitPositions(callStacks map[*Vuln]CallStack) {
	for _, cs := range callStacks {
		for i := range cs {
			updateInitPosition(&cs[i])
			if i != len(cs)-1 {
				updateInitCallPosition(&cs[i], cs[i+1])
			}
		}
	}
}

// updateInitCallPosition updates the position of a call to init in a stack frame, if
// one already does not exist:
//
//	P1.init -> P2.init: position of call to P2.init is the position of "import P2"
//	statement in P1
//
//	P.init -> P.init#d: P.init is an implicit init. We say it calls the explicit
//	P.init#d at the place of "package P" statement.
func updateInitCallPosition(curr *StackEntry, next StackEntry) {
	call := curr.Call
	if !isInit(next.Function) || (call.Pos != nil && call.Pos.IsValid()) {
		// Skip non-init functions and inits whose call site position is available.
		return
	}

	var pos token.Position
	if curr.Function.Name == "init" && curr.Function.Package == next.Function.Package {
		// We have implicit P.init calling P.init#d. Set the call position to
		// be at "package P" statement position.
		pos = packageStatementPos(curr.Function.Package)
	} else {
		// Choose the beginning of the import statement as the position.
		pos = importStatementPos(curr.Function.Package, next.Function.Package.PkgPath)
	}

	call.Pos = &pos
}

func importStatementPos(pkg *packages.Package, importPath string) token.Position {
	var importSpec *ast.ImportSpec
spec:
	for _, f := range pkg.Syntax {
		for _, impSpec := range f.Imports {
			// Import spec paths have quotation marks.
			impSpecPath, err := strconv.Unquote(impSpec.Path.Value)
			if err != nil {
				panic(fmt.Sprintf("import specification: package path has no quotation marks: %v", err))
			}
			if impSpecPath == importPath {
				importSpec = impSpec
				break spec
			}
		}
	}

	if importSpec == nil {
		// for sanity, in case of a wild call graph imprecision
		return token.Position{}
	}

	// Choose the beginning of the import statement as the position.
	return pkg.Fset.Position(importSpec.Pos())
}

func packageStatementPos(pkg *packages.Package) token.Position {
	if len(pkg.Syntax) == 0 {
		return token.Position{}
	}
	// Choose beginning of the package statement as the position. Pick
	// the first file since it is as good as any.
	return pkg.Fset.Position(pkg.Syntax[0].Package)
}

// updateInitPosition updates the position of P.init function in a stack frame if one
// is not available. The new position is the position of the "package P" statement.
func updateInitPosition(se *StackEntry) {
	fun := se.Function
	if !isInit(fun) || (fun.Pos != nil && fun.Pos.IsValid()) {
		// Skip non-init functions and inits whose position is available.
		return
	}

	pos := packageStatementPos(fun.Package)
	fun.Pos = &pos
}

func isInit(f *FuncNode) bool {
	// A source init function, or anonymous functions used in inits, will
	// be named "init#x" by vulncheck (more precisely, ssa), where x is a
	// positive integer. Implicit inits are named simply "init".
	return f.Name == "init" || strings.HasPrefix(f.Name, "init#")
}

// binaryCallstacks computes representative call stacks for binary results.
func binaryCallstacks(vr *Result) map[*Vuln]CallStack {
	callstacks := map[*Vuln]CallStack{}
	for _, vv := range uniqueVulns(vr.Vulns) {
		f := &FuncNode{Package: vv.Package, Name: vv.Symbol}
		parts := strings.Split(vv.Symbol, ".")
		if len(parts) != 1 {
			f.RecvType = parts[0]
			f.Name = parts[1]
		}
		callstacks[vv] = CallStack{StackEntry{Function: f}}
	}
	return callstacks
}

// uniqueVulns does for binary mode what sourceCallstacks does for source mode.
// It tries not to report redundant symbols. Since there are no call stacks in
// binary mode, the following approximate approach is used. Do not report unexported
// symbols for a <vulnID, pkg, module> triple if there are some exported symbols.
// Otherwise, report all unexported symbols to avoid not reporting anything.
func uniqueVulns(vulns []*Vuln) []*Vuln {
	type key struct {
		id  string
		pkg string
		mod string
	}
	hasExported := make(map[key]bool)
	for _, v := range vulns {
		if isExported(v.Symbol) {
			k := key{id: v.OSV.ID, pkg: v.Package.PkgPath, mod: v.Package.Module.Path}
			hasExported[k] = true
		}
	}

	var uniques []*Vuln
	for _, v := range vulns {
		k := key{id: v.OSV.ID, pkg: v.Package.PkgPath, mod: v.Package.Module.Path}
		if isExported(v.Symbol) || !hasExported[k] {
			uniques = append(uniques, v)
		}
	}
	return uniques
}

// isExported checks if the symbol is exported. Assumes that the
// symbol is of the form "identifier", "identifier1.identifier2",
// or "identifier.".
func isExported(symbol string) bool {
	parts := strings.Split(symbol, ".")
	last := parts[len(parts)-1]
	if last == "" { // case for "identifier."
		return false
	}
	return unicode.IsUpper(rune(last[0]))
}
