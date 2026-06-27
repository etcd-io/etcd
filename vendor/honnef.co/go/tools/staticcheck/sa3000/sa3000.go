package sa3000

import (
	"go/ast"
	"go/types"
	"go/version"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA3000",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `\'TestMain\' doesn't call \'os.Exit\', hiding test failures`,
		Text: `Test executables (and in turn \"go test\") exit with a non-zero status
code if any tests failed. When specifying your own \'TestMain\' function,
it is your responsibility to arrange for this, by calling \'os.Exit\' with
the correct code. The correct code is returned by \'(*testing.M).Run\', so
the usual way of implementing \'TestMain\' is to end it with
\'os.Exit(m.Run())\'.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	var (
		fnmain    ast.Node
		callsExit bool
		callsRun  bool
		arg       types.Object
	)
	fn := func(node ast.Node, push bool) bool {
		if !push {
			if fnmain != nil && node == fnmain {
				if !callsExit && callsRun {
					report.Report(pass, fnmain, "TestMain should call os.Exit to set exit code")
				}
				fnmain = nil
				callsExit = false
				callsRun = false
				arg = nil
			}
			return true
		}

		switch node := node.(type) {
		case *ast.FuncDecl:
			if fnmain != nil {
				return true
			}
			if !isTestMain(pass, node) {
				return false
			}
			if version.Compare(code.StdlibVersion(pass, node), "go1.15") >= 0 {
				// Beginning with Go 1.15, the test framework will call
				// os.Exit for us.
				return false
			}
			fnmain = node
			arg = pass.TypesInfo.ObjectOf(node.Type.Params.List[0].Names[0])
			return true
		case *ast.CallExpr:
			if code.IsCallTo(pass, node, "os.Exit") {
				callsExit = true
				return false
			}
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if arg != pass.TypesInfo.ObjectOf(ident) {
				return true
			}
			if sel.Sel.Name == "Run" {
				callsRun = true
				return false
			}
			return true
		default:
			lint.ExhaustiveTypeSwitch(node)
			return true
		}
	}
	pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Nodes([]ast.Node{(*ast.FuncDecl)(nil), (*ast.CallExpr)(nil)}, fn)
	return nil, nil
}

func isTestMain(pass *analysis.Pass, decl *ast.FuncDecl) bool {
	if decl.Name.Name != "TestMain" {
		return false
	}
	if len(decl.Type.Params.List) != 1 {
		return false
	}
	arg := decl.Type.Params.List[0]
	if len(arg.Names) != 1 {
		return false
	}
	return code.IsOfPointerToTypeWithName(pass, arg.Type, "testing.M")
}
