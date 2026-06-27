package st1016

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1016",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:      `Use consistent method receiver names`,
		Since:      "2019.1",
		NonDefault: true,
		MergeIf:    lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg
	for _, m := range irpkg.Members {
		names := map[string]int{}

		var firstFn *types.Func
		if T, ok := m.Object().(*types.TypeName); ok && !T.IsAlias() {
			ms := typeutil.IntuitiveMethodSet(T.Type(), nil)
			for _, sel := range ms {
				fn := sel.Obj().(*types.Func)
				recv := fn.Type().(*types.Signature).Recv()
				if code.IsGenerated(pass, recv.Pos()) {
					// Don't concern ourselves with methods in generated code
					continue
				}
				if typeutil.Dereference(recv.Type()) != T.Type() {
					// skip embedded methods
					continue
				}
				if firstFn == nil {
					firstFn = fn
				}
				if recv.Name() != "" && recv.Name() != "_" {
					names[recv.Name()]++
				}
			}
		}

		if len(names) > 1 {
			var seen []string
			for name, count := range names {
				seen = append(seen, fmt.Sprintf("%dx %q", count, name))
			}
			sort.Strings(seen)

			report.Report(pass, firstFn, fmt.Sprintf("methods on the same type should have the same receiver name (seen %s)", strings.Join(seen, ", ")))
		}
	}
	return nil, nil
}
