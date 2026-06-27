package sa1005

import (
	"go/ast"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1005",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `Invalid first argument to \'exec.Command\'`,
		Text: `\'os/exec\' runs programs directly (using variants of the fork and exec
system calls on Unix systems). This shouldn't be confused with running
a command in a shell. The shell will allow for features such as input
redirection, pipes, and general scripting. The shell is also
responsible for splitting the user's input into a program name and its
arguments. For example, the equivalent to

    ls / /tmp

would be

    exec.Command("ls", "/", "/tmp")

If you want to run a command in a shell, consider using something like
the following – but be aware that not all systems, particularly
Windows, will have a \'/bin/sh\' program:

    exec.Command("/bin/sh", "-c", "ls | grep Awesome")`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query = pattern.MustParse(`(CallExpr (Symbol "os/exec.Command") arg1:_)`)

func run(pass *analysis.Pass) (any, error) {
	for _, m := range code.Matches(pass, query) {
		arg1 := m.State["arg1"].(ast.Expr)
		val, ok := code.ExprToString(pass, arg1)
		if !ok {
			continue
		}
		if !strings.Contains(val, " ") || strings.Contains(val, `\`) || strings.Contains(val, "/") {
			continue
		}
		report.Report(pass, arg1,
			"first argument to exec.Command looks like a shell command, but a program name or path are expected")
	}
	return nil, nil
}
