package gochecknoglobals

import (
	"4d63.com/gochecknoglobals/checknoglobals"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(checknoglobals.Analyzer()).
		WithDesc("Check that no global variables exist.").
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
