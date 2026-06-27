package rules

import (
	"go/ast"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

// callListRule is a base for rules that simply check a CallList and issue on match.
// It provides the standard Match() implementation used by most call-based rules.
type callListRule struct {
	issue.MetaData
	calls gosec.CallList
}

func newCallListRule(id, what string, severity, confidence issue.Score) callListRule {
	return callListRule{
		MetaData: issue.NewMetaData(id, what, severity, confidence),
		calls:    gosec.NewCallList(),
	}
}

func (r *callListRule) Add(selector, ident string) *callListRule {
	r.calls.Add(selector, ident)
	return r
}

func (r *callListRule) AddAll(selector string, idents ...string) *callListRule {
	r.calls.AddAll(selector, idents...)
	return r
}

func (r *callListRule) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if r.calls.ContainsPkgCallExpr(n, c, false) != nil {
		return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
	}
	return nil, nil
}
