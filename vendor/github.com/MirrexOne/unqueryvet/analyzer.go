// Package unqueryvet provides a Go static analysis tool that detects SELECT * usage
package unqueryvet

import (
	"golang.org/x/tools/go/analysis"

	"github.com/MirrexOne/unqueryvet/internal/analyzer"
	"github.com/MirrexOne/unqueryvet/pkg/config"
)

// Analyzer is the main unqueryvet analyzer instance
// This is the primary export that golangci-lint will use
var Analyzer = analyzer.NewAnalyzer()

// New creates a new instance of the unqueryvet analyzer
func New() *analysis.Analyzer {
	return Analyzer
}

// NewWithConfig creates a new analyzer instance with custom configuration
// This is the recommended way to use unqueryvet with custom settings
func NewWithConfig(cfg *config.UnqueryvetSettings) *analysis.Analyzer {
	if cfg == nil {
		return Analyzer
	}
	return analyzer.NewAnalyzerWithSettings(*cfg)
}
