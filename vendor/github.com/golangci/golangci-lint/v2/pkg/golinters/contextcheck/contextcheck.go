package contextcheck

import (
	"github.com/kkHAIKE/contextcheck"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/inspect"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
)

func New() *goanalysis.Linter {
	analyzer := contextcheck.NewAnalyzer(contextcheck.Configuration{})
	// TODO(ldez) there is a problem with this linter:
	// I think the problem related to facts.
	// The BuildSSA pass has been changed inside (0.39.0):
	// https://github.com/golang/tools/commit/b74c09864920a69a4d2f6ef0ecb4f9cff226893a
	analyzer.Requires = append(analyzer.Requires, ctrlflow.Analyzer, inspect.Analyzer)

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithContextSetter(func(lintCtx *linter.Context) {
			analyzer.Run = contextcheck.NewRun(lintCtx.Packages, false)
		}).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
