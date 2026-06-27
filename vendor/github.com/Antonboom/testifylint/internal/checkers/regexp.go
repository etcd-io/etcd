package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Regexp detects situations like
//
//	assert.Regexp(t, regexp.MustCompile(`\[.*\] DEBUG \(.*TestNew.*\): message`), out)
//	assert.NotRegexp(t, regexp.MustCompile(`\[.*\] TRACE message`), out)
//
// and requires
//
//	assert.Regexp(t, `\[.*\] DEBUG \(.*TestNew.*\): message`, out)
//	assert.NotRegexp(t, `\[.*\] TRACE message`, out)
type Regexp struct{}

// NewRegexp constructs Regexp checker.
func NewRegexp() Regexp     { return Regexp{} }
func (Regexp) Name() string { return "regexp" }

func (checker Regexp) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	switch call.Fn.NameFTrimmed {
	default:
		return nil
	case "Regexp", "NotRegexp":
	}

	if len(call.Args) < 1 {
		return nil
	}

	ce, ok := call.Args[0].(*ast.CallExpr)
	if !ok || len(ce.Args) != 1 {
		return nil
	}

	if isRegexpMustCompileCall(pass, ce) {
		return newRemoveMustCompileDiagnostic(pass, checker.Name(), call, ce, ce.Args[0])
	}
	return nil
}
