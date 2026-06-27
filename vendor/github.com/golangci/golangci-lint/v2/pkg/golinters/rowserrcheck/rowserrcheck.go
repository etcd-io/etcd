package rowserrcheck

import (
	"github.com/golangci/rowserrcheck/passes/rowserr"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.RowsErrCheckSettings) *goanalysis.Linter {
	var pkgs []string

	if settings != nil {
		pkgs = settings.Packages
	}

	return goanalysis.
		NewLinterFromAnalyzer(rowserr.NewAnalyzer(pkgs...)).
		WithDesc("checks whether Rows.Err of rows is checked successfully").
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
