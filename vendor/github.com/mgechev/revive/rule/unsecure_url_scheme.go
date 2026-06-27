package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/mgechev/revive/lint"
)

// UnsecureURLSchemeRule checks if a file contains string literals with unsecure URL schemes.
// For example: "http://" in place of "https://".
type UnsecureURLSchemeRule struct{}

// Apply applied the rule to the given file.
func (*UnsecureURLSchemeRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if file.IsTest() {
		return nil // skip test files
	}

	var failures []lint.Failure
	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := lintUnsecureURLSchemeRule{
		onFailure: onFailure,
	}

	ast.Walk(w, file.AST)
	return failures
}

// Name returns the rule name.
func (*UnsecureURLSchemeRule) Name() string {
	return "unsecure-url-scheme"
}

type lintUnsecureURLSchemeRule struct {
	onFailure func(lint.Failure)
}

const (
	schemeSeparator  = "://"
	schemeHTTP       = "http"
	schemeWS         = "ws"
	urlPrefixHTTP    = schemeHTTP + schemeSeparator
	urlPrefixWS      = schemeWS + schemeSeparator
	lenURLPrefixHTTP = len(urlPrefixHTTP)
	lenURLPrefixWS   = len(urlPrefixWS)
)

func (w lintUnsecureURLSchemeRule) Visit(node ast.Node) ast.Visitor {
	n, ok := node.(*ast.BasicLit)
	if !ok || n.Kind != token.STRING {
		return w // not a string literal
	}

	value, _ := strconv.Unquote(n.Value) // n.Value has one of the following forms: "..." or `...`

	var scheme string
	var lenURLPrefix int
	switch {
	case strings.HasPrefix(value, urlPrefixHTTP):
		scheme = schemeHTTP
		lenURLPrefix = lenURLPrefixHTTP
	case strings.HasPrefix(value, urlPrefixWS):
		scheme = schemeWS
		lenURLPrefix = lenURLPrefixWS
	default:
		return nil // not an URL or not an unsecure one
	}

	if len(value) <= lenURLPrefix {
		return nil // there is no host part in the string
	}

	if strings.Contains(value, "localhost") || strings.Contains(value, "127.0.0.1") || strings.Contains(value, "0.0.0.0") || strings.Contains(value, "//::") {
		return nil // do not fail on local URL
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Failure:    fmt.Sprintf("prefer secure protocol %s over %s in %s", scheme+"s", scheme, n.Value),
		Node:       n,
	})

	return nil
}
