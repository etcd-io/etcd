package matcher

import (
	"go/ast"
	"go/constant"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
)

type BeNumericallyMatcher struct {
	op      token.Token
	value   value.Valuer
	argType Type
}

var compareOps = map[string]token.Token{
	`"=="`: token.EQL,
	`"<"`:  token.LSS,
	`">"`:  token.GTR,
	`"="`:  token.ASSIGN,
	`"!="`: token.NEQ,
	`"<="`: token.LEQ,
	`">="`: token.GEQ,
}

func getCompareOp(opExp ast.Expr) token.Token {
	basic, ok := opExp.(*ast.BasicLit)
	if !ok {
		return token.ILLEGAL
	}
	if basic.Kind != token.STRING {
		return token.ILLEGAL
	}

	if tk, ok := compareOps[basic.Value]; ok {
		return tk
	}

	return token.ILLEGAL
}

func newBeNumericallyMatcher(opExp, orig, clone ast.Expr, pass *analysis.Pass) Info {
	op := getCompareOp(opExp)
	if op == token.ILLEGAL {
		return &UnspecifiedMatcher{
			matcherName: beNumerically,
		}
	}

	val := value.GetValuer(orig, clone, pass)
	argType := BeNumericallyMatcherType

	if val.IsValueNumeric() {
		if v := val.GetValue().String(); v == "0" {
			switch op {
			case token.EQL:
				argType |= EqualZero

			case token.NEQ, token.GTR:
				argType |= GreaterThanZero
			}
		} else if v == "1" && op == token.GEQ {
			argType |= GreaterThanZero
		}
	}

	return &BeNumericallyMatcher{
		op:      op,
		value:   val,
		argType: argType,
	}
}

func (m BeNumericallyMatcher) Type() Type {
	return m.argType
}

func (BeNumericallyMatcher) MatcherName() string {
	return beNumerically
}

func (m BeNumericallyMatcher) GetValueExpr() ast.Expr {
	return m.value.GetValueExpr()
}

func (m BeNumericallyMatcher) GetValue() constant.Value {
	return m.value.GetValue()
}

func (m BeNumericallyMatcher) GetType() gotypes.Type {
	return m.value.GetType()
}

func (m BeNumericallyMatcher) GetOp() token.Token {
	return m.op
}

func (m BeNumericallyMatcher) IsValueZero() bool {
	return m.value.IsValueZero()
}

func (m BeNumericallyMatcher) IsValueInt() bool {
	return m.value.IsValueInt()
}

func (m BeNumericallyMatcher) IsValueNumeric() bool {
	return m.value.IsValueNumeric()
}

func (m BeNumericallyMatcher) IsError() bool {
	return m.value.IsError()
}

func (m BeNumericallyMatcher) IsFunc() bool {
	return m.value.IsFunc()
}

func (m BeNumericallyMatcher) IsInterface() bool {
	return m.value.IsInterface()
}

func (m BeNumericallyMatcher) IsPointer() bool {
	return m.value.IsPointer()
}
