package exhaustive

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// nodeVisitor is like the visitor function used by inspector.WithStack,
// except that it returns an additional value: a short description of
// the result of this node visit.
//
// The result value is typically useful in debugging or in unit tests to check
// that the nodeVisitor function took the expected code path.
type nodeVisitor func(n ast.Node, push bool, stack []ast.Node) (proceed bool, result string)

// toVisitor converts a nodeVisitor to a function suitable for use
// with inspector.WithStack.
func toVisitor(v nodeVisitor) func(ast.Node, bool, []ast.Node) bool {
	return func(node ast.Node, push bool, stack []ast.Node) bool {
		proceed, _ := v(node, push, stack)
		return proceed
	}
}

// Result values returned by node visitors.
const (
	resultEmptyMapLiteral = "empty map literal"
	resultNotMapLiteral   = "not map literal"
	resultKeyNilPkg       = "nil map key package"
	resultKeyNotEnum      = "not all map key type terms are known enum types"

	resultNoSwitchTag = "no switch tag"
	resultTagNotValue = "switch tag not value type"
	resultTagNilPkg   = "nil switch tag package"
	resultTagNotEnum  = "not all switch tag terms are known enum types"

	resultNotPush              = "not push"
	resultGeneratedFile        = "generated file"
	resultIgnoreComment        = "has ignore comment"
	resultNoEnforceComment     = "has no enforce comment"
	resultEnumMembersAccounted = "required enum members accounted for"
	resultDefaultCaseSuffices  = "default case satisfies exhaustiveness"
	resultMissingDefaultCase   = "missing required default case"
	resultReportedDiagnostic   = "reported diagnostic"
	resultEnumTypes            = "invalid or empty composing enum types"
)

// switchConfig is configuration for switchChecker.
type switchConfig struct {
	explicit                   bool
	defaultSignifiesExhaustive bool
	defaultCaseRequired        bool
	checkGenerated             bool
	ignoreConstant             *regexp.Regexp // can be nil
	ignoreType                 *regexp.Regexp // can be nil
}

// There are few possibilities, and often none, so we use a possibly-nil slice
func userDirectives(comments []*ast.CommentGroup) []string {
	var directives []string
	for _, c := range comments {
		for _, cc := range c.List {
			// The order matters here: we always want to check the longest first.
			for _, d := range []string{
				enforceDefaultCaseRequiredComment,
				ignoreDefaultCaseRequiredComment,
				enforceComment,
				ignoreComment,
			} {
				if strings.HasPrefix(cc.Text, d) {
					directives = append(directives, d)
					// The break here is important: once we associate a comment
					// with a particular (longest-possible) directive, we don't want
					// to map to another!
					break
				}
			}
		}
	}
	return directives
}

// Can be replaced with slices.Contains with go1.21
func directivesIncludes(directives []string, d string) bool {
	for _, ud := range directives {
		if ud == d {
			return true
		}
	}
	return false
}

// switchChecker returns a node visitor that checks exhaustiveness of
// enum switch statements for the supplied pass, and reports
// diagnostics. The node visitor expects only *ast.SwitchStmt nodes.
func switchChecker(pass *analysis.Pass, cfg switchConfig, generated boolCache, comments commentCache) nodeVisitor {
	return func(n ast.Node, push bool, stack []ast.Node) (bool, string) {
		if !push {
			// The proceed return value should not matter; it is ignored by
			// inspector package for pop calls.
			// Nevertheless, return true to be on the safe side for the future.
			return true, resultNotPush
		}

		file := stack[0].(*ast.File)

		if !cfg.checkGenerated && generated.get(file) {
			// Don't check this file.
			// Return false because the children nodes of node `n` don't have to be checked.
			return false, resultGeneratedFile
		}

		sw := n.(*ast.SwitchStmt)

		switchComments := comments.get(pass.Fset, file)[sw]
		uDirectives := userDirectives(switchComments)
		if !cfg.explicit && directivesIncludes(uDirectives, ignoreComment) {
			// Skip checking of this switch statement due to ignore
			// comment. Still return true because there may be nested
			// switch statements that are not to be ignored.
			return true, resultIgnoreComment
		}
		if cfg.explicit && !directivesIncludes(uDirectives, enforceComment) {
			// Skip checking of this switch statement due to missing
			// enforce comment.
			return true, resultNoEnforceComment
		}
		requireDefaultCase := cfg.defaultCaseRequired
		if directivesIncludes(uDirectives, ignoreDefaultCaseRequiredComment) {
			requireDefaultCase = false
		}
		if directivesIncludes(uDirectives, enforceDefaultCaseRequiredComment) {
			// We have "if" instead of "else if" here in case of conflicting ignore/enforce directives.
			// In that case, because this is second, we will default to enforcing.
			requireDefaultCase = true
		}

		if sw.Tag == nil {
			return true, resultNoSwitchTag // never possible for valid Go program?
		}

		t := pass.TypesInfo.Types[sw.Tag]
		if !t.IsValue() {
			return true, resultTagNotValue
		}

		es, ok := composingEnumTypes(pass, t.Type)
		if !ok || len(es) == 0 {
			return true, resultEnumTypes
		}

		var checkl checklist
		checkl.ignoreConstant(cfg.ignoreConstant)
		checkl.ignoreType(cfg.ignoreType)

		for _, e := range es {
			checkl.add(e.typ, e.members, pass.Pkg == e.typ.Pkg())
		}

		defaultCaseExists := analyzeSwitchClauses(sw, pass.TypesInfo, checkl.found)
		if !defaultCaseExists && requireDefaultCase {
			// Even if the switch explicitly enumerates all the
			// enum values, the user has still required all switches
			// to have a default case. We check this first to avoid
			// early-outs
			pass.Report(makeMissingDefaultDiagnostic(sw, dedupEnumTypes(toEnumTypes(es))))
			return true, resultMissingDefaultCase
		}
		if len(checkl.remaining()) == 0 {
			// All enum members accounted for.
			// Nothing to report.
			return true, resultEnumMembersAccounted
		}
		if defaultCaseExists && cfg.defaultSignifiesExhaustive {
			// Though enum members are not accounted for, the
			// existence of the default case signifies
			// exhaustiveness.  So don't report.
			return true, resultDefaultCaseSuffices
		}
		pass.Report(makeSwitchDiagnostic(sw, dedupEnumTypes(toEnumTypes(es)), checkl.remaining()))
		return true, resultReportedDiagnostic
	}
}

func isDefaultCase(c *ast.CaseClause) bool {
	return c.List == nil // see doc comment on List field
}

// analyzeSwitchClauses analyzes the clauses in the supplied switch
// statement. The info param typically is pass.TypesInfo. The each
// function is called for each enum member name found in the switch
// statement. The hasDefaultCase return value indicates whether the
// switch statement has a default clause.
func analyzeSwitchClauses(sw *ast.SwitchStmt, info *types.Info, each func(val constantValue)) (hasDefaultCase bool) {
	for _, stmt := range sw.Body.List {
		caseCl := stmt.(*ast.CaseClause)
		if isDefaultCase(caseCl) {
			hasDefaultCase = true
			continue
		}
		for _, expr := range caseCl.List {
			if val, ok := exprConstVal(expr, info); ok {
				each(val)
			}
		}
	}
	return hasDefaultCase
}

func makeSwitchDiagnostic(sw *ast.SwitchStmt, enumTypes []enumType, missing map[member]struct{}) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos: sw.Pos(),
		End: sw.End(),
		Message: fmt.Sprintf(
			"missing cases in switch of type %s: %s",
			diagnosticEnumTypes(enumTypes),
			diagnosticGroups(groupify(missing, enumTypes)),
		),
	}
}

func makeMissingDefaultDiagnostic(sw *ast.SwitchStmt, enumTypes []enumType) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos: sw.Pos(),
		End: sw.End(),
		Message: fmt.Sprintf(
			"missing default case in switch of type %s",
			diagnosticEnumTypes(enumTypes),
		),
	}
}
