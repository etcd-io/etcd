package ssautil

import (
	"sync"

	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
)

// PackageAnalysisCache stores expensive SSA-derived artifacts that can be
// shared by multiple analyzers running on the same package.
type PackageAnalysisCache struct {
	ssa *buildssa.SSA

	callGraphOnce sync.Once
	callGraph     *callgraph.Graph
}

// NewPackageAnalysisCache builds a cache object for a package-level SSA result.
func NewPackageAnalysisCache(ssaResult *buildssa.SSA) *PackageAnalysisCache {
	return &PackageAnalysisCache{ssa: ssaResult}
}

// CallGraph returns a lazily initialized CHA call graph for the package.
// It is safe for concurrent use by multiple analyzers.
func (c *PackageAnalysisCache) CallGraph() *callgraph.Graph {
	if c == nil {
		return nil
	}

	c.callGraphOnce.Do(func() {
		if c.ssa == nil || len(c.ssa.SrcFuncs) == 0 || c.ssa.SrcFuncs[0] == nil {
			return
		}
		c.callGraph = cha.CallGraph(c.ssa.SrcFuncs[0].Prog)
	})

	return c.callGraph
}
