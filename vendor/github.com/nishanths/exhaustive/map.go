package exhaustive

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

// mapConfig is configuration for mapChecker.
type mapConfig struct {
	explicit       bool
	checkGenerated bool
	ignoreConstant *regexp.Regexp // can be nil
	ignoreType     *regexp.Regexp // can be nil
}

// mapChecker returns a node visitor that checks for exhaustiveness of
// map literals for the supplied pass, and reports diagnostics. The
// node visitor expects only *ast.CompositeLit nodes.
func mapChecker(pass *analysis.Pass, cfg mapConfig, generated boolCache, comments commentCache) nodeVisitor {
	return func(n ast.Node, push bool, stack []ast.Node) (bool, string) {
		if !push {
			return true, resultNotPush
		}

		file := stack[0].(*ast.File)

		if !cfg.checkGenerated && generated.get(file) {
			return false, resultGeneratedFile
		}

		lit := n.(*ast.CompositeLit)

		mapType, ok := pass.TypesInfo.Types[lit.Type].Type.(*types.Map)
		if !ok {
			namedType, ok2 := pass.TypesInfo.Types[lit.Type].Type.(*types.Named)
			if !ok2 {
				return true, resultNotMapLiteral
			}
			mapType, ok = namedType.Underlying().(*types.Map)
			if !ok {
				return true, resultNotMapLiteral
			}
		}

		if len(lit.Elts) == 0 {
			return false, resultEmptyMapLiteral
		}

		fileComments := comments.get(pass.Fset, file)
		var relatedComments []*ast.CommentGroup
		for i := range stack {
			// iterate over stack in the reverse order (from inner
			// node to outer node)
			node := stack[len(stack)-1-i]
			switch node.(type) {
			// need to check comments associated with following nodes,
			// because logic of ast package doesn't associate comment
			// with *ast.CompositeLit as required.
			case *ast.CompositeLit, // stack[len(stack)-1]
				*ast.ReturnStmt, // return ...
				*ast.IndexExpr,  // map[enum]...{...}[key]
				*ast.CallExpr,   // myfunc(map...)
				*ast.UnaryExpr,  // &map...
				*ast.AssignStmt, // variable assignment (without var keyword)
				*ast.DeclStmt,   // var declaration, parent of *ast.GenDecl
				*ast.GenDecl,    // var declaration, parent of *ast.ValueSpec
				*ast.ValueSpec:  // var declaration
				relatedComments = append(relatedComments, fileComments[node]...)
				continue
			default:
				// stop iteration on the first inappropriate node
				break
			}
		}

		if !cfg.explicit && hasCommentPrefix(relatedComments, ignoreComment) {
			// Skip checking of this map literal due to ignore
			// comment. Still return true because there may be nested
			// map literals that are not to be ignored.
			return true, resultIgnoreComment
		}
		if cfg.explicit && !hasCommentPrefix(relatedComments, enforceComment) {
			return true, resultNoEnforceComment
		}

		es, ok := composingEnumTypes(pass, mapType.Key())
		if !ok || len(es) == 0 {
			return true, resultEnumTypes
		}

		var checkl checklist
		checkl.ignoreConstant(cfg.ignoreConstant)
		checkl.ignoreType(cfg.ignoreType)

		for _, e := range es {
			checkl.add(e.typ, e.members, pass.Pkg == e.typ.Pkg())
		}

		analyzeMapLiteral(lit, pass.TypesInfo, checkl.found)
		if len(checkl.remaining()) == 0 {
			return true, resultEnumMembersAccounted
		}
		pass.Report(makeMapDiagnostic(lit, dedupEnumTypes(toEnumTypes(es)), checkl.remaining()))
		return true, resultReportedDiagnostic
	}
}

func analyzeMapLiteral(lit *ast.CompositeLit, info *types.Info, each func(constantValue)) {
	for _, e := range lit.Elts {
		expr, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if val, ok := exprConstVal(expr.Key, info); ok {
			each(val)
		}
	}
}

func makeMapDiagnostic(lit *ast.CompositeLit, enumTypes []enumType, missing map[member]struct{}) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos: lit.Pos(),
		End: lit.End(),
		Message: fmt.Sprintf(
			"missing keys in map of key type %s: %s",
			diagnosticEnumTypes(enumTypes),
			diagnosticGroups(groupify(missing, enumTypes)),
		),
	}
}
