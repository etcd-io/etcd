package rules

import (
	"go/ast"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type httpServeWithoutTimeouts struct {
	callListRule
}

// NewHTTPServeWithoutTimeouts detects use of net/http serve functions that have no support for setting timeouts.
func NewHTTPServeWithoutTimeouts(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &httpServeWithoutTimeouts{
		callListRule: newCallListRule(id, "Use of net/http serve function that has no support for setting timeouts", issue.Medium, issue.High),
	}
	rule.AddAll("net/http", "ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
