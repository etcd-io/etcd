// Package no_unused_link provides a checker for unused links in godocs.
package no_unused_link

import (
	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

const noUnusedLinkRule = model.NoUnusedLinkRule

var ruleSet = model.RuleSet{}.Add(noUnusedLinkRule)

// NoUnusedLinkChecker checks for unused links.
type NoUnusedLinkChecker struct{}

// NewNoUnusedLinkChecker returns a new instance of the corresponding checker.
func NewNoUnusedLinkChecker() *NoUnusedLinkChecker {
	return &NoUnusedLinkChecker{}
}

// GetCoveredRules implements the corresponding interface method.
func (r *NoUnusedLinkChecker) GetCoveredRules() model.RuleSet {
	return ruleSet
}

// Apply implements the corresponding interface method.
func (r *NoUnusedLinkChecker) Apply(actx *model.AnalysisContext) error {
	includeTests := actx.Config.GetRuleOptions().NoUnusedLinkIncludeTests

	docs := make(map[*model.CommentGroup]struct{}, 10*len(actx.InspectorResult.Files))

	for _, ir := range util.AnalysisApplicableFiles(actx, includeTests, model.RuleSet{}.Add(noUnusedLinkRule)) {
		if ir.PackageDoc != nil {
			docs[ir.PackageDoc] = struct{}{}
		}

		for _, sd := range ir.SymbolDecl {
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
		checkNoUnusedLink(actx, doc)
	}
	return nil
}

func checkNoUnusedLink(actx *model.AnalysisContext, doc *model.CommentGroup) {
	if doc.DisabledRules.All || doc.DisabledRules.Rules.Has(noUnusedLinkRule) {
		return
	}

	if doc.Text == "" {
		return
	}

	for _, linkDef := range doc.Parsed.Links {
		if linkDef.Used {
			continue
		}
		actx.Pass.ReportRangef(&doc.CG, "godoc has unused link (%q)", linkDef.Text)
	}
}
