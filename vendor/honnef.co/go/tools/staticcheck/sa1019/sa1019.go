package sa1019

import (
	"fmt"
	"go/ast"
	"go/types"
	"go/version"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/deprecated"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1019",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, deprecated.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Using a deprecated function, variable, constant or field`,
		Since:    "2017.1",
		Severity: lint.SeverityDeprecated,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func formatGoVersion(s string) string {
	return "Go " + strings.TrimPrefix(s, "go")
}

func run(pass *analysis.Pass) (any, error) {
	deprs := pass.ResultOf[deprecated.Analyzer].(deprecated.Result)

	// Selectors can appear outside of function literals, e.g. when
	// declaring package level variables.

	isStdlibPath := func(path string) bool {
		// Modules with no dot in the first path element are reserved for the standard library and tooling.
		// This is the best we can currently do.
		// Nobody tells us which import paths are part of the standard library.
		//
		// We check the entire path instead of just the first path element, because the standard library doesn't contain paths with any dots, anyway.

		return !strings.Contains(path, ".")
	}

	handleDeprecation := func(depr *deprecated.IsDeprecated, node ast.Node, deprecatedObjName string, pkgPath string, tfn types.Object) {
		std, ok := knowledge.StdlibDeprecations[deprecatedObjName]
		if !ok && isStdlibPath(pkgPath) {
			// Deprecated object in the standard library, but we don't know the details of the deprecation.
			// Don't flag it at all, to avoid flagging an object that was deprecated in 1.N when targeting 1.N-1.
			// See https://staticcheck.dev/issues/1108 for the background on this.
			return
		}
		if ok {
			// In the past, we made use of the AlternativeAvailableSince field. If a function was deprecated in Go
			// 1.6 and an alternative had been available in Go 1.0, then we'd recommend using the alternative even
			// if targeting Go 1.2. The idea was to suggest writing future-proof code by using already-existing
			// alternatives. This had a major flaw, however: the user would need to use at least Go 1.6 for
			// Staticcheck to know that the function had been deprecated. Thus, targeting Go 1.2 and using Go 1.2
			// would behave differently from targeting Go 1.2 and using Go 1.6. This is especially a problem if the
			// user tries to ignore the warning. Depending on the Go version in use, the ignore directive may or may
			// not match, causing a warning of its own.
			//
			// To avoid this issue, we no longer try to be smart. We now only compare the targeted version against
			// the version that deprecated an object.
			//
			// Unfortunately, this issue also applies to AlternativeAvailableSince == DeprecatedNeverUse. Even though it
			// is only applied to seriously flawed API, such as broken cryptography, users may wish to ignore those
			// warnings.
			//
			// See also https://staticcheck.dev/issues/1318.
			if version.Compare(code.StdlibVersion(pass, node), std.DeprecatedSince) == -1 {
				return
			}
		}

		if tfn != nil {
			if _, ok := deprs.Objects[tfn]; ok {
				// functions that are deprecated may use deprecated
				// symbols
				return
			}
		}

		if ok {
			switch std.AlternativeAvailableSince {
			case knowledge.DeprecatedNeverUse:
				report.Report(pass, node,
					fmt.Sprintf("%s has been deprecated since %s because it shouldn't be used: %s",
						report.Render(pass, node), formatGoVersion(std.DeprecatedSince), depr.Msg))
			case std.DeprecatedSince, knowledge.DeprecatedUseNoLonger:
				report.Report(pass, node,
					fmt.Sprintf("%s has been deprecated since %s: %s",
						report.Render(pass, node), formatGoVersion(std.DeprecatedSince), depr.Msg))
			default:
				report.Report(pass, node,
					fmt.Sprintf("%s has been deprecated since %s and an alternative has been available since %s: %s",
						report.Render(pass, node), formatGoVersion(std.DeprecatedSince), formatGoVersion(std.AlternativeAvailableSince), depr.Msg))
			}
		} else {
			report.Report(pass, node, fmt.Sprintf("%s is deprecated: %s", report.Render(pass, node), depr.Msg))
		}
	}

	var tfn types.Object
	stack := 0
	fn := func(node ast.Node, push bool) bool {
		if !push {
			stack--
			return false
		}
		stack++
		if stack == 1 {
			tfn = nil
		}
		if fn, ok := node.(*ast.FuncDecl); ok {
			tfn = pass.TypesInfo.ObjectOf(fn.Name)
		}

		// FIXME(dh): this misses dot-imported objects
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		obj := pass.TypesInfo.ObjectOf(sel.Sel)
		if obj_, ok := obj.(*types.Func); ok {
			obj = obj_.Origin()
		}
		if obj.Pkg() == nil {
			return true
		}

		if obj.Pkg() == pass.Pkg {
			// A package is allowed to use its own deprecated objects
			return true
		}

		// A package "foo" has two related packages "foo_test" and "foo.test", for external tests and the package main
		// generated by 'go test' respectively. "foo_test" can import and use "foo", "foo.test" imports and uses "foo"
		// and "foo_test".

		if strings.TrimSuffix(pass.Pkg.Path(), "_test") == obj.Pkg().Path() {
			// foo_test (the external tests of foo) can use objects from foo.
			return true
		}
		if strings.TrimSuffix(pass.Pkg.Path(), ".test") == obj.Pkg().Path() {
			// foo.test (the main package of foo's tests) can use objects from foo.
			return true
		}
		if strings.TrimSuffix(pass.Pkg.Path(), ".test") == strings.TrimSuffix(obj.Pkg().Path(), "_test") {
			// foo.test (the main package of foo's tests) can use objects from foo's external tests.
			return true
		}

		if depr, ok := deprs.Objects[obj]; ok {
			handleDeprecation(depr, sel, code.SelectorName(pass, sel), obj.Pkg().Path(), tfn)
		}
		return true
	}

	fn2 := func(node ast.Node) {
		spec := node.(*ast.ImportSpec)
		var imp *types.Package
		if spec.Name != nil {
			imp = pass.TypesInfo.ObjectOf(spec.Name).(*types.PkgName).Imported()
		} else {
			imp = pass.TypesInfo.Implicits[spec].(*types.PkgName).Imported()
		}

		p := spec.Path.Value
		path := p[1 : len(p)-1]
		if depr, ok := deprs.Packages[imp]; ok {
			if path == "github.com/golang/protobuf/proto" {
				gen, ok := code.Generator(pass, spec.Path.Pos())
				if ok && gen == generated.ProtocGenGo {
					return
				}
			}

			if strings.TrimSuffix(pass.Pkg.Path(), "_test") == path {
				// foo_test can import foo
				return
			}
			if strings.TrimSuffix(pass.Pkg.Path(), ".test") == path {
				// foo.test can import foo
				return
			}
			if strings.TrimSuffix(pass.Pkg.Path(), ".test") == strings.TrimSuffix(path, "_test") {
				// foo.test can import foo_test
				return
			}

			handleDeprecation(depr, spec.Path, path, path, nil)
		}
	}
	pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Nodes(nil, fn)
	code.Preorder(pass, fn2, (*ast.ImportSpec)(nil))
	return nil, nil
}
