package gocognit

import (
	"fmt"
	"sort"
	"sync"

	"github.com/uudashr/gocognit"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const linterName = "gocognit"

func New(settings *config.GocognitSettings) *goanalysis.Linter {
	var mu sync.Mutex
	var resIssues []*goanalysis.Issue

	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: linterName,
			Doc:  "Computes and checks the cognitive complexity of functions",
			Run: func(pass *analysis.Pass) (any, error) {
				issues := runGocognit(pass, settings)

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
		WithLoadMode(goanalysis.LoadModeSyntax)
}

func runGocognit(pass *analysis.Pass, settings *config.GocognitSettings) []*goanalysis.Issue {
	var stats []gocognit.Stat
	for _, f := range pass.Files {
		stats = gocognit.ComplexityStats(f, pass.Fset, stats)
	}
	if len(stats) == 0 {
		return nil
	}

	sort.SliceStable(stats, func(i, j int) bool {
		return stats[i].Complexity > stats[j].Complexity
	})

	issues := make([]*goanalysis.Issue, 0, len(stats))
	for _, s := range stats {
		if s.Complexity <= settings.MinComplexity {
			break // Break as the stats is already sorted from greatest to least
		}

		issues = append(issues, goanalysis.NewIssue(&result.Issue{
			Pos: s.Pos,
			Text: fmt.Sprintf("cognitive complexity %d of func %s is high (> %d)",
				s.Complexity, internal.FormatCode(s.FuncName), settings.MinComplexity),
			FromLinter: linterName,
		}, pass))
	}

	return issues
}
