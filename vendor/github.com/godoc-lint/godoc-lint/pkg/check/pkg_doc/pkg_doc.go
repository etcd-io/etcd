// Package pkg_doc provides a checker for package godocs.
package pkg_doc

import (
	"go/ast"
	"strings"

	"github.com/godoc-lint/godoc-lint/pkg/check/shared"
	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

const (
	pkgDocRule        = model.PkgDocRule
	singlePkgDocRule  = model.SinglePkgDocRule
	requirePkgDocRule = model.RequirePkgDocRule
)

var ruleSet = model.RuleSet{}.Add(
	pkgDocRule,
	singlePkgDocRule,
	requirePkgDocRule,
)

// PkgDocChecker checks package godocs.
type PkgDocChecker struct{}

// NewPkgDocChecker returns a new instance of the corresponding checker.
func NewPkgDocChecker() *PkgDocChecker {
	return &PkgDocChecker{}
}

// GetCoveredRules implements the corresponding interface method.
func (r *PkgDocChecker) GetCoveredRules() model.RuleSet {
	return ruleSet
}

// Apply implements the corresponding interface method.
func (r *PkgDocChecker) Apply(actx *model.AnalysisContext) error {
	checkPkgDocRule(actx)
	checkSinglePkgDocRule(actx)
	checkRequirePkgDocRule(actx)
	return nil
}

const (
	commandPkgName     = "main"
	commandTestPkgName = "main_test"
)

func checkPkgDocRule(actx *model.AnalysisContext) {
	if !actx.Config.IsAnyRuleApplicable(model.RuleSet{}.Add(pkgDocRule)) {
		return
	}

	includeTests := actx.Config.GetRuleOptions().PkgDocIncludeTests

	for f, ir := range util.AnalysisApplicableFiles(actx, includeTests, model.RuleSet{}.Add(pkgDocRule)) {
		if ir.PackageDoc == nil {
			continue
		}

		if ir.PackageDoc.DisabledRules.All || ir.PackageDoc.DisabledRules.Rules.Has(pkgDocRule) {
			continue
		}

		if f.Name.Name == commandPkgName || f.Name.Name == commandTestPkgName {
			// Skip command packages, as they are not required to start with
			// "Package main" or "Package main_test".
			//
			// See for more details:
			//   - https://github.com/godoc-lint/godoc-lint/issues/10
			//   - https://go.dev/doc/comment#cmd
			continue
		}

		if ir.PackageDoc.Text == "" {
			continue
		}

		if shared.HasDeprecatedParagraph(ir.PackageDoc.Parsed.Content) {
			// If there's a paragraph starting with "Deprecated:", we skip the
			// entire godoc. The reason is a deprecated symbol will not appear
			// when docs are rendered.
			//
			// Another reason is that we cannot just skip those paragraphs and
			// look for the symbol in the remaining text. To do that, we need
			// to iterate over all comment.Block nodes, and check if a block
			// is a paragraph AND starts with the deprecation marker. This is
			// simple, but the challenge appears when we get to the first block
			// that does not have the marker and we want to check if it starts
			// with the symbol name. We'd expect that to be a paragraph, but
			// that is not always the case. For example, take this decl:
			//
			//    // Deprecated: blah blah
			//    //
			//    // Foo is integer
			//    //
			//    // Deprecation: blah blah
			//    type Foo int
			//
			// The first block is a paragraph which we can easily skip due to
			// the "Deprecated:" marker. However, the second block is actually
			// parsed as a heading. One can verify this by copy/pasting it in
			// a Go file and check the gopls hover.
			//
			// There might be a workaround for this, but this also means the
			// godoc parser behaves in unexpected ways, and until we don't
			// really know the extent of its quirks, it's safer to just skip
			// further checks on such godocs.
			continue
		}

		if expectedPrefix, ok := checkPkgDocPrefix(ir.PackageDoc.Text, f.Name.Name); !ok {
			actx.Pass.Reportf(ir.PackageDoc.CG.Pos(), "package godoc should start with %q", expectedPrefix+" ")
		}
	}
}

func checkPkgDocPrefix(text, packageName string) (string, bool) {
	expectedPrefix := "Package " + packageName
	if !strings.HasPrefix(text, expectedPrefix) {
		return expectedPrefix, false
	}
	rest := text[len(expectedPrefix):]
	return expectedPrefix, rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\r' || rest[0] == '\n'
}

func checkSinglePkgDocRule(actx *model.AnalysisContext) {
	if !actx.Config.IsAnyRuleApplicable(model.RuleSet{}.Add(singlePkgDocRule)) {
		return
	}

	includeTests := actx.Config.GetRuleOptions().SinglePkgDocIncludeTests

	documentedPkgs := make(map[string][]*ast.File, 2)

	for f, ir := range util.AnalysisApplicableFiles(actx, includeTests, model.RuleSet{}.Add(singlePkgDocRule)) {
		if ir.PackageDoc == nil || ir.PackageDoc.Text == "" {
			continue
		}

		if ir.PackageDoc.DisabledRules.All || ir.PackageDoc.DisabledRules.Rules.Has(singlePkgDocRule) {
			continue
		}

		pkg := f.Name.Name
		if _, ok := documentedPkgs[pkg]; !ok {
			documentedPkgs[pkg] = make([]*ast.File, 0, 2)
		}
		documentedPkgs[pkg] = append(documentedPkgs[pkg], f)
	}

	for pkg, fs := range documentedPkgs {
		if len(fs) < 2 {
			continue
		}
		for _, f := range fs {
			ir := actx.InspectorResult.Files[f]
			actx.Pass.Reportf(ir.PackageDoc.CG.Pos(), "package has more than one godoc (%q)", pkg)
		}
	}
}

func checkRequirePkgDocRule(actx *model.AnalysisContext) {
	if !actx.Config.IsAnyRuleApplicable(model.RuleSet{}.Add(requirePkgDocRule)) {
		return
	}

	includeTests := actx.Config.GetRuleOptions().RequirePkgDocIncludeTests

	pkgFiles := make(map[string][]*ast.File, 2)

	for f := range util.AnalysisApplicableFiles(actx, includeTests, model.RuleSet{}.Add(requirePkgDocRule)) {
		pkg := f.Name.Name
		if _, ok := pkgFiles[pkg]; !ok {
			pkgFiles[pkg] = make([]*ast.File, 0, len(actx.Pass.Files))
		}
		pkgFiles[pkg] = append(pkgFiles[pkg], f)
	}

	for pkg, fs := range pkgFiles {
		pkgHasGodoc := false
		for _, f := range fs {
			ir := actx.InspectorResult.Files[f]

			if ir.PackageDoc == nil || ir.PackageDoc.Text == "" {
				continue
			}

			if ir.PackageDoc.DisabledRules.All || ir.PackageDoc.DisabledRules.Rules.Has(requirePkgDocRule) {
				continue
			}

			pkgHasGodoc = true
			break
		}

		if pkgHasGodoc {
			continue
		}

		// Add a diagnostic message to the first file of the package.
		actx.Pass.Reportf(fs[0].Name.Pos(), "package should have a godoc (%q)", pkg)
	}
}
