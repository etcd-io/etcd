package s1017

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1017",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Replace manual trimming with \'strings.TrimPrefix\'`,
		Text: `Instead of using \'strings.HasPrefix\' and manual slicing, use the
\'strings.TrimPrefix\' function. If the string doesn't start with the
prefix, the original string will be returned. Using \'strings.TrimPrefix\'
reduces complexity, and avoids common bugs, such as off-by-one
mistakes.`,
		Before: `
if strings.HasPrefix(str, prefix) {
    str = str[len(prefix):]
}`,
		After:   `str = strings.TrimPrefix(str, prefix)`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	sameNonDynamic := func(node1, node2 ast.Node) bool {
		if reflect.TypeOf(node1) != reflect.TypeOf(node2) {
			return false
		}

		switch node1 := node1.(type) {
		case *ast.Ident:
			return pass.TypesInfo.ObjectOf(node1) == pass.TypesInfo.ObjectOf(node2.(*ast.Ident))
		case *ast.SelectorExpr, *ast.IndexExpr:
			return astutil.Equal(node1, node2)
		case *ast.BasicLit:
			return astutil.Equal(node1, node2)
		}
		return false
	}

	isLenOnIdent := func(fn ast.Expr, ident ast.Expr) bool {
		call, ok := fn.(*ast.CallExpr)
		if !ok {
			return false
		}
		if !code.IsCallTo(pass, call, "len") {
			return false
		}
		if len(call.Args) != 1 {
			return false
		}
		return sameNonDynamic(call.Args[knowledge.Arg("len.v")], ident)
	}

	seen := make(map[ast.Node]struct{})
	fn := func(node ast.Node) {
		var pkg string
		var fun string

		ifstmt := node.(*ast.IfStmt)
		if ifstmt.Init != nil {
			return
		}
		if ifstmt.Else != nil {
			seen[ifstmt.Else] = struct{}{}
			return
		}
		if _, ok := seen[ifstmt]; ok {
			return
		}
		if len(ifstmt.Body.List) != 1 {
			return
		}
		condCall, ok := ifstmt.Cond.(*ast.CallExpr)
		if !ok {
			return
		}

		condCallName := code.CallName(pass, condCall)
		switch condCallName {
		case "strings.HasPrefix":
			pkg = "strings"
			fun = "HasPrefix"
		case "strings.HasSuffix":
			pkg = "strings"
			fun = "HasSuffix"
		case "strings.Contains":
			pkg = "strings"
			fun = "Contains"
		case "bytes.HasPrefix":
			pkg = "bytes"
			fun = "HasPrefix"
		case "bytes.HasSuffix":
			pkg = "bytes"
			fun = "HasSuffix"
		case "bytes.Contains":
			pkg = "bytes"
			fun = "Contains"
		default:
			return
		}

		assign, ok := ifstmt.Body.List[0].(*ast.AssignStmt)
		if !ok {
			return
		}
		if assign.Tok != token.ASSIGN {
			return
		}
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return
		}
		if !sameNonDynamic(condCall.Args[0], assign.Lhs[0]) {
			return
		}

		switch rhs := assign.Rhs[0].(type) {
		case *ast.CallExpr:
			if len(rhs.Args) < 2 || !sameNonDynamic(condCall.Args[0], rhs.Args[0]) || !sameNonDynamic(condCall.Args[1], rhs.Args[1]) {
				return
			}

			rhsName := code.CallName(pass, rhs)
			if condCallName == "strings.HasPrefix" && rhsName == "strings.TrimPrefix" ||
				condCallName == "strings.HasSuffix" && rhsName == "strings.TrimSuffix" ||
				condCallName == "strings.Contains" && rhsName == "strings.Replace" ||
				condCallName == "bytes.HasPrefix" && rhsName == "bytes.TrimPrefix" ||
				condCallName == "bytes.HasSuffix" && rhsName == "bytes.TrimSuffix" ||
				condCallName == "bytes.Contains" && rhsName == "bytes.Replace" {
				report.Report(pass, ifstmt, fmt.Sprintf("should replace this if statement with an unconditional %s", rhsName), report.FilterGenerated())
			}
		case *ast.SliceExpr:
			slice := rhs
			if !ok {
				return
			}
			if slice.Slice3 {
				return
			}
			if !sameNonDynamic(slice.X, condCall.Args[0]) {
				return
			}

			validateOffset := func(off ast.Expr) bool {
				switch off := off.(type) {
				case *ast.CallExpr:
					return isLenOnIdent(off, condCall.Args[1])
				case *ast.BasicLit:
					if pkg != "strings" {
						return false
					}
					if _, ok := condCall.Args[1].(*ast.BasicLit); !ok {
						// Only allow manual slicing with an integer
						// literal if the second argument to HasPrefix
						// was a string literal.
						return false
					}
					s, ok1 := code.ExprToString(pass, condCall.Args[1])
					n, ok2 := code.ExprToInt(pass, off)
					if !ok1 || !ok2 || n != int64(len(s)) {
						return false
					}
					return true
				default:
					return false
				}
			}

			switch fun {
			case "HasPrefix":
				// TODO(dh) We could detect a High that is len(s), but another
				// rule will already flag that, anyway.
				if slice.High != nil {
					return
				}
				if !validateOffset(slice.Low) {
					return
				}
			case "HasSuffix":
				if slice.Low != nil {
					n, ok := code.ExprToInt(pass, slice.Low)
					if !ok || n != 0 {
						return
					}
				}
				switch index := slice.High.(type) {
				case *ast.BinaryExpr:
					if index.Op != token.SUB {
						return
					}
					if !isLenOnIdent(index.X, condCall.Args[0]) {
						return
					}
					if !validateOffset(index.Y) {
						return
					}
				default:
					return
				}
			default:
				return
			}

			var replacement string
			switch fun {
			case "HasPrefix":
				replacement = "TrimPrefix"
			case "HasSuffix":
				replacement = "TrimSuffix"
			}
			report.Report(pass, ifstmt, fmt.Sprintf("should replace this if statement with an unconditional %s.%s", pkg, replacement),
				report.ShortRange(),
				report.FilterGenerated())
		}
	}
	code.Preorder(pass, fn, (*ast.IfStmt)(nil))
	return nil, nil
}
