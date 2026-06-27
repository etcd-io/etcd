package checknoglobals

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// allowedExpression is a struct representing packages and methods that will
// be an allowed combination to use as a global variable, f.ex. Name `regexp`
// and SelName `MustCompile`.
type allowedExpression struct {
	Name    string
	SelName string
}

const Doc = `check that no global variables exist

This analyzer checks for global variables and errors on any found.

A global variable is a variable declared in package scope and that can be read
and written to by any function within the package. Global variables can cause
side effects which are difficult to keep track of. A code in one function may
change the variables state while another unrelated chunk of code may be
effected by it.`

// Analyzer provides an Analyzer that checks that there are no global
// variables, except for errors and variables containing regular
// expressions.
func Analyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:             "gochecknoglobals",
		Doc:              Doc,
		Run:              checkNoGlobals,
		RunDespiteErrors: true,
	}
}

func isAllowed(cm ast.CommentMap, v ast.Node, ti *types.Info) bool {
	switch i := v.(type) {
	case *ast.GenDecl:
		return hasEmbedComment(cm, i)
	case *ast.Ident:
		return i.Name == "_" || i.Name == "version" || isError(i, ti) || identHasEmbedComment(cm, i)
	case *ast.CallExpr:
		if expr, ok := i.Fun.(*ast.SelectorExpr); ok {
			return isAllowedSelectorExpression(expr)
		}
	case *ast.CompositeLit:
		if expr, ok := i.Type.(*ast.SelectorExpr); ok {
			return isAllowedSelectorExpression(expr)
		}
	}

	return false
}

func isAllowedSelectorExpression(v *ast.SelectorExpr) bool {
	x, ok := v.X.(*ast.Ident)
	if !ok {
		return false
	}

	allowList := []allowedExpression{
		{Name: "regexp", SelName: "MustCompile"},
	}

	for _, i := range allowList {
		if x.Name == i.Name && v.Sel.Name == i.SelName {
			return true
		}
	}

	return false
}

// isError reports whether the AST identifier looks like
// an error and implements the error interface.
func isError(i *ast.Ident, ti *types.Info) bool {
	return looksLikeError(i) && implementsError(i, ti)
}

// looksLikeError returns true if the AST identifier starts
// with 'err' or 'Err', or false otherwise.
func looksLikeError(i *ast.Ident) bool {
	prefix := "err"
	if i.IsExported() {
		prefix = "Err"
	}
	return strings.HasPrefix(i.Name, prefix)
}

// implementsError reports whether the AST identifier
// implements the error interface.
func implementsError(i *ast.Ident, ti *types.Info) bool {
	t := ti.TypeOf(i)
	et := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	return types.Implements(t, et)
}

func identHasEmbedComment(cm ast.CommentMap, i *ast.Ident) bool {
	if i.Obj == nil {
		return false
	}

	spec, ok := i.Obj.Decl.(*ast.ValueSpec)
	if !ok {
		return false
	}

	return hasEmbedComment(cm, spec)
}

// hasEmbedComment returns true if the AST node has
// a '//go:embed ' comment, or false otherwise.
func hasEmbedComment(cm ast.CommentMap, n ast.Node) bool {
	for _, g := range cm[n] {
		for _, c := range g.List {
			if strings.HasPrefix(c.Text, "//go:embed ") {
				return true
			}
		}
	}
	return false
}

func checkNoGlobals(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if !strings.HasSuffix(filename, ".go") {
			continue
		}

		fileCommentMap := ast.NewCommentMap(pass.Fset, file, file.Comments)

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if genDecl.Tok != token.VAR {
				continue
			}
			if isAllowed(fileCommentMap, genDecl, pass.TypesInfo) {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec := spec.(*ast.ValueSpec)
				onlyAllowedValues := false

				for _, vn := range valueSpec.Values {
					if isAllowed(fileCommentMap, vn, pass.TypesInfo) {
						onlyAllowedValues = true
						continue
					}

					onlyAllowedValues = false
					break
				}

				if onlyAllowedValues {
					continue
				}

				for _, vn := range valueSpec.Names {
					if isAllowed(fileCommentMap, vn, pass.TypesInfo) {
						continue
					}

					message := fmt.Sprintf("%s is a global variable", vn.Name)
					pass.Report(analysis.Diagnostic{
						Pos:      vn.Pos(),
						Category: "global",
						Message:  message,
					})
				}
			}
		}
	}

	return nil, nil
}
