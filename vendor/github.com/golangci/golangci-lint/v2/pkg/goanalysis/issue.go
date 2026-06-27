package goanalysis

import (
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/result"
)

type Issue struct {
	*result.Issue
	Pass *analysis.Pass
}

func NewIssue(issue *result.Issue, pass *analysis.Pass) *Issue {
	return &Issue{
		Issue: issue,
		Pass:  pass,
	}
}

type EncodingIssue struct {
	FromLinter           string
	Text                 string
	Severity             string
	Pos                  token.Position
	LineRange            *result.Range
	SuggestedFixes       []analysis.SuggestedFix
	ExpectNoLint         bool
	ExpectedNoLintLinter string
}
