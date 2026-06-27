package rule

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// GetReturnRule warns on getters that do not yield any result.
type GetReturnRule struct{}

// Apply applies the rule to given file.
func (*GetReturnRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	for _, decl := range file.AST.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if !isGetter(fd.Name.Name) {
			continue
		}

		if hasResults(fd.Type.Results) {
			continue
		}

		if isHTTPHandler(fd.Type.Params) {
			continue // the Get prefix in the function name refers to HTTP GET
		}

		failures = append(failures, lint.Failure{
			Confidence: 0.8,
			Node:       fd,
			Category:   lint.FailureCategoryLogic,
			Failure:    fmt.Sprintf("function '%s' seems to be a getter but it does not return any result", fd.Name.Name),
		})
	}

	return failures
}

// Name returns the rule name.
func (*GetReturnRule) Name() string {
	return "get-return"
}

const getterPrefix = "GET"

var lenGetterPrefix = len(getterPrefix)

func isGetter(name string) bool {
	nameHasGetterPrefix := strings.HasPrefix(strings.ToUpper(name), getterPrefix)
	if !nameHasGetterPrefix {
		return false
	}

	isJustGet := len(name) == lenGetterPrefix
	if isJustGet {
		return false
	}

	c := name[lenGetterPrefix]
	lowerCaseAfterGetterPrefix := c >= 'a' && c <= 'z'

	return !lowerCaseAfterGetterPrefix
}

func hasResults(rs *ast.FieldList) bool {
	return rs != nil && len(rs.List) > 0
}

// isHTTPHandler returns true if the given params match with the signature of an HTTP handler, false otherwise
// A params list is considered to be an HTTP handler if the first two parameters are
// http.ResponseWriter, *http.Request in that order.
func isHTTPHandler(params *ast.FieldList) bool {
	typeNames := astutils.GetTypeNames(params)
	if len(typeNames) < 2 {
		return false
	}

	return typeNames[0] == "http.ResponseWriter" && typeNames[1] == "*http.Request"
}
