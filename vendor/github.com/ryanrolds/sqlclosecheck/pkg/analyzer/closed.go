package analyzer

import (
	"flag"

	"golang.org/x/tools/go/analysis"
)

type closedAnalyzer struct{}

func NewClosedAnalyzer() *analysis.Analyzer {
	analyzer := &closedAnalyzer{}
	flags := flag.NewFlagSet("closedAnalyzer", flag.ExitOnError)
	return newAnalyzer(analyzer.Run, flags)
}

// Run implements the main analysis pass
func (a *closedAnalyzer) Run(pass *analysis.Pass) (interface{}, error) {
	// pssa, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	// if !ok {
	// 	return nil, nil
	// }

	return nil, nil
}
