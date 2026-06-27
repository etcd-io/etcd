// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"context"
	"sync"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/vuln/internal/client"
	"golang.org/x/vuln/internal/govulncheck"
	"golang.org/x/vuln/internal/osv"
)

// Source detects vulnerabilities in pkgs and emits the findings to handler.
func Source(ctx context.Context, handler govulncheck.Handler, cfg *govulncheck.Config, client *client.Client, graph *PackageGraph) error {
	vr, err := source(ctx, handler, cfg, client, graph)
	if err != nil {
		return err
	}

	if cfg.ScanLevel.WantSymbols() {
		return emitCallFindings(handler, sourceCallstacks(vr))
	}
	return nil
}

// source detects vulnerabilities in packages. It emits findings to handler
// and produces a Result that contains info on detected vulnerabilities.
//
// Assumes that pkgs are non-empty and belong to the same program.
func source(ctx context.Context, handler govulncheck.Handler, cfg *govulncheck.Config, client *client.Client, graph *PackageGraph) (*Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// If we are building the callgraph, build ssa and the callgraph in parallel
	// with fetching vulnerabilities. If the vulns set is empty, return without
	// waiting for SSA construction or callgraph to finish.
	var (
		wg       sync.WaitGroup // guards entries, cg, and buildErr
		entries  []*ssa.Function
		cg       *callgraph.Graph
		buildErr error
	)
	if cfg.ScanLevel.WantSymbols() {
		fset := graph.TopPkgs()[0].Fset
		wg.Add(1)
		go func() {
			defer wg.Done()
			prog, ssaPkgs := buildSSA(graph.TopPkgs(), fset)
			entries = entryPoints(ssaPkgs)
			cg, buildErr = callGraph(ctx, prog, entries)
		}()
	}

	if err := handler.SBOM(graph.SBOM()); err != nil {
		return nil, err
	}

	if err := handler.Progress(&govulncheck.Progress{Message: fetchingVulnsMessage}); err != nil {
		return nil, err
	}

	mv, err := FetchVulnerabilities(ctx, client, graph.Modules())
	if err != nil {
		return nil, err
	}

	// Emit OSV entries immediately in their raw unfiltered form.
	if err := emitOSVs(handler, mv); err != nil {
		return nil, err
	}

	if err := handler.Progress(&govulncheck.Progress{Message: checkingSrcVulnsMessage}); err != nil {
		return nil, err
	}

	affVulns := affectingVulnerabilities(mv, "", "")
	if err := emitModuleFindings(handler, affVulns); err != nil {
		return nil, err
	}

	if !cfg.ScanLevel.WantPackages() || len(affVulns) == 0 {
		return &Result{}, nil
	}

	impVulns := importedVulnPackages(affVulns, graph)
	// Emit information on imported vulnerable packages now as
	// call graph computation might take a while.
	if err := emitPackageFindings(handler, impVulns); err != nil {
		return nil, err
	}

	// Return result immediately if not in symbol mode or
	// if there are no vulnerabilities imported.
	if !cfg.ScanLevel.WantSymbols() || len(impVulns) == 0 {
		return &Result{Vulns: impVulns}, nil
	}

	wg.Wait() // wait for build to finish
	if buildErr != nil {
		return nil, err
	}

	entryFuncs, callVulns := calledVulnSymbols(entries, affVulns, cg, graph)
	return &Result{EntryFunctions: entryFuncs, Vulns: callVulns}, nil
}

// importedVulnPackages detects imported vulnerable packages.
func importedVulnPackages(affVulns affectingVulns, graph *PackageGraph) []*Vuln {
	var vulns []*Vuln
	analyzed := make(map[*packages.Package]bool) // skip analyzing the same package multiple times
	var vulnImports func(pkg *packages.Package)
	vulnImports = func(pkg *packages.Package) {
		if analyzed[pkg] {
			return
		}

		osvs := affVulns.ForPackage(pkgModPath(pkg), pkg.PkgPath)
		// Create Vuln entry for each OSV entry for pkg.
		for _, osv := range osvs {
			vuln := &Vuln{
				OSV:     osv,
				Package: graph.GetPackage(pkg.PkgPath),
			}
			vulns = append(vulns, vuln)
		}

		analyzed[pkg] = true
		for _, imp := range pkg.Imports {
			vulnImports(imp)
		}
	}

	for _, pkg := range graph.TopPkgs() {
		vulnImports(pkg)
	}
	return vulns
}

// calledVulnSymbols detects vuln symbols transitively reachable from sources
// via call graph cg.
//
// A slice of call graph is computed related to the reachable vulnerabilities. Each
// reachable Vuln has attached FuncNode that can be upward traversed to the entry points.
// Entry points that reach the vulnerable symbols are also returned.
func calledVulnSymbols(sources []*ssa.Function, affVulns affectingVulns, cg *callgraph.Graph, graph *PackageGraph) ([]*FuncNode, []*Vuln) {
	sinksWithVulns := vulnFuncs(cg, affVulns, graph)

	// Compute call graph backwards reachable
	// from vulnerable functions and methods.
	var sinks []*callgraph.Node
	for n := range sinksWithVulns {
		sinks = append(sinks, n)
	}
	bcg := callGraphSlice(sinks, false)

	// Interesect backwards call graph with forward
	// reachable graph to remove redundant edges.
	var filteredSources []*callgraph.Node
	for _, e := range sources {
		if n, ok := bcg.Nodes[e]; ok {
			filteredSources = append(filteredSources, n)
		}
	}
	fcg := callGraphSlice(filteredSources, true)

	// Get the sinks that are in fact reachable from entry points.
	filteredSinks := make(map[*callgraph.Node][]*osv.Entry)
	for n, vs := range sinksWithVulns {
		if fn, ok := fcg.Nodes[n.Func]; ok {
			filteredSinks[fn] = vs
		}
	}

	// Transform the resulting call graph slice into
	// vulncheck representation.
	return vulnCallGraph(filteredSources, filteredSinks, graph)
}

// callGraphSlice computes a slice of callgraph beginning at starts
// in the direction (forward/backward) controlled by forward flag.
func callGraphSlice(starts []*callgraph.Node, forward bool) *callgraph.Graph {
	g := &callgraph.Graph{Nodes: make(map[*ssa.Function]*callgraph.Node)}

	visited := make(map[*callgraph.Node]bool)
	var visit func(*callgraph.Node)
	visit = func(n *callgraph.Node) {
		if visited[n] {
			return
		}
		visited[n] = true

		var edges []*callgraph.Edge
		if forward {
			edges = n.Out
		} else {
			edges = n.In
		}

		for _, edge := range edges {
			nCallee := g.CreateNode(edge.Callee.Func)
			nCaller := g.CreateNode(edge.Caller.Func)
			callgraph.AddEdge(nCaller, edge.Site, nCallee)

			if forward {
				visit(edge.Callee)
			} else {
				visit(edge.Caller)
			}
		}
	}

	for _, s := range starts {
		visit(s)
	}
	return g
}

// vulnCallGraph creates vulnerability call graph in terms of sources and sinks.
func vulnCallGraph(sources []*callgraph.Node, sinks map[*callgraph.Node][]*osv.Entry, graph *PackageGraph) ([]*FuncNode, []*Vuln) {
	var entries []*FuncNode
	var vulns []*Vuln
	nodes := make(map[*ssa.Function]*FuncNode)

	// First create entries and sinks and store relevant information.
	for _, s := range sources {
		fn := createNode(nodes, s.Func, graph)
		entries = append(entries, fn)
	}

	for s, osvs := range sinks {
		f := s.Func
		funNode := createNode(nodes, s.Func, graph)

		// Populate CallSink field for each detected vuln symbol.
		for _, osv := range osvs {
			vulns = append(vulns, calledVuln(funNode, osv, dbFuncName(f), funNode.Package))
		}
	}

	visited := make(map[*callgraph.Node]bool)
	var visit func(*callgraph.Node)
	visit = func(n *callgraph.Node) {
		if visited[n] {
			return
		}
		visited[n] = true

		for _, edge := range n.In {
			nCallee := createNode(nodes, edge.Callee.Func, graph)
			nCaller := createNode(nodes, edge.Caller.Func, graph)

			call := edge.Site
			cs := &CallSite{
				Parent:   nCaller,
				Name:     call.Common().Value.Name(),
				RecvType: callRecvType(call),
				Resolved: resolved(call),
				Pos:      instrPosition(call),
			}
			nCallee.CallSites = append(nCallee.CallSites, cs)

			visit(edge.Caller)
		}
	}

	for s := range sinks {
		visit(s)
	}
	return entries, vulns
}

// vulnFuncs returns vulnerability information for vulnerable functions in cg.
func vulnFuncs(cg *callgraph.Graph, affVulns affectingVulns, graph *PackageGraph) map[*callgraph.Node][]*osv.Entry {
	m := make(map[*callgraph.Node][]*osv.Entry)
	for f, n := range cg.Nodes {
		p := pkgPath(f)
		vulns := affVulns.ForSymbol(pkgModPath(graph.GetPackage(p)), p, dbFuncName(f))
		if len(vulns) > 0 {
			m[n] = vulns
		}
	}
	return m
}

func createNode(nodes map[*ssa.Function]*FuncNode, f *ssa.Function, graph *PackageGraph) *FuncNode {
	if fn, ok := nodes[f]; ok {
		return fn
	}
	fn := &FuncNode{
		Name:     f.Name(),
		Package:  graph.GetPackage(pkgPath(f)),
		RecvType: funcRecvType(f),
		Pos:      funcPosition(f),
	}
	nodes[f] = fn
	return fn
}

func calledVuln(call *FuncNode, osv *osv.Entry, symbol string, pkg *packages.Package) *Vuln {
	return &Vuln{
		Symbol:   symbol,
		Package:  pkg,
		OSV:      osv,
		CallSink: call,
	}
}
