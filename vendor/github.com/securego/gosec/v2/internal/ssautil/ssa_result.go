// Package ssautil provides shared SSA analysis utilities for gosec analyzers.
package ssautil

import (
	"errors"
	"log"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

var (
	ErrNoSSAResult    = errors.New("no SSA result found in the analysis pass")
	ErrInvalidSSAType = errors.New("the analysis pass result is not of type SSA")
)

// SSAAnalyzerResult contains various information returned by the
// SSA analysis along with some configuration
type SSAAnalyzerResult struct {
	Config map[string]any
	Logger *log.Logger
	SSA    *buildssa.SSA
	Shared *PackageAnalysisCache
}

// GetSSAResult retrieves the SSA result from analysis pass
func GetSSAResult(pass *analysis.Pass) (*SSAAnalyzerResult, error) {
	result, ok := pass.ResultOf[buildssa.Analyzer]
	if !ok {
		return nil, ErrNoSSAResult
	}
	ssaResult, ok := result.(*SSAAnalyzerResult)
	if !ok {
		return nil, ErrInvalidSSAType
	}
	return ssaResult, nil
}
