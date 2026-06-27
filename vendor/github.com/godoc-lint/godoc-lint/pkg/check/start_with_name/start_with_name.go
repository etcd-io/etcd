// Package start_with_name provides a checker for godocs starting with the
// symbol name.
package start_with_name

import (
	"go/ast"
	"regexp"
	"strings"

	"github.com/godoc-lint/godoc-lint/pkg/check/shared"
	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

const startWithNameRule = model.StartWithNameRule

var ruleSet = model.RuleSet{}.Add(startWithNameRule)

// StartWithNameChecker checks starter of godocs.
type StartWithNameChecker struct{}

// NewStartWithNameChecker returns a new instance of the corresponding checker.
func NewStartWithNameChecker() *StartWithNameChecker {
	return &StartWithNameChecker{}
}

// GetCoveredRules implements the corresponding interface method.
func (r *StartWithNameChecker) GetCoveredRules() model.RuleSet {
	return ruleSet
}

// Apply implements the corresponding interface method.
func (r *StartWithNameChecker) Apply(actx *model.AnalysisContext) error {
	if !actx.Config.IsAnyRuleApplicable(model.RuleSet{}.Add(startWithNameRule)) {
		return nil
	}

	includeTests := actx.Config.GetRuleOptions().StartWithNameIncludeTests
	includePrivate := actx.Config.GetRuleOptions().StartWithNameIncludeUnexported

	for _, ir := range util.AnalysisApplicableFiles(actx, includeTests, model.RuleSet{}.Add(startWithNameRule)) {
		for _, decl := range ir.SymbolDecl {
			isExported := ast.IsExported(decl.Name)
			if decl.IsMethod && decl.MethodRecvBaseTypeName != "" {
				// A method is considered exported (in terms of godoc visibility)
				// only if both the method name and the base type name are exported.
				isExported = isExported && ast.IsExported(decl.MethodRecvBaseTypeName)
			}

			if !isExported && !includePrivate {
				continue
			}

			if decl.Name == "_" {
				// Blank identifiers should be ignored; e.g.:
				//
				//   var _ = 0
				continue
			}

			if decl.Kind == model.SymbolDeclKindBad {
				continue
			}

			if decl.Doc == nil || decl.Doc.Text == "" {
				continue
			}

			if decl.Doc.DisabledRules.All || decl.Doc.DisabledRules.Rules.Has(startWithNameRule) {
				continue
			}

			if decl.MultiNameDecl {
				continue
			}

			if shared.HasDeprecatedParagraph(decl.Doc.Parsed.Content) {
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

			if matchSymbolName(decl.Doc.Text, decl.Name) {
				continue
			}

			actx.Pass.ReportRangef(&decl.Doc.CG, "godoc should start with symbol name (%q)", decl.Name)
		}
	}
	return nil
}

var (
	startPattern                = regexp.MustCompile(`^(?:(A|a|AN|An|an|THE|The|the) )?(?P<symbol_name>.+?)\b`)
	startPatternSymbolNameIndex = startPattern.SubexpIndex("symbol_name")
)

func matchSymbolName(text, symbol string) bool {
	head := strings.SplitN(text, "\n", 2)[0]
	head, _ = strings.CutPrefix(head, "\r")
	head = strings.SplitN(head, " ", 2)[0]
	head = strings.SplitN(head, "\t", 2)[0]

	if head == symbol {
		return true
	}

	match := startPattern.FindStringSubmatch(text)
	if match == nil {
		return false
	}
	return match[startPatternSymbolNameIndex] == symbol
}
