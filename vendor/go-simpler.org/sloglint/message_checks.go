package sloglint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

func staticMessage(pass *analysis.Pass, msg ast.Expr) {
	var isStatic func(msg ast.Expr) bool
	isStatic = func(msg ast.Expr) bool {
		switch msg := msg.(type) {
		case *ast.BasicLit: // e.g. slog.Info("msg")
			return msg.Kind == token.STRING
		case *ast.Ident: // e.g. slog.Info(constMsg)
			_, isConst := pass.TypesInfo.ObjectOf(msg).(*types.Const)
			return isConst
		case *ast.BinaryExpr: // e.g. slog.Info("x" + "y")
			if msg.Op != token.ADD {
				panic("unreachable") // Only "+" can be applied to strings.
			}
			return isStatic(msg.X) && isStatic(msg.Y)
		default:
			return false
		}
	}

	if !isStatic(msg) {
		pass.ReportRangef(msg, "message should be a string literal or a constant")
	}
}

func messageStyle(pass *analysis.Pass, msg ast.Expr, style string) {
	lit, ok := msg.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}

	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		panic("unreachable") // String literals are always quoted.
	}

	runes := []rune(strings.TrimSpace(s))
	if len(runes) < 2 {
		return
	}

	first, second := runes[0], runes[1]

	if !unicode.IsLetter(first) {
		return // e.g. "200 OK"
	}

	switch style {
	case messageStyleLowercased:
		if unicode.IsLower(first) {
			return
		}
		if unicode.IsPunct(second) {
			return // e.g. "U.S."
		}
		if unicode.IsUpper(second) {
			return // e.g. "HTTP"
		}
	case messageStyleCapitalized:
		if unicode.IsUpper(first) {
			return
		}
		if unicode.IsUpper(second) {
			return // e.g. "iPhone"
		}
	}

	pass.ReportRangef(msg, "message should be %s", style)
}
