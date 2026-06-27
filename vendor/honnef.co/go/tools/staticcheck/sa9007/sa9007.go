package sa9007

import (
	"fmt"
	"os"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA9007",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "Deleting a directory that shouldn't be deleted",
		Text: `
It is virtually never correct to delete system directories such as
/tmp or the user's home directory. However, it can be fairly easy to
do by mistake, for example by mistakenly using \'os.TempDir\' instead
of \'ioutil.TempDir\', or by forgetting to add a suffix to the result
of \'os.UserHomeDir\'.

Writing

    d := os.TempDir()
    defer os.RemoveAll(d)

in your unit tests will have a devastating effect on the stability of your system.

This check flags attempts at deleting the following directories:

- os.TempDir
- os.UserCacheDir
- os.UserConfigDir
- os.UserHomeDir
`,
		Since:    "2022.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				call, ok := instr.(ir.CallInstruction)
				if !ok {
					continue
				}
				if !irutil.IsCallTo(call.Common(), "os.RemoveAll") {
					continue
				}

				kind := ""
				ex := ""
				callName := ""
				arg := irutil.Flatten(call.Common().Args[0])
				switch arg := arg.(type) {
				case *ir.Call:
					callName = irutil.CallName(&arg.Call)
					if callName != "os.TempDir" {
						continue
					}
					kind = "temporary"
					ex = os.TempDir()
				case *ir.Extract:
					if arg.Index != 0 {
						continue
					}
					first, ok := arg.Tuple.(*ir.Call)
					if !ok {
						continue
					}
					callName = irutil.CallName(&first.Call)
					switch callName {
					case "os.UserCacheDir":
						kind = "cache"
						ex, _ = os.UserCacheDir()
					case "os.UserConfigDir":
						kind = "config"
						ex, _ = os.UserConfigDir()
					case "os.UserHomeDir":
						kind = "home"
						ex, _ = os.UserHomeDir()
					default:
						continue
					}
				default:
					continue
				}

				if ex == "" {
					report.Report(pass, call, fmt.Sprintf("this call to os.RemoveAll deletes the user's entire %s directory, not a subdirectory therein", kind),
						report.Related(arg, fmt.Sprintf("this call to %s returns the user's %s directory", callName, kind)))
				} else {
					report.Report(pass, call, fmt.Sprintf("this call to os.RemoveAll deletes the user's entire %s directory, not a subdirectory therein", kind),
						report.Related(arg, fmt.Sprintf("this call to %s returns the user's %s directory, for example %s", callName, kind, ex)))
				}
			}
		}
	}
	return nil, nil
}
