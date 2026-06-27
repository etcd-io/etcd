package tparallel

import (
	"github.com/moricho/tparallel"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(tparallel.Analyzer).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
