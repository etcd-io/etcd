package mirror

import (
	"github.com/butuzov/mirror"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	// mirror only lints test files if the `--with-tests` flag is passed,
	// so we pass the `with-tests` flag as true to the analyzer before running it.
	// This can be turned off by using the regular golangci-lint flags such as `--tests` or `--skip-files`
	// or can be disabled per linter via exclude rules.
	// (see https://github.com/golangci/golangci-lint/issues/2527#issuecomment-1023707262)
	cfg := map[string]any{
		"with-tests": true,
	}

	return goanalysis.
		NewLinterFromAnalyzer(mirror.NewAnalyzer()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
