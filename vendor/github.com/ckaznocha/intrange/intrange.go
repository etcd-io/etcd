package intrange

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var (
	Analyzer = &analysis.Analyzer{
		Name:     "intrange",
		Doc:      "intrange is a linter to find places where for loops could make use of an integer range.",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	errFailedAnalysis = errors.New("failed analysis")
)

const (
	msg                = "for loop can be changed to use an integer range (Go 1.22+)"
	msgLenRange        = "for loop can be changed to `%s := range %s`"
	msgLenRangeNoIdent = "for loop can be changed to `range %s`"
)

func run(pass *analysis.Pass) (any, error) {
	result, ok := pass.ResultOf[inspect.Analyzer]
	if !ok {
		return nil, fmt.Errorf(
			"%w: %s",
			errFailedAnalysis,
			inspect.Analyzer.Name,
		)
	}

	resultInspector, ok := result.(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf(
			"%w: %s",
			errFailedAnalysis,
			inspect.Analyzer.Name,
		)
	}

	resultInspector.Preorder([]ast.Node{(*ast.ForStmt)(nil), (*ast.RangeStmt)(nil)}, check(pass))

	return nil, nil
}

func check(pass *analysis.Pass) func(node ast.Node) {
	return func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.ForStmt:
			checkForStmt(pass, stmt)
		case *ast.RangeStmt:
			checkRangeStmt(pass, stmt)
		default:
			return
		}
	}
}

func checkForStmt(pass *analysis.Pass, forStmt *ast.ForStmt) {
	// Existing checks for other patterns
	if forStmt.Init == nil || forStmt.Cond == nil || forStmt.Post == nil {
		return
	}

	// i := 0;;
	init, ok := forStmt.Init.(*ast.AssignStmt)
	if !ok {
		return
	}

	initAssign := init.Tok == token.ASSIGN

	if len(init.Lhs) != 1 || len(init.Rhs) != 1 {
		return
	}

	initIdent, ok := init.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}

	if !compareNumberLit(init.Rhs[0], 0) {
		return
	}

	cond, ok := forStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return
	}

	var (
		operand               ast.Expr
		hasEquivalentOperator bool
	)

	switch cond.Op {
	case token.LSS, token.LEQ: // ;i < n; || ;i <= n;
		x, ok := cond.X.(*ast.Ident)
		if !ok {
			return
		}

		if x.Name != initIdent.Name {
			return
		}

		hasEquivalentOperator = cond.Op == token.LEQ
		operand = cond.Y
	case token.GTR, token.GEQ: // ;n > i; || ;n >= i;
		y, ok := cond.Y.(*ast.Ident)
		if !ok {
			return
		}

		if y.Name != initIdent.Name {
			return
		}

		hasEquivalentOperator = cond.Op == token.GEQ
		operand = cond.X
	default:
		return
	}

	switch post := forStmt.Post.(type) {
	case *ast.IncDecStmt: // ;;i++
		if post.Tok != token.INC {
			return
		}

		ident, ok := post.X.(*ast.Ident)
		if !ok {
			return
		}

		if ident.Name != initIdent.Name {
			return
		}
	case *ast.AssignStmt:
		switch post.Tok {
		case token.ADD_ASSIGN: // ;;i += 1
			if len(post.Lhs) != 1 {
				return
			}

			ident, ok := post.Lhs[0].(*ast.Ident)
			if !ok {
				return
			}

			if ident.Name != initIdent.Name {
				return
			}

			if len(post.Rhs) != 1 {
				return
			}

			if !compareNumberLit(post.Rhs[0], 1) {
				return
			}
		case token.ASSIGN: // ;;i = i + 1 && ;;i = 1 + i
			if len(post.Lhs) != 1 || len(post.Rhs) != 1 {
				return
			}

			ident, ok := post.Lhs[0].(*ast.Ident)
			if !ok {
				return
			}

			if ident.Name != initIdent.Name {
				return
			}

			bin, ok := post.Rhs[0].(*ast.BinaryExpr)
			if !ok {
				return
			}

			if bin.Op != token.ADD {
				return
			}

			switch x := bin.X.(type) {
			case *ast.Ident: // ;;i = i + 1
				if x.Name != initIdent.Name {
					return
				}

				if !compareNumberLit(bin.Y, 1) {
					return
				}
			case *ast.BasicLit: // ;;i = 1 + i
				if !compareNumberLit(x, 1) {
					return
				}

				ident, ok := bin.Y.(*ast.Ident)
				if !ok {
					return
				}

				if ident.Name != initIdent.Name {
					return
				}
			default:
				return
			}
		default:
			return
		}
	default:
		return
	}

	bc := &bodyChecker{
		initIdent: initIdent,
		nExpr:     findNExpr(operand),
	}

	ast.Inspect(forStmt.Body, bc.check)

	if bc.modified {
		return
	}

	if initAssign {
		pass.Report(analysis.Diagnostic{
			Pos:     forStmt.Pos(),
			Message: msg + "\nBecause the key is not part of the loop's scope, take care to consider side effects.",
		})

		return
	}

	operandIsNumberLit := isNumberLit(operand)

	if hasEquivalentOperator && !operandIsNumberLit {
		return
	}

	rangeX := operandToString(
		pass,
		initIdent,
		operand,
		hasEquivalentOperator && operandIsNumberLit,
	)

	var replacement string
	if bc.accessed {
		replacement = fmt.Sprintf("%s := range %s", initIdent.Name, rangeX)
	} else {
		replacement = fmt.Sprintf("range %s", rangeX)
	}

	if isFunctionOrMethodCall(operand) {
		pass.Report(analysis.Diagnostic{
			Pos:     forStmt.Pos(),
			Message: msg + "\nBecause the key is returned by a function or method, take care to consider side effects.",
		})

		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:     forStmt.Pos(),
		Message: msg,
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: fmt.Sprintf("Replace loop with `%s`", replacement),
				TextEdits: []analysis.TextEdit{
					{
						Pos:     forStmt.Init.Pos(),
						End:     forStmt.Post.End(),
						NewText: []byte(replacement),
					},
				},
			},
		},
	})
}

func checkRangeStmt(pass *analysis.Pass, rangeStmt *ast.RangeStmt) {
	if rangeStmt.Value != nil {
		return
	}

	startPos := rangeStmt.Range
	usesKey := rangeStmt.Key != nil
	identName := ""

	if usesKey {
		ident, ok := rangeStmt.Key.(*ast.Ident)
		if !ok {
			return
		}

		if ident.Name == "_" {
			usesKey = false
		}

		identName = ident.Name
		startPos = ident.Pos()
	}

	if rangeStmt.X == nil {
		return
	}

	x, ok := rangeStmt.X.(*ast.CallExpr)
	if !ok {
		return
	}

	if _, ok = x.Fun.(*ast.Ident); !ok {
		return
	}

	if !isLen(x) {
		return
	}

	arg, ok := x.Args[0].(*ast.Ident)
	if !ok {
		return
	}

	// make sure arg is a slice or array
	obj := pass.TypesInfo.ObjectOf(arg)
	if obj == nil {
		return
	}

	switch obj.Type().Underlying().(type) {
	case *types.Slice, *types.Array:
	default:
		return
	}

	if usesKey {
		pass.Report(analysis.Diagnostic{
			Pos:     startPos,
			End:     x.End(),
			Message: fmt.Sprintf(msgLenRange, identName, arg.Name),
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: fmt.Sprintf("Replace `len(%s)` with `%s`", arg.Name, arg.Name),
					TextEdits: []analysis.TextEdit{
						{
							Pos:     x.Pos(),
							End:     x.End(),
							NewText: []byte(arg.Name),
						},
					},
				},
			},
		})

		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:     startPos,
		End:     x.End(),
		Message: fmt.Sprintf(msgLenRangeNoIdent, arg.Name),
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: fmt.Sprintf("Replace `len(%s)` with `%s`", arg.Name, arg.Name),
				TextEdits: []analysis.TextEdit{
					{
						Pos:     startPos,
						End:     x.End(),
						NewText: []byte(fmt.Sprintf("range %s", arg.Name)),
					},
				},
			},
		},
	})
}

func findNExpr(expr ast.Expr) ast.Expr {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if isLen(e) {
			return findNExpr(e.Args[0])
		}

		return nil
	case *ast.BasicLit:
		return nil
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e
	case *ast.IndexExpr:
		return e
	default:
		return nil
	}
}

func recursiveOperandToString(
	expr ast.Expr,
	incrementInt bool,
) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		args := ""

		for i, v := range e.Args {
			if i > 0 {
				args += ", "
			}

			args += recursiveOperandToString(v, incrementInt && len(e.Args) == 1)
		}

		return recursiveOperandToString(e.Fun, false) + "(" + args + ")"
	case *ast.BasicLit:
		if incrementInt && e.Kind == token.INT {
			v, err := strconv.Atoi(e.Value)
			if err == nil {
				return strconv.Itoa(v + 1)
			}

			return e.Value
		}

		return e.Value
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return recursiveOperandToString(e.X, false) + "." + recursiveOperandToString(e.Sel, false)
	case *ast.IndexExpr:
		return recursiveOperandToString(e.X, false) + "[" + recursiveOperandToString(e.Index, false) + "]"
	case *ast.BinaryExpr:
		return recursiveOperandToString(e.X, false) + " " + e.Op.String() + " " + recursiveOperandToString(e.Y, false)
	case *ast.StarExpr:
		return "*" + recursiveOperandToString(e.X, false)
	default:
		return ""
	}
}

func identEqual(a, b ast.Expr) bool {
	if a == nil || b == nil {
		return false
	}

	switch aT := a.(type) {
	case *ast.Ident:
		identB, ok := b.(*ast.Ident)
		if !ok {
			return false
		}

		return aT.Name == identB.Name
	case *ast.SelectorExpr:
		selectorB, ok := b.(*ast.SelectorExpr)
		if !ok {
			return false
		}

		return identEqual(aT.Sel, selectorB.Sel) && identEqual(aT.X, selectorB.X)
	case *ast.IndexExpr:
		indexB, ok := b.(*ast.IndexExpr)
		if ok {
			return identEqual(aT.X, indexB.X) && identEqual(aT.Index, indexB.Index)
		}

		return identEqual(aT.X, b)
	case *ast.BasicLit:
		litB, ok := b.(*ast.BasicLit)
		if !ok {
			return false
		}

		return aT.Value == litB.Value
	default:
		return false
	}
}

type bodyChecker struct {
	initIdent *ast.Ident
	nExpr     ast.Expr
	modified  bool
	accessed  bool
}

func (b *bodyChecker) check(n ast.Node) bool {
	switch stmt := n.(type) {
	case *ast.AssignStmt:
		for _, lhs := range stmt.Lhs {
			if identEqual(lhs, b.initIdent) || identEqual(lhs, b.nExpr) {
				b.modified = true

				return false
			}
		}
	case *ast.IncDecStmt:
		if identEqual(stmt.X, b.initIdent) || identEqual(stmt.X, b.nExpr) {
			b.modified = true

			return false
		}
	case *ast.Ident:
		if identEqual(stmt, b.initIdent) {
			b.accessed = true
		}
	}

	return true
}

func isNumberLit(exp ast.Expr) bool {
	switch lit := exp.(type) {
	case *ast.BasicLit:
		if lit.Kind == token.INT {
			return true
		}

		return false
	case *ast.CallExpr:
		switch fun := lit.Fun.(type) {
		case *ast.Ident:
			if !isIntCast(fun) {
				return false
			}
		default:
			return false
		}

		if len(lit.Args) != 1 {
			return false
		}

		return isNumberLit(lit.Args[0])
	default:
		return false
	}
}

func compareNumberLit(exp ast.Expr, val int) bool {
	switch lit := exp.(type) {
	case *ast.BasicLit:
		if lit.Kind != token.INT {
			return false
		}

		n := strconv.Itoa(val)

		switch lit.Value {
		case n, "0x" + n, "0X" + n:
			return true
		default:
			return false
		}
	case *ast.CallExpr:
		switch fun := lit.Fun.(type) {
		case *ast.Ident:
			if !isIntCast(fun) {
				return false
			}
		default:
			return false
		}

		if len(lit.Args) != 1 {
			return false
		}

		return compareNumberLit(lit.Args[0], val)
	default:
		return false
	}
}

func operandToString(
	pass *analysis.Pass,
	i *ast.Ident,
	operand ast.Expr,
	increment bool,
) string {
	s := recursiveOperandToString(operand, increment)
	t := pass.TypesInfo.TypeOf(i)

	if t == types.Typ[types.Int] {
		if len(s) > 5 && s[:4] == "int(" && s[len(s)-1] == ')' {
			s = s[4 : len(s)-1]
		}

		return s
	}

	if len(s) > 2 && s[len(s)-1] == ')' {
		return s
	}

	if operandIdent, ok := operand.(*ast.Ident); ok {
		if operandType := pass.TypesInfo.TypeOf(operandIdent); operandType != nil &&
			operandType == t {
			return s
		}
	}

	return t.String() + "(" + s + ")"
}

func isFunctionOrMethodCall(expr ast.Expr) bool {
	e, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	fun, ok := e.Fun.(*ast.Ident)
	if !ok {
		return true
	}

	if isLen(e) || isIntCast(fun) {
		return false
	}

	return true
}

func isIntCast(ident *ast.Ident) bool {
	switch ident.Name {
	case
		"int",
		"int8",
		"int16",
		"int32",
		"int64",
		"uint",
		"uint8",
		"uint16",
		"uint32",
		"uint64":
		return true
	default:
		return false
	}
}

func isLen(exp *ast.CallExpr) bool {
	fun, ok := exp.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	return fun.Name == "len" && len(exp.Args) == 1
}
