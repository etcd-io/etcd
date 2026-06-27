// Package max_len provides a checker for maximum line length of godocs.
package max_len

import (
	"fmt"
	gdc "go/doc/comment"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"

	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

const maxLenRule = model.MaxLenRule

var ruleSet = model.RuleSet{}.Add(maxLenRule)

// MaxLenChecker checks maximum line length of godocs.
type MaxLenChecker struct{}

// NewMaxLenChecker returns a new instance of the corresponding checker.
func NewMaxLenChecker() *MaxLenChecker {
	return &MaxLenChecker{}
}

// GetCoveredRules implements the corresponding interface method.
func (r *MaxLenChecker) GetCoveredRules() model.RuleSet {
	return ruleSet
}

// Apply implements the corresponding interface method.
func (r *MaxLenChecker) Apply(actx *model.AnalysisContext) error {
	includeTests := actx.Config.GetRuleOptions().MaxLenIncludeTests
	maxLen := int(actx.Config.GetRuleOptions().MaxLenLength)
	ignoreRegexps := actx.Config.GetRuleOptions().MaxLenIgnorePatterns

	docs := make(map[*model.CommentGroup]struct{}, 10*len(actx.InspectorResult.Files))

	for _, ir := range util.AnalysisApplicableFiles(actx, includeTests, model.RuleSet{}.Add(maxLenRule)) {
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
		checkMaxLen(actx, doc, maxLen, ignoreRegexps)
	}
	return nil
}

func checkMaxLen(actx *model.AnalysisContext, doc *model.CommentGroup, maxLen int, ignoreRegexps []*regexp.Regexp) {
	if doc.DisabledRules.All || doc.DisabledRules.Rules.Has(maxLenRule) {
		return
	}

	linkDefsMap := make(map[string]struct{}, len(doc.Parsed.Links))
	for _, linkDef := range doc.Parsed.Links {
		linkDefLine := fmt.Sprintf("[%s]: %s", linkDef.Text, linkDef.URL)
		linkDefsMap[linkDefLine] = struct{}{}
	}

	nonCodeBlocks := make([]gdc.Block, 0, len(doc.Parsed.Content))
	for _, b := range doc.Parsed.Content {
		if _, ok := b.(*gdc.Code); ok {
			continue
		}
		nonCodeBlocks = append(nonCodeBlocks, b)
	}
	strippedCodeAndLinks := &gdc.Doc{
		Content: nonCodeBlocks,
	}
	text := string((&gdc.Printer{}).Comment(strippedCodeAndLinks))
	linesIter := strings.SplitSeq(removeCarriageReturn(text), "\n")

	// This is a clone of the original comment list since we need to remove
	// elements from it, if needed. The main purpose of this is to don't miss
	// duplicate long lines.
	cgl := slices.Clone(doc.CG.List)

	for l := range linesIter {
		lineLen := utf8.RuneCountInString(l)
		if lineLen <= maxLen {
			continue
		}
		if shouldIgnoreLine(l, ignoreRegexps) {
			continue
		}

		// Here we try to find the accurate position of the line within the
		// original comment group. Historically, we would use the entire godoc
		// block range to report the issue (See #55).
		//
		// However, the following approach does not work with /*..*/ comment
		// blocks, since for them individual lines are not separately available,
		// i.e. all lines would be in a single ast.Comment instance and
		// therefore the ast.Comment.Pos will always point to the starting /*
		// token.

		foundAt := -1
		for i, c := range cgl {
			// As of [ast.Comment] docs, c.Text does not include carriage
			// returns (\r) that may have been present in the source. This is
			// good, since we have already stripped them from the lines.
			//
			// If the comment is a //-style one, c.Text starts with "// ". Since
			// we're only interested in //-style cases, it's enough to check for
			// that prefix.
			if c.Text == "// "+l {
				foundAt = i
				break
			}
		}

		var rng analysis.Range
		if foundAt != -1 {
			rng = cgl[foundAt]
			// Remove the found comment line from the list so that we don't miss
			// duplicate long lines, or even reporting the same line number more
			// than once while leaving the other(s).
			cgl = slices.Delete(cgl, foundAt, foundAt+1)
		} else {
			// Fallback to reporting the entire godoc block.
			rng = &doc.CG
		}

		actx.Pass.ReportRangef(rng, "godoc line is too long (%d > %d)", lineLen, maxLen)
	}
}

func shouldIgnoreLine(line string, ignoreRegexps []*regexp.Regexp) bool {
	for _, re := range ignoreRegexps {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func removeCarriageReturn(s string) string {
	return strings.ReplaceAll(s, "\r", "")
}
