package nestif

import (
	"github.com/nakabonne/nestif"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.NestifSettings) *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: "nestif",
			Doc:  "Reports deeply nested if statements",
			Run: func(pass *analysis.Pass) (any, error) {
				runNestIf(pass, settings)

				return nil, nil
			},
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}

func runNestIf(pass *analysis.Pass, settings *config.NestifSettings) {
	checker := &nestif.Checker{
		MinComplexity: settings.MinComplexity,
	}

	for _, file := range pass.Files {
		position, isGoFile := goanalysis.GetGoFilePosition(pass, file)
		if !isGoFile {
			continue
		}

		issues := checker.Check(file, pass.Fset)
		if len(issues) == 0 {
			continue
		}

		nonAdjPosition := pass.Fset.PositionFor(file.Pos(), false)

		f := pass.Fset.File(file.Pos())

		for _, issue := range issues {
			pass.Report(analysis.Diagnostic{
				Pos:     f.LineStart(goanalysis.AdjustPos(issue.Pos.Line, nonAdjPosition.Line, position.Line)),
				Message: issue.Message,
			})
		}
	}
}
