package sloglint

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

func noGlobalLogger(pass *analysis.Pass, call *ast.CallExpr, defaultOnly bool) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)

	switch fn.Name() {
	case "Log", "LogAttrs",
		"Debug", "Info", "Warn", "Error",
		"DebugContext", "InfoContext", "WarnContext", "ErrorContext",
		"With":
	default:
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	if ident.Name == "slog" {
		pass.ReportRangef(sel.X, "default logger should not be used")
		return
	}

	if defaultOnly {
		return
	}

	if obj := pass.TypesInfo.ObjectOf(ident); obj != nil && obj.Parent() == obj.Pkg().Scope() {
		pass.ReportRangef(sel.X, "global logger should not be used")
	}
}

func contextOnly(pass *analysis.Pass, call *ast.CallExpr, cursor inspector.Cursor, scopeOnly bool) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)

	switch fn.Name() {
	case "Debug", "Info", "Warn", "Error":
	default:
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	if !scopeOnly {
		// Don't suggest fixes here, we don't know whether there is a context in the scope.
		pass.ReportRangef(sel.Sel, "%sContext should be used instead", fn.Name())
		return
	}

	for cursor := range cursor.Enclosing(new(ast.FuncDecl), new(ast.FuncLit)) {
		var params []*ast.Field
		switch fn := cursor.Node().(type) {
		case *ast.FuncDecl:
			params = fn.Type.Params.List
		case *ast.FuncLit:
			params = fn.Type.Params.List
		}

		if len(params) == 0 {
			continue
		}

		for _, param := range params {
			if len(param.Names) == 0 {
				continue
			}

			var ctxArg string
			switch name := param.Names[0]; typeName(pass.TypesInfo, name) {
			case "context.Context":
				ctxArg = name.Name
			case "*net/http.Request":
				ctxArg = name.Name + ".Context()"
			default:
				continue
			}

			pass.Report(analysis.Diagnostic{
				Pos:     sel.Sel.Pos(),
				End:     sel.Sel.End(),
				Message: fmt.Sprintf("%sContext should be used instead", fn.Name()),
				SuggestedFixes: []analysis.SuggestedFix{{
					TextEdits: []analysis.TextEdit{{
						Pos:     sel.Sel.Pos(),
						End:     call.Lparen + 1,
						NewText: fmt.Appendf(nil, "%sContext(%s, ", fn.Name(), ctxArg),
					}},
				}},
			})
			return
		}
	}
}

func discardHandler(pass *analysis.Pass, call *ast.CallExpr) {
	if len(call.Args) == 0 {
		return
	}

	sel, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok {
		return
	}

	obj := pass.TypesInfo.ObjectOf(sel.Sel)
	if obj == nil {
		return
	}

	if obj.Pkg().Name() != "io" || obj.Name() != "Discard" {
		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.Pos(),
		Message: "use slog.DiscardHandler instead",
		SuggestedFixes: []analysis.SuggestedFix{{
			TextEdits: []analysis.TextEdit{{
				Pos:     call.Pos(),
				End:     call.End(),
				NewText: []byte("slog.DiscardHandler"),
			}},
		}},
	})
}
