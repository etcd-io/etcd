package opaque

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"github.com/uudashr/iface/internal/directive"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the analysis pass for detecting opaque interface returns.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	r := runner{}

	analyzer := &analysis.Analyzer{
		Name:     "opaque",
		Doc:      "Detects functions that return an interface type, but only ever return a single concrete implementation.",
		URL:      "https://pkg.go.dev/github.com/uudashr/iface/opaque",
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

	// Find function declarations that return an interface

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		funcDecl := n.(*ast.FuncDecl)

		if funcDecl.Recv != nil {
			// skip methods
			return
		}

		if funcDecl.Body == nil {
			// skip functions without body
			return
		}

		if funcDecl.Type.Results == nil {
			// skip functions without return values
			return
		}

		if r.debug {
			fmt.Printf("Function declaration %s\n", funcDecl.Name.Name)
			fmt.Printf(" Results len=%d\n", len(funcDecl.Type.Results.List))
		}

		dir := directive.ParseIgnore(funcDecl.Doc)
		if dir != nil && dir.ShouldIgnore(pass.Analyzer.Name) {
			// skip ignored function
			return
		}

		// Pre-check, only function that has interface return type will be processed
		var hasInterfaceReturnType bool

		var outCount int

		for i, result := range funcDecl.Type.Results.List {
			outInc := 1
			if namesLen := len(result.Names); namesLen > 0 {
				outInc = namesLen
			}

			outCount += outInc

			resType := result.Type
			typ := pass.TypesInfo.TypeOf(resType)

			if r.debug {
				fmt.Printf("  [%d] len=%d %v %v %v | %v %v interface=%t\n", i, len(result.Names), result.Names, resType, reflect.TypeOf(resType), typ, reflect.TypeOf(typ), types.IsInterface(typ))
			}

			if types.IsInterface(typ) && !hasInterfaceReturnType {
				hasInterfaceReturnType = true
			}
		}

		if r.debug {
			fmt.Printf("  hasInterface=%t outCount=%d\n", hasInterfaceReturnType, outCount)
		}

		if !hasInterfaceReturnType {
			// skip, since it has no interface return type
			return
		}

		// Collect types on every return statement
		retStmtTypes := make([]map[types.Type]struct{}, outCount)
		for i := range retStmtTypes {
			retStmtTypes[i] = make(map[types.Type]struct{})
		}

		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				// ignore nested functions
				return false
			case *ast.ReturnStmt:
				if r.debug {
					fmt.Printf("  Return statements %v len=%d\n", n.Results, len(n.Results))
				}

				for i, result := range n.Results {
					if r.debug {
						fmt.Printf("   [%d] %v %v\n", i, result, reflect.TypeOf(result))
					}

					switch res := result.(type) {
					case *ast.CallExpr:
						if r.debug {
							fmt.Printf("       CallExpr Fun: %v %v\n", res.Fun, reflect.TypeOf(res.Fun))
						}

						typ := pass.TypesInfo.TypeOf(res)
						switch typ := typ.(type) {
						case *types.Tuple:
							for i := range typ.Len() {
								v := typ.At(i)
								vTyp := v.Type()
								retStmtTypes[i][vTyp] = struct{}{}

								if r.debug {
									fmt.Printf("          Tuple [%d]: %v %v | %v %v interface=%t\n", i, v, reflect.TypeOf(v), vTyp, reflect.TypeOf(vTyp), types.IsInterface(vTyp))
								}
							}
						default:
							retStmtTypes[i][typ] = struct{}{}
						}

					case *ast.Ident:
						if r.debug {
							fmt.Printf("       Ident: %v %v\n", res, reflect.TypeOf(res))
						}

						typ := pass.TypesInfo.TypeOf(res)
						isNilStmt := isUntypedNil(typ)

						if r.debug {
							fmt.Printf("        Ident type: %v %v interface=%t, untypedNil=%t\n", typ, reflect.TypeOf(typ), types.IsInterface(typ), isNilStmt)
						}

						if !isNilStmt {
							retStmtTypes[i][typ] = struct{}{}
						}
					case *ast.UnaryExpr:
						if r.debug {
							fmt.Printf("       UnaryExpr X: %v \n", res.X)
						}

						typ := pass.TypesInfo.TypeOf(res)

						if r.debug {
							fmt.Printf("        UnaryExpr type: %v %v interface=%t\n", typ, reflect.TypeOf(typ), types.IsInterface(typ))
						}

						retStmtTypes[i][typ] = struct{}{}
					default:
						if r.debug {
							fmt.Printf("       Unknown: %v %v\n", res, reflect.TypeOf(res))
						}

						typ := pass.TypesInfo.TypeOf(res)
						retStmtTypes[i][typ] = struct{}{}
					}
				}

				return false
			default:
				return true
			}
		})

		// Compare func return types with the return statement types
		var nextIdx int

		for _, result := range funcDecl.Type.Results.List {
			resType := result.Type
			typ := pass.TypesInfo.TypeOf(resType)

			consumeCount := 1
			if len(result.Names) > 0 {
				consumeCount = len(result.Names)
			}

			currentIdx := nextIdx
			nextIdx += consumeCount

			// Check return type
			if !types.IsInterface(typ) {
				// it is a concrete type
				continue
			}

			if typ.String() == "error" {
				// very common case to have return type error
				continue
			}

			if !fromSamePackage(pass, typ) {
				// ignore interface from other package
				continue
			}

			// Check statement type
			stmtTyps := retStmtTypes[currentIdx]

			stmtTypsSize := len(stmtTyps)
			if stmtTypsSize > 1 {
				// it has multiple implementation
				continue
			}

			if stmtTypsSize == 0 {
				// function use named return value, while return statement is empty
				continue
			}

			var stmtTyp types.Type
			for t := range stmtTyps {
				// expect only one, we don't have to break it
				stmtTyp = t
			}

			if types.IsInterface(stmtTyp) {
				// not concrete type, skip
				continue
			}

			if r.debug {
				fmt.Printf("stmtType: %v %v | %v %v\n", stmtTyp, reflect.TypeOf(stmtTyp), stmtTyp.Underlying(), reflect.TypeOf(stmtTyp.Underlying()))
			}

			switch stmtTyp := stmtTyp.(type) {
			case *types.Basic:
				if stmtTyp.Kind() == types.UntypedNil {
					// ignore nil
					continue
				}
			case *types.Named:
				if _, ok := stmtTyp.Underlying().(*types.Signature); ok {
					// skip function type
					continue
				}
			}

			retTypeName := typ.String()
			if fromSamePackage(pass, typ) {
				retTypeName = removePkgPrefix(retTypeName)
			}

			stmtTypName := stmtTyp.String()
			if fromSamePackage(pass, stmtTyp) {
				stmtTypName = removePkgPrefix(stmtTypName)
			}

			msg := fmt.Sprintf("'%s' function return '%s' interface at the %s result, abstract a single concrete implementation of '%s'",
				funcDecl.Name.Name,
				retTypeName,
				positionStr(currentIdx),
				stmtTypName)

			pass.Report(analysis.Diagnostic{
				Pos:     result.Pos(),
				Message: msg,
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: "Replace the interface return type with the concrete type",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     result.Pos(),
								End:     result.End(),
								NewText: []byte(stmtTypName),
							},
						},
					},
				},
			})
		}
	})

	return nil, nil
}

func isUntypedNil(typ types.Type) bool {
	if b, ok := typ.(*types.Basic); ok {
		return b.Kind() == types.UntypedNil
	}

	return false
}

func positionStr(idx int) string {
	switch idx {
	case 0:
		return "1st"
	case 1:
		return "2nd"
	case 2:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", idx+1)
	}
}

func fromSamePackage(pass *analysis.Pass, typ types.Type) bool {
	switch typ := typ.(type) {
	case *types.Named:
		currentPkg := pass.Pkg
		ifacePkg := typ.Obj().Pkg()

		return currentPkg == ifacePkg
	case *types.Pointer:
		return fromSamePackage(pass, typ.Elem())
	default:
		return false
	}
}

func removePkgPrefix(typeStr string) string {
	if typeStr[0] == '*' {
		return "*" + removePkgPrefix(typeStr[1:])
	}

	if lastDot := strings.LastIndex(typeStr, "."); lastDot != -1 {
		return typeStr[lastDot+1:]
	}

	return typeStr
}
