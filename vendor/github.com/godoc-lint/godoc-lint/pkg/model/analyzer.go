package model

import (
	"golang.org/x/tools/go/analysis"
)

// Analyzer defines an analyzer.
type Analyzer interface {
	// GetAnalyzer returns the underlying analyzer.
	GetAnalyzer() *analysis.Analyzer
}
