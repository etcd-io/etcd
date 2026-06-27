package rules

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type implicitAliasing struct {
	issue.MetaData
	aliases         map[*types.Var]struct{}
	rightBrace      token.Pos
	acceptableAlias []*ast.UnaryExpr
}

func containsUnary(exprs []*ast.UnaryExpr, expr *ast.UnaryExpr) bool {
	for _, e := range exprs {
		if e == expr {
			return true
		}
	}
	return false
}

func getIdentExpr(expr ast.Expr) (*ast.Ident, bool) {
	return doGetIdentExpr(expr, false)
}

func doGetIdentExpr(expr ast.Expr, hasSelector bool) (*ast.Ident, bool) {
	switch node := expr.(type) {
	case *ast.Ident:
		return node, hasSelector
	case *ast.SelectorExpr:
		return doGetIdentExpr(node.X, true)
	case *ast.UnaryExpr:
		return doGetIdentExpr(node.X, hasSelector)
	default:
		return nil, false
	}
}

func (r *implicitAliasing) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	// This rule does not apply for Go 1.22+, where range loop variables have per-iteration scope.
	// See https://go.dev/doc/go1.22#language.
	major, minor, _ := gosec.GoVersion()
	if major == 1 && minor >= 22 || major > 1 {
		return nil, nil
	}

	switch node := n.(type) {
	case *ast.RangeStmt:
		// Add the range value variable (if it's an identifier) to the set of aliased loop vars.
		if valueIdent, ok := node.Value.(*ast.Ident); ok {
			if obj := c.Info.ObjectOf(valueIdent); obj != nil {
				if v, ok := obj.(*types.Var); ok {
					r.aliases[v] = struct{}{}
					if r.rightBrace < node.Body.Rbrace {
						r.rightBrace = node.Body.Rbrace
					}
				}
			}
		}

	case *ast.UnaryExpr:
		// Clear aliases if we're outside the last tracked range loop body.
		if node.Pos() > r.rightBrace {
			r.aliases = make(map[*types.Var]struct{})
			r.acceptableAlias = make([]*ast.UnaryExpr, 0)
		}

		// Short-circuit if no aliases to check.
		if len(r.aliases) == 0 {
			return nil, nil
		}

		// Acceptable if this &expr is directly returned (top-level in return stmt).
		if containsUnary(r.acceptableAlias, node) {
			return nil, nil
		}

		// Check for & on a tracked loop variable.
		if node.Op == token.AND {
			if identExpr, hasSelector := getIdentExpr(node.X); identExpr != nil {
				if obj := c.Info.ObjectOf(identExpr); obj != nil {
					if v, ok := obj.(*types.Var); ok {
						if _, aliased := r.aliases[v]; aliased {
							_, isPointer := c.Info.TypeOf(identExpr).(*types.Pointer)
							if !hasSelector || !isPointer {
								return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
							}
						}
					}
				}
			}
		}

	case *ast.ReturnStmt:
		// Mark direct &loopVar in return statements as acceptable (only one iteration's value returned).
		for _, res := range node.Results {
			if unary, ok := res.(*ast.UnaryExpr); ok && unary.Op == token.AND {
				r.acceptableAlias = append(r.acceptableAlias, unary)
			}
		}
	}

	return nil, nil
}

// NewImplicitAliasing detects implicit memory aliasing in range loops (pre-Go 1.22).
func NewImplicitAliasing(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	return &implicitAliasing{
		aliases:         make(map[*types.Var]struct{}),
		rightBrace:      token.NoPos,
		acceptableAlias: make([]*ast.UnaryExpr, 0),
		MetaData:        issue.NewMetaData(id, "Implicit memory aliasing in for loop.", issue.Medium, issue.Medium),
	}, []ast.Node{(*ast.RangeStmt)(nil), (*ast.UnaryExpr)(nil), (*ast.ReturnStmt)(nil)}
}

/*
This rule is prone to flag false positives.

Within GoSec, the rule is just an AST match-- there are a handful of other
implementation strategies which might lend more nuance to the rule at the
cost of allowing false negatives.

From a tooling side, I'd rather have this rule flag false positives than
potentially have some false negatives-- especially if the sentiment of this
rule (as I understand it, and Go) is that referencing a rangeStmt-yielded
value is kinda strange and does not have a strongly justified use case.

Which is to say-- a false positive _should_ just be changed.
*/
