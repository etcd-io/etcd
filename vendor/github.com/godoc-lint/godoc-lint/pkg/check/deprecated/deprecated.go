// Package deprecated provides a checker for correct usage of deprecation
// markers.
package deprecated

import (
	"go/ast"
	"go/doc/comment"
	"regexp"

	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

const deprecatedRule = model.DeprecatedRule

var ruleSet = model.RuleSet{}.Add(deprecatedRule)

// DeprecatedChecker checks correct usage of "Deprecated:" markers.
type DeprecatedChecker struct{}

// NewDeprecatedChecker returns a new instance of the corresponding checker.
func NewDeprecatedChecker() *DeprecatedChecker {
	return &DeprecatedChecker{}
}

// GetCoveredRules implements the corresponding interface method.
func (r *DeprecatedChecker) GetCoveredRules() model.RuleSet {
	return ruleSet
}

// Apply implements the corresponding interface method.
func (r *DeprecatedChecker) Apply(actx *model.AnalysisContext) error {
	docs := make(map[*model.CommentGroup]struct{}, 10*len(actx.InspectorResult.Files))

	for _, ir := range util.AnalysisApplicableFiles(actx, false, model.RuleSet{}.Add(deprecatedRule)) {
		if ir.PackageDoc != nil {
			docs[ir.PackageDoc] = struct{}{}
		}

		for _, sd := range ir.SymbolDecl {
			isExported := ast.IsExported(sd.Name)
			if !isExported {
				continue
			}

			if sd.ParentDoc != nil {
				docs[sd.ParentDoc] = struct{}{}
			}
			if sd.Doc == nil {
				continue
			}
			docs[sd.Doc] = struct{}{}
		}
	}

	for doc := range docs {
		checkDeprecations(actx, doc)
	}
	return nil
}

// probableDeprecationRE finds probable deprecation markers, including the
// correct usage.
var probableDeprecationRE = regexp.MustCompile(`(?i)^deprecated:.?`)

const correctDeprecationMarker = "Deprecated: "

func checkDeprecations(actx *model.AnalysisContext, doc *model.CommentGroup) {
	if doc.DisabledRules.All || doc.DisabledRules.Rules.Has(deprecatedRule) {
		return
	}

	for _, block := range doc.Parsed.Content {
		// The correct usage of deprecation markers is to put them at the beginning
		// of a paragraph (i.e. not a heading, code block, etc). Also the syntax is
		// strict and must only be "Deprecated: " (case-sensitive and with the
		// trailing whitespace).
		//
		// However, not all wrong usages are reliably discoverable. For example:
		//
		//   // Foo is a symbol.
		//   // deprecated: use Bar.
		//   // func Foo() {}
		//
		// In this cases, the "deprecated: " marker is at the beginning of a new
		// line, but as of godoc parser, it's just in the middle of the paragraph
		// started at the top line. There might be some smart ways to detect these
		// cases, but the problem is there will be false-positives to them. For
		// instance, a valid case like this should never be detected as a wrong
		// usage:
		//
		//   // Foo is a symbol but here are the reasons why it's
		//   // deprecated: blah, blah, ...
		//   // func Foo() {}
		//
		// Needless to say we don't want to deal with human language complexities.

		par, ok := block.(*comment.Paragraph)
		if !ok || len(par.Text) == 0 {
			continue
		}
		text, ok := (par.Text[0]).(comment.Plain)
		if !ok {
			continue
		}

		if match := probableDeprecationRE.FindString(string(text)); match == "" || match == correctDeprecationMarker {
			continue
		}

		actx.Pass.ReportRangef(&doc.CG, "deprecation note should be formatted as %q", correctDeprecationMarker)
		break
	}
}
