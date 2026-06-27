package err113

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

func inspectComparision(file *ast.File, pass *analysis.Pass, n ast.Node) bool { // nolint: unparam
	// check whether the call expression matches time.Now().Sub()
	be, ok := n.(*ast.BinaryExpr)
	if !ok {
		return true
	}

	// check if it is a comparison operation
	if be.Op != token.EQL && be.Op != token.NEQ {
		return true
	}

	if !areBothErrors(be.X, be.Y, pass.TypesInfo) {
		return true
	}

	root := inspector.New([]*ast.File{file}).Root()
	c, ok := root.FindNode(n)
	if !ok {
		panic(fmt.Errorf("could not find node %T in inspector for file %q", n, file.Name.Name))
	}

	for cur := c.Parent(); cur != root; cur = cur.Parent() {
		if isMethodNamed(cur, pass, "Is") {
			return true
		}
	}

	oldExpr := render(pass.Fset, be)

	negate := ""
	if be.Op == token.NEQ {
		negate = "!"
	}

	newExpr := fmt.Sprintf("%s%s.Is(%s, %s)", negate, "errors", rawString(be.X), rawString(be.Y))

	pass.Report(
		analysis.Diagnostic{
			Pos:     be.Pos(),
			Message: fmt.Sprintf("do not compare errors directly %q, use %q instead", oldExpr, newExpr),
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: fmt.Sprintf("should replace %q with %q", oldExpr, newExpr),
					TextEdits: []analysis.TextEdit{
						{
							Pos:     be.Pos(),
							End:     be.End(),
							NewText: []byte(newExpr),
						},
					},
				},
			},
		},
	)

	return true
}

var errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func isMethodNamed(cur inspector.Cursor, pass *analysis.Pass, name string) bool {
	funcNode, ok := cur.Node().(*ast.FuncDecl)

	if !ok || funcNode.Name == nil || funcNode.Name.Name != name {
		return false
	}

	if funcNode.Recv == nil && len(funcNode.Recv.List) != 1 {
		return false
	}

	typ := pass.TypesInfo.Types[funcNode.Recv.List[0].Type]
	return typ.Type != nil && types.Implements(typ.Type, errorType)
}

func isError(v ast.Expr, info *types.Info) bool {
	if intf, ok := info.TypeOf(v).Underlying().(*types.Interface); ok {
		return intf.NumMethods() == 1 && intf.Method(0).FullName() == "(error).Error"
	}

	return false
}

func isEOF(ex ast.Expr, info *types.Info) bool {
	se, ok := ex.(*ast.SelectorExpr)
	if !ok || se.Sel.Name != "EOF" {
		return false
	}

	if ep, ok := asImportedName(se.X, info); !ok || ep != "io" {
		return false
	}

	return true
}

func asImportedName(ex ast.Expr, info *types.Info) (string, bool) {
	ei, ok := ex.(*ast.Ident)
	if !ok {
		return "", false
	}

	ep, ok := info.ObjectOf(ei).(*types.PkgName)
	if !ok {
		return "", false
	}

	return ep.Imported().Path(), true
}

func areBothErrors(x, y ast.Expr, typesInfo *types.Info) bool {
	// check that both left and right hand side are not nil
	if typesInfo.Types[x].IsNil() || typesInfo.Types[y].IsNil() {
		return false
	}

	// check that both left and right hand side are not io.EOF
	if isEOF(x, typesInfo) || isEOF(y, typesInfo) {
		return false
	}

	// check that both left and right hand side are errors
	if !isError(x, typesInfo) && !isError(y, typesInfo) {
		return false
	}

	return true
}

func rawString(x ast.Expr) string {
	switch t := x.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", rawString(t.X), t.Sel.Name)
	case *ast.CallExpr:
		return fmt.Sprintf("%s()", rawString(t.Fun))
	}
	return fmt.Sprintf("%s", x)
}
