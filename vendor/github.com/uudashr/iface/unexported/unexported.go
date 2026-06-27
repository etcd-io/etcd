package unexported

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/uudashr/iface/internal/directive"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer detects unexported interfaces used in exported functions or methods.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	r := runner{}

	analyzer := &analysis.Analyzer{
		Name:     "unexported",
		Doc:      "Detects interfaces which are not exported but are used as parameters or return values in exported functions or methods.",
		URL:      "https://pkg.go.dev/github.com/uudashr/iface/unexported",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      r.run,
	}

	analyzer.Flags.BoolVar(&r.debug, "nerd", false, "enable nerd mode")

	return analyzer
}

type runner struct {
	debug bool
}

func (r *runner) run(pass *analysis.Pass) (any, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		funcDecl := n.(*ast.FuncDecl)

		r.debugln("FuncDecl:", funcDecl.Name.Name)

		dir := directive.ParseIgnore(funcDecl.Doc)
		if dir != nil && dir.ShouldIgnore(pass.Analyzer.Name) {
			// skip ignored function
			r.debugln(" skip ignored")

			return
		}

		var recvName string

		if recv := funcDecl.Recv; recv != nil {
			recvType := recv.List[0].Type

			if r.debug {
				infoType := pass.TypesInfo.TypeOf(recvType)
				fmt.Printf(" recvType: %v infoType: %v reflectType: %T\n", recvType, infoType, recvType)
			}

			switch typ := recvType.(type) {
			case *ast.Ident:
				r.debugln("  recvIdent:", typ.Name)

				recvName = typ.Name
			case *ast.StarExpr:
				r.debugln("  recvStarExpr:", typ.X)

				if ident, ok := typ.X.(*ast.Ident); ok {
					r.debugln("   recvIdent:", ident.Name)
					recvName = ident.Name
				} else {
					r.debugln("   unhandled")
				}
			default:
				r.debugln("  unhandled")
			}
		}

		if !funcDecl.Name.IsExported() {
			// skip unexported functions
			r.debugln(" skip non-exported")

			return
		}

		r.debugln(" params:")

		params := funcDecl.Type.Params

		for _, param := range params.List {
			paramType := param.Type
			infoType := pass.TypesInfo.TypeOf(paramType)

			r.debugf("  paramType: %v infoType: %v reflectType: %T\n", paramType, infoType, paramType)

			if !types.IsInterface(infoType) {
				// skip non-interface
				r.debugln("   skip non-interface")

				continue
			}

			if infoType.String() == "error" {
				// skip error interface
				r.debugln("   skip error interface")

				continue
			}

			if infoType.String() == "any" {
				// skip any interface
				r.debugln("   skip any interface")

				continue
			}

			switch typ := paramType.(type) {
			case *ast.Ident:
				if !typ.IsExported() {
					r.debugln("   unexported")

					funcMethod := "function"
					funcMethodName := funcDecl.Name.Name

					if recvName != "" {
						funcMethod = "method"
						funcMethodName = recvName + "." + funcDecl.Name.Name
					}

					pass.Report(analysis.Diagnostic{
						Pos:     typ.Pos(),
						Message: fmt.Sprintf("unexported interface '%s' used as parameter in exported %s '%s'", typ.Name, funcMethod, funcMethodName),
					})
				}
			default:
				r.debugln("   unhandled")
			}
		}

		r.debugln(" results:")

		results := funcDecl.Type.Results
		if results == nil {
			r.debugln("  no results")

			return
		}

		for _, result := range results.List {
			resultType := result.Type
			infoType := pass.TypesInfo.TypeOf(resultType)

			r.debugf("  resultType: %v infoType: %v reflectType: %T\n", resultType, infoType, resultType)

			if !types.IsInterface(infoType) {
				r.debugln("   skip non-interface")

				continue
			}

			if infoType.String() == "error" {
				// skip error interface
				r.debugln("   skip error interface")

				continue
			}

			if infoType.String() == "any" {
				// skip any interface
				r.debugln("   skip any interface")

				continue
			}

			switch typ := resultType.(type) {
			case *ast.Ident:
				if !typ.IsExported() {
					r.debugln("   unexported")

					funcMethod := "function"
					funcMethodName := funcDecl.Name.Name

					if recvName != "" {
						funcMethod = "method"
						funcMethodName = recvName + "." + funcDecl.Name.Name
					}

					pass.Report(analysis.Diagnostic{
						Pos:     typ.Pos(),
						Message: fmt.Sprintf("unexported interface '%s' used as return value in exported %s '%s'", typ.Name, funcMethod, funcMethodName),
					})
				}
			default:
				r.debugln("   unhandled")
			}
		}
	})

	return nil, nil
}

func (r *runner) debugln(a ...any) {
	if r.debug {
		fmt.Println(a...)
	}
}

func (r *runner) debugf(format string, a ...any) {
	if r.debug {
		fmt.Printf(format, a...)
	}
}
