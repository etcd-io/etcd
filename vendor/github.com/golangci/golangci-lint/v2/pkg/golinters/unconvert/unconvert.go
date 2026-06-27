package unconvert

import (
	"sync"

	"github.com/golangci/unconvert"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const linterName = "unconvert"

func New(settings *config.UnconvertSettings) *goanalysis.Linter {
	var mu sync.Mutex
	var resIssues []*goanalysis.Issue

	unconvert.SetFastMath(settings.FastMath)
	unconvert.SetSafe(settings.Safe)

	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: linterName,
			Doc:  "Remove unnecessary type conversions",
			Run: func(pass *analysis.Pass) (any, error) {
				issues := runUnconvert(pass)

				if len(issues) == 0 {
					return nil, nil
				}

				mu.Lock()
				resIssues = append(resIssues, issues...)
				mu.Unlock()

				return nil, nil
			},
		}).
		WithIssuesReporter(func(*linter.Context) []*goanalysis.Issue {
			return resIssues
		}).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}

func runUnconvert(pass *analysis.Pass) []*goanalysis.Issue {
	positions := unconvert.Run(pass)

	var issues []*goanalysis.Issue
	for _, position := range positions {
		issues = append(issues, goanalysis.NewIssue(&result.Issue{
			Pos:        position,
			Text:       "unnecessary conversion",
			FromLinter: linterName,
		}, pass))
	}

	return issues
}
