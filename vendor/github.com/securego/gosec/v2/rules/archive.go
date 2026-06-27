package rules

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type archive struct {
	callListRule
	argTypes []string
}

// getArchiveBaseType returns the underlying type (*archive/zip.File or *archive/tar.Header)
// if the expression is a direct .Name selector on such a type or a short-declared variable
// assigned from such a selector (e.g., name := file.Name).
func getArchiveBaseType(expr ast.Expr, ctx *gosec.Context, file *ast.File) types.Type {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		return ctx.Info.TypeOf(e.X)
	case *ast.Ident:
		obj := ctx.Info.ObjectOf(e)
		if v, ok := obj.(*types.Var); ok && file != nil {
			var baseType types.Type
			ast.Inspect(file, func(n ast.Node) bool {
				if assign, ok := n.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
					for i, lhs := range assign.Lhs {
						if id, ok := lhs.(*ast.Ident); ok &&
							id.Pos() == v.Pos() && ctx.Info.ObjectOf(id) == v {
							if i < len(assign.Rhs) {
								if sel, ok := assign.Rhs[i].(*ast.SelectorExpr); ok {
									baseType = ctx.Info.TypeOf(sel.X)
								}
							}
							return false // Stop once defining assignment found
						}
					}
				}
				return true
			})
			return baseType
		}
	}
	return nil
}

// Match inspects AST nodes to determine if filepath.Join uses an argument derived
// from zip.File or tar.Header (typically the unsafe .Name field).
func (a *archive) Match(n ast.Node, ctx *gosec.Context) (*issue.Issue, error) {
	if node := a.calls.ContainsPkgCallExpr(n, ctx, false); node != nil {
		// All relevant variables are local (archive extraction context), so inspect the file containing the call
		file := gosec.ContainingFile(node, ctx)
		for _, arg := range node.Args {
			if baseType := getArchiveBaseType(arg, ctx, file); baseType != nil {
				if slices.Contains(a.argTypes, baseType.String()) {
					return ctx.NewIssue(n, a.ID(), a.What, a.Severity, a.Confidence), nil
				}
			}
		}
	}
	return nil, nil
}

// NewArchive creates a new rule which detects file traversal when extracting zip/tar archives.
func NewArchive(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &archive{
		callListRule: newCallListRule(id, "File traversal when extracting zip/tar archive", issue.Medium, issue.High),
		argTypes:     []string{"*archive/zip.File", "*archive/tar.Header"},
	}
	rule.Add("path/filepath", "Join").Add("path", "Join")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
