package goanalysis

import (
	"fmt"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis/pkgerrors"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
	"github.com/golangci/golangci-lint/v2/pkg/timeutils"
)

type runAnalyzersConfig interface {
	getName() string
	getLinterNameForDiagnostic(*Diagnostic) string
	getAnalyzers() []*analysis.Analyzer
	useOriginalPackages() bool
	reportIssues(*linter.Context) []*Issue
	getLoadMode() LoadMode
}

func runAnalyzers(cfg runAnalyzersConfig, lintCtx *linter.Context) ([]*result.Issue, error) {
	log := lintCtx.Log.Child(logutils.DebugKeyGoAnalysis)
	sw := timeutils.NewStopwatch("analyzers", log)

	const stagesToPrint = 10
	defer sw.PrintTopStages(stagesToPrint)

	runner := newRunner(cfg.getName(), log, lintCtx.PkgCache, lintCtx.LoadGuard, cfg.getLoadMode(), sw)

	pkgs := lintCtx.Packages
	if cfg.useOriginalPackages() {
		pkgs = lintCtx.OriginalPackages
	}

	issues, pkgsFromCache := loadIssuesFromCache(pkgs, lintCtx, cfg.getAnalyzers())
	var pkgsToAnalyze []*packages.Package
	for _, pkg := range pkgs {
		if !pkgsFromCache[pkg] {
			pkgsToAnalyze = append(pkgsToAnalyze, pkg)
		}
	}

	diags, errs, passToPkg := runner.run(cfg.getAnalyzers(), pkgsToAnalyze)

	defer func() {
		if len(errs) == 0 {
			// If we try to save to cache even if we have compilation errors
			// we won't see them on repeated runs.
			saveIssuesToCache(pkgs, pkgsFromCache, issues, lintCtx, cfg.getAnalyzers())
		}
	}()

	buildAllIssues := func() []*result.Issue {
		var retIssues []*result.Issue

		reportedIssues := cfg.reportIssues(lintCtx)
		for _, reportedIssue := range reportedIssues {
			if reportedIssue.Pkg == nil {
				reportedIssue.Pkg = passToPkg[reportedIssue.Pass]
			}

			retIssues = append(retIssues, reportedIssue.Issue)
		}

		return slices.Concat(retIssues, buildIssues(diags, cfg.getLinterNameForDiagnostic))
	}

	errIssues, err := pkgerrors.BuildIssuesFromIllTypedError(errs, lintCtx)
	if err != nil {
		return nil, err
	}

	issues = append(issues, errIssues...)
	issues = append(issues, buildAllIssues()...)

	return issues, nil
}

func buildIssues(diags []*Diagnostic, linterNameBuilder func(diag *Diagnostic) string) []*result.Issue {
	var issues []*result.Issue

	for _, diag := range diags {
		linterName := linterNameBuilder(diag)

		var text string
		if diag.Analyzer.Name == linterName {
			text = diag.Message
		} else {
			text = fmt.Sprintf("%s: %s", diag.Analyzer.Name, diag.Message)
		}

		var suggestedFixes []analysis.SuggestedFix

		for _, sf := range diag.SuggestedFixes {
			// Skip suggested fixes on cgo files.
			// The related error is: "diff has out-of-bounds edits"
			// This is a temporary workaround.
			if !strings.HasSuffix(diag.File.Name(), ".go") {
				continue
			}

			nsf := analysis.SuggestedFix{Message: sf.Message}

			for _, edit := range sf.TextEdits {
				end := edit.End

				if !end.IsValid() {
					end = edit.Pos
				}

				// To be applied the positions need to be "adjusted" based on the file.
				// This is the difference between the "displayed" positions and "effective" positions.
				nsf.TextEdits = append(nsf.TextEdits, analysis.TextEdit{
					Pos:     token.Pos(diag.File.Offset(edit.Pos)),
					End:     token.Pos(diag.File.Offset(end)),
					NewText: edit.NewText,
				})
			}

			suggestedFixes = append(suggestedFixes, nsf)
		}

		issues = append(issues, &result.Issue{
			FromLinter:     linterName,
			Text:           text,
			Pos:            diag.Position,
			Pkg:            diag.Pkg,
			SuggestedFixes: suggestedFixes,
		})

		if len(diag.Related) > 0 {
			for _, info := range diag.Related {
				relatedPos := diag.Pkg.Fset.Position(info.Pos)

				if relatedPos.Filename != diag.Position.Filename {
					relatedPos = diag.Position
				}

				issues = append(issues, &result.Issue{
					FromLinter: linterName,
					Text:       fmt.Sprintf("%s(related information): %s", diag.Analyzer.Name, info.Message),
					Pos:        relatedPos,
					Pkg:        diag.Pkg,
				})
			}
		}
	}
	return issues
}
