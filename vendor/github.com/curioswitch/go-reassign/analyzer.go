package reassign

import (
	"github.com/curioswitch/go-reassign/internal/analyzer"
	"golang.org/x/tools/go/analysis"
)

const FlagPattern = analyzer.FlagPattern

// NewAnalyzer returns an analyzer for checking that package variables are not reassigned.
func NewAnalyzer() *analysis.Analyzer {
	return analyzer.New()
}
