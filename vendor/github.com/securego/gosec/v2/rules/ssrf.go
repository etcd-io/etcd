package rules

import (
	"go/ast"
	"go/types"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type ssrf struct {
	callListRule
}

// ResolveVar tries to resolve the first argument of a call expression
// The first argument is the url
func (r *ssrf) ResolveVar(n *ast.CallExpr, c *gosec.Context) bool {
	if len(n.Args) > 0 {
		arg := n.Args[0]
		if ident, ok := arg.(*ast.Ident); ok {
			obj := c.Info.ObjectOf(ident)
			if _, ok := obj.(*types.Var); ok {
				scope := c.Pkg.Scope()
				if scope != nil && scope.Lookup(ident.Name) != nil {
					// a URL defined in a variable at package scope can be changed at any time
					return true
				}
				if !gosec.TryResolve(ident, c) {
					return true
				}
			}
		}
	}
	return false
}

// Match inspects AST nodes to determine if certain net/http methods are called with variable input
func (r *ssrf) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	// Call expression is using http package directly
	if node := r.calls.ContainsPkgCallExpr(n, c, false); node != nil {
		if r.ResolveVar(node, c) {
			return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
		}
	}
	return nil, nil
}

// NewSSRFCheck detects cases where HTTP requests are sent
func NewSSRFCheck(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &ssrf{newCallListRule(id, "Potential HTTP request made with variable url", issue.Medium, issue.Medium)}
	rule.AddAll("net/http", "Do", "Get", "Head", "Post", "PostForm", "RoundTrip")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
