package testutil

import (
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

type Test struct {
	Dir     string
	Version string
}

type Analyzer struct {
	Analyzer *analysis.Analyzer
	Tests    []Test
}

func Run(t *testing.T, analyzers []Analyzer) {
	for _, a := range analyzers {
		a := a
		t.Run(a.Analyzer.Name, func(t *testing.T) {
			t.Parallel()
			for _, test := range a.Tests {
				if test.Version != "" {
					if err := a.Analyzer.Flags.Lookup("go").Value.Set(test.Version); err != nil {
						t.Fatal(err)
					}
				}
				analysistest.Run(t, analysistest.TestData(), a.Analyzer, test.Dir)
			}
		})
	}
}
