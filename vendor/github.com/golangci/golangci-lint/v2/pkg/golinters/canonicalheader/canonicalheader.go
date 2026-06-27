package canonicalheader

import (
	"github.com/lasiar/canonicalheader"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(canonicalheader.Analyzer).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
