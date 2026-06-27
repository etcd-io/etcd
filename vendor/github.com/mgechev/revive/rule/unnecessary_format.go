package rule

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UnnecessaryFormatRule spots calls to formatting functions without leveraging formatting directives.
type UnnecessaryFormatRule struct{}

// Apply applies the rule to given file.
func (*UnnecessaryFormatRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	fileAst := file.AST
	walker := lintUnnecessaryFormat{
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	ast.Walk(walker, fileAst)

	return failures
}

// Name returns the rule name.
func (*UnnecessaryFormatRule) Name() string {
	return "unnecessary-format"
}

type lintUnnecessaryFormat struct {
	onFailure func(lint.Failure)
}

type formattingSpec struct {
	formatArgPosition byte
	alternative       string
}

var formattingFuncs = map[string]formattingSpec{
	"fmt.Appendf": {1, `"fmt.Append"`},
	"fmt.Errorf":  {0, `"errors.New"`},
	"fmt.Fprintf": {1, `"fmt.Fprint"`},
	"fmt.Fscanf":  {1, `"fmt.Fscan" or "fmt.Fscanln"`},
	"fmt.Printf":  {0, `"fmt.Print" or "fmt.Println"`},
	"fmt.Scanf":   {0, `"fmt.Scan"`},
	"fmt.Sprintf": {0, `"fmt.Sprint" or just the string itself"`},
	"fmt.Sscanf":  {1, `"fmt.Sscan"`},
	// standard logging functions
	"log.Fatalf": {0, `"log.Fatal"`},
	"log.Panicf": {0, `"log.Panic"`},
	"log.Printf": {0, `"log.Print"`},
	// Variable logger possibly being the std logger
	// Will trigger a false positive if all the following holds:
	//   1. the variable is not the std logger
	//   2. the Fatalf/Panicf/Printf method expects a string as first argument
	//      but the string is not expected to be a format string
	//   3. the actual first argument is a string literal
	//   4. the actual first argument does not contain a %
	"logger.Fatalf": {0, `"logger.Fatal"`},
	"logger.Panicf": {0, `"logger.Panic"`},
	"logger.Printf": {0, `"logger.Print"`},
	// standard testing functions
	// Variable t possibly being a testing.T
	// False positive risk: see comment on logger
	"t.Errorf": {0, `"t.Error"`},
	"t.Fatalf": {0, `"t.Fatal"`},
	"t.Logf":   {0, `"t.Log"`},
	"t.Skipf":  {0, `"t.Skip"`},
	// Variable b possibly being a testing.B
	// False positive risk: see comment on logger
	"b.Errorf": {0, `"b.Error"`},
	"b.Fatalf": {0, `"b.Fatal"`},
	"b.Logf":   {0, `"b.Log"`},
	"b.Skipf":  {0, `"b.Skip"`},
	// Variable f possibly being a testing.F
	// False positive risk: see comment on logger
	"f.Errorf": {0, `"f.Error"`},
	"f.Fatalf": {0, `"f.Fatal"`},
	"f.Logf":   {0, `"f.Log"`},
	"f.Skipf":  {0, `"f.Skip"`},
	// standard trace functions
	"trace.Logf": {2, `"trace.Log"`},
}

func (w lintUnnecessaryFormat) Visit(n ast.Node) ast.Visitor {
	ce, ok := n.(*ast.CallExpr)
	if !ok || len(ce.Args) < 1 {
		return w
	}

	funcName := astutils.GoFmt(ce.Fun)
	spec, ok := formattingFuncs[funcName]
	if !ok {
		return w
	}

	pos := int(spec.formatArgPosition)
	if len(ce.Args) <= pos {
		return w // not enough params /!\
	}

	arg := ce.Args[pos]
	if !astutils.IsStringLiteral(arg) {
		return w
	}

	format := astutils.GoFmt(arg)

	if strings.Contains(format, `%`) {
		return w
	}

	failure := lint.Failure{
		Category:   lint.FailureCategoryOptimization,
		Node:       ce.Fun,
		Confidence: 0.8,
		Failure:    fmt.Sprintf("unnecessary use of formatting function %q, you can replace it with %s", funcName, spec.alternative),
	}

	w.onFailure(failure)

	return w
}
