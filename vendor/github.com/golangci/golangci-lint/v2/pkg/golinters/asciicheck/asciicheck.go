package asciicheck

import (
	"github.com/golangci/asciicheck"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(asciicheck.NewAnalyzer()).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
