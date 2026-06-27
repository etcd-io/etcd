package durationcheck

import (
	"github.com/charithe/durationcheck"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(durationcheck.Analyzer).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
