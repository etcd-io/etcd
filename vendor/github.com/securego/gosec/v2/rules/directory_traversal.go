package rules

import (
	"go/ast"
	"regexp"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type traversal struct {
	pattern *regexp.Regexp
	issue.MetaData
}

func (r *traversal) Match(n ast.Node, ctx *gosec.Context) (*issue.Issue, error) {
	switch node := n.(type) {
	case *ast.CallExpr:
		return r.matchCallExpr(node, ctx)
	}
	return nil, nil
}

func (r *traversal) matchCallExpr(assign *ast.CallExpr, ctx *gosec.Context) (*issue.Issue, error) {
	for _, i := range assign.Args {
		if basiclit, ok1 := i.(*ast.BasicLit); ok1 {
			if fun, ok2 := assign.Fun.(*ast.SelectorExpr); ok2 {
				if x, ok3 := fun.X.(*ast.Ident); ok3 {
					str := x.Name + "." + fun.Sel.Name + "(" + basiclit.Value + ")"
					if gosec.RegexMatchWithCache(r.pattern, str) {
						return ctx.NewIssue(assign, r.ID(), r.What, r.Severity, r.Confidence), nil
					}
				}
			}
		}
	}
	return nil, nil
}

// NewDirectoryTraversal attempts to find the use of http.Dir("/")
func NewDirectoryTraversal(id string, conf gosec.Config) (gosec.Rule, []ast.Node) {
	pattern := `http\.Dir\("\/"\)|http\.Dir\('\/'\)`
	if val, ok := conf[id]; ok {
		conf := val.(map[string]interface{})
		if configPattern, ok := conf["pattern"]; ok {
			if cfgPattern, ok := configPattern.(string); ok {
				pattern = cfgPattern
			}
		}
	}

	return &traversal{
		pattern:  regexp.MustCompile(pattern),
		MetaData: issue.NewMetaData(id, "Potential directory traversal", issue.Medium, issue.Medium),
	}, []ast.Node{(*ast.CallExpr)(nil)}
}
