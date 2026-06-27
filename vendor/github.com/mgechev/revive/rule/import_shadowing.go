package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/mgechev/revive/lint"
)

// ImportShadowingRule spots identifiers that shadow an import.
type ImportShadowingRule struct{}

// Apply applies the rule to given file.
func (r *ImportShadowingRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	importNames := map[string]struct{}{}
	for _, imp := range file.AST.Imports {
		importNames[r.getName(imp)] = struct{}{}
	}

	fileAst := file.AST
	walker := importShadowing{
		packageNameIdent: fileAst.Name,
		importNames:      importNames,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
		alreadySeen: map[*ast.Object]struct{}{}, //nolint:staticcheck // TODO: ast.Object is deprecated
		skipIdents:  map[*ast.Ident]struct{}{},
	}

	ast.Walk(walker, fileAst)

	return failures
}

// Name returns the rule name.
func (*ImportShadowingRule) Name() string {
	return "import-shadowing"
}

func (r *ImportShadowingRule) getName(imp *ast.ImportSpec) string {
	const pathSep = "/"
	const strDelim = `"`
	if imp.Name != nil {
		return imp.Name.Name
	}

	path := strings.Trim(imp.Path.Value, strDelim)
	parts := strings.Split(path, pathSep)

	lastSegment := parts[len(parts)-1]
	if r.isVersion(lastSegment) && len(parts) >= 2 {
		// Use the previous segment when current is a version (v1, v2, etc.).
		return parts[len(parts)-2]
	}

	return lastSegment
}

type importShadowing struct {
	packageNameIdent *ast.Ident
	importNames      map[string]struct{}
	onFailure        func(lint.Failure)
	alreadySeen      map[*ast.Object]struct{} //nolint:staticcheck // TODO: ast.Object is deprecated
	skipIdents       map[*ast.Ident]struct{}
}

// Visit visits AST nodes and checks if id nodes (ast.Ident) shadow an import name.
func (w importShadowing) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.AssignStmt:
		if n.Tok == token.DEFINE {
			return w // analyze variable declarations of the form id := expr
		}

		return nil // skip assigns of the form id = expr (not an id declaration)
	case *ast.CallExpr, // skip call expressions (not an id declaration)
		*ast.ImportSpec,   // skip import section subtree because we already have the list of imports
		*ast.KeyValueExpr, // skip analysis of key-val expressions ({key:value}): ids of such expressions, even the same of an import name, do not shadow the import name
		*ast.ReturnStmt,   // skip skipping analysis of returns, ids in expression were already analyzed
		*ast.SelectorExpr, // skip analysis of selector expressions (anId.otherId): because if anId shadows an import name, it was already detected, and otherId does not shadows the import name
		*ast.StructType:   // skip analysis of struct type because struct fields can not shadow an import name
		return nil
	case *ast.FuncDecl:
		if n.Recv != nil {
			w.skipIdents[n.Name] = struct{}{}
		}
	case *ast.Ident:
		if n == w.packageNameIdent {
			return nil // skip the ident corresponding to the package name of this file
		}

		id := n.Name
		if id == "_" {
			return w // skip _ id
		}

		_, isImportName := w.importNames[id]
		_, alreadySeen := w.alreadySeen[n.Obj]
		_, skipIdent := w.skipIdents[n]
		if isImportName && !alreadySeen && !skipIdent {
			w.onFailure(lint.Failure{
				Confidence: 1,
				Node:       n,
				Category:   lint.FailureCategoryNaming,
				Failure:    fmt.Sprintf("The name '%s' shadows an import name", id),
			})

			w.alreadySeen[n.Obj] = struct{}{}
		}
	}

	return w
}

func (*ImportShadowingRule) isVersion(name string) bool {
	if len(name) < 2 || (name[0] != 'v' && name[0] != 'V') {
		return false
	}

	for i := 1; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}

	return true
}
