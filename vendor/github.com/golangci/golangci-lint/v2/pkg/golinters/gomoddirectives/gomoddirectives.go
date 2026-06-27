package gomoddirectives

import (
	"regexp"
	"sync"

	"github.com/ldez/gomoddirectives"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const linterName = "gomoddirectives"

func New(settings *config.GoModDirectivesSettings) *goanalysis.Linter {
	var issues []*goanalysis.Issue
	var once sync.Once

	var opts gomoddirectives.Options
	if settings != nil {
		opts.ReplaceAllowLocal = settings.ReplaceLocal
		opts.ReplaceAllowList = settings.ReplaceAllowList
		opts.RetractAllowNoExplanation = settings.RetractAllowNoExplanation
		opts.ExcludeForbidden = settings.ExcludeForbidden
		opts.ToolchainForbidden = settings.ToolchainForbidden
		opts.ToolForbidden = settings.ToolForbidden
		opts.GoDebugForbidden = settings.GoDebugForbidden
		opts.CheckModulePath = settings.CheckModulePath

		if settings.ToolchainPattern != "" {
			exp, err := regexp.Compile(settings.ToolchainPattern)
			if err != nil {
				internal.LinterLogger.Fatalf("%s: invalid toolchain pattern: %v", linterName, err)
			} else {
				opts.ToolchainPattern = exp
			}
		}

		if settings.GoVersionPattern != "" {
			exp, err := regexp.Compile(settings.GoVersionPattern)
			if err != nil {
				internal.LinterLogger.Fatalf("%s: invalid Go version pattern: %v", linterName, err)
			} else {
				opts.GoVersionPattern = exp
			}
		}
	}

	analyzer := &analysis.Analyzer{
		Name: linterName,
		Doc:  "Manage the use of 'replace', 'retract', and 'excludes' directives in go.mod.",
		Run:  goanalysis.DummyRun,
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithContextSetter(func(lintCtx *linter.Context) {
			analyzer.Run = func(pass *analysis.Pass) (any, error) {
				once.Do(func() {
					results, err := gomoddirectives.AnalyzePass(pass, opts)
					if err != nil {
						lintCtx.Log.Warnf("running %s failed: %s: "+
							"if you are not using go modules it is suggested to disable this linter", linterName, err)
						return
					}

					for _, p := range results {
						issues = append(issues, goanalysis.NewIssue(&result.Issue{
							FromLinter: linterName,
							Pos:        p.Start,
							Text:       p.Reason,
						}, pass))
					}
				})

				return nil, nil
			}
		}).
		WithIssuesReporter(func(*linter.Context) []*goanalysis.Issue {
			return issues
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
