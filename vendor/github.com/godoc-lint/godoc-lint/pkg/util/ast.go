package util

import (
	"go/ast"
	"go/token"
	"iter"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/godoc-lint/godoc-lint/pkg/model"
)

// GetPassFileToken is a helper function to return the file token associated
// with the given AST file.
func GetPassFileToken(f *ast.File, pass *analysis.Pass) *token.File {
	if f.Pos() == token.NoPos {
		return nil
	}
	ft := pass.Fset.File(f.Pos())
	if ft == nil {
		return nil
	}
	return ft
}

// AnalysisApplicableFiles returns an iterator looping over files that are ready
// to be analyzed.
//
// The yield-ed arguments are never nil.
func AnalysisApplicableFiles(actx *model.AnalysisContext, includeTests bool, ruleSet model.RuleSet) iter.Seq2[*ast.File, *model.FileInspection] {
	return func(yield func(*ast.File, *model.FileInspection) bool) {
		if actx.InspectorResult == nil {
			return
		}

		for _, f := range actx.Pass.Files {
			ir := actx.InspectorResult.Files[f]

			if ir == nil {
				continue
			}

			ft := GetPassFileToken(f, actx.Pass)
			if ft == nil {
				continue
			}

			if !actx.Config.IsPathApplicable(ft.Name()) {
				continue
			}

			if !includeTests && strings.HasSuffix(ft.Name(), "_test.go") {
				continue
			}

			if ir.DisabledRules.All || ir.DisabledRules.Rules.IsSupersetOf(ruleSet) {
				continue
			}

			if !yield(f, ir) {
				return
			}
		}
	}
}
