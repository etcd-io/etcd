package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name:     "nosprintfhostport",
	Doc:      "Checks for misuse of Sprintf to construct a host with port in a URL.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspector.Preorder(nodeFilter, func(node ast.Node) {
		callExpr := node.(*ast.CallExpr)
		if p, f, ok := getCallExprFunction(callExpr); ok && p == "fmt" && f == "Sprintf" {
			if err := checkForHostPortConstruction(callExpr); err != nil {
				pass.Reportf(node.Pos(), "%s", err.Error())
			}
		}
	})

	return nil, nil
}

// getCallExprFunction returns the package and function name from a callExpr, if any.
func getCallExprFunction(callExpr *ast.CallExpr) (pkg string, fn string, result bool) {
	selector, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	gopkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return gopkg.Name, selector.Sel.Name, true
}

// getStringLiteral returns the value at a position if it's a string literal.
func getStringLiteral(args []ast.Expr, pos int) (string, bool) {
	if len(args) < pos+1 {
		return "", false
	}

	// Let's see if our format string is a string literal.
	fsRaw, ok := args[pos].(*ast.BasicLit)
	if !ok {
		return "", false
	}
	if fsRaw.Kind == token.STRING && len(fsRaw.Value) >= 2 {
		return fsRaw.Value[1 : len(fsRaw.Value)-1], true
	} else {
		return "", false
	}
}

// checkForHostPortConstruction checks to see if a sprintf call looks like a URI with a port,
// essentially scheme://%s:<something else>, or scheme://user:pass@%s:<something else>.
//
// Matching requirements:
//   - Scheme as per RFC3986 is ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )
//   - A format string substitution in the host portion, preceded by an optional username/password@
//   - A colon indicating a port will be specified
func checkForHostPortConstruction(sprintf *ast.CallExpr) error {
	fs, ok := getStringLiteral(sprintf.Args, 0)
	if !ok {
		return nil
	}

	regexes := []*regexp.Regexp{
		regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+-.]*://%s:[^@]*$`),    // URL without basic auth user
		regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+-.]*://[^/]*@%s:.*$`), // URL with basic auth
	}

	for idx, re := range regexes {
		if !re.MatchString(fs) {
			continue
		}

		// Match without basic auth and only handle cases where the hostname and optionally the port are specified.
		if idx == 0 && len(sprintf.Args) <= 3 {
			arg, ok := getStringLiteral(sprintf.Args, 1)
			if ok && !strings.Contains(arg, ":") {
				continue
			}
		}

		return fmt.Errorf("host:port in url should be constructed with net.JoinHostPort and not directly with fmt.Sprintf")
	}

	return nil
}
