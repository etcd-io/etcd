package s1009

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1009",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Omit redundant nil check on slices, maps, and channels`,
		Text: `The \'len\' function is defined for all slices, maps, and
channels, even nil ones, which have a length of zero. It is not necessary to
check for nil before checking that their length is not zero.`,
		Before:  `if x != nil && len(x) != 0 {}`,
		After:   `if len(x) != 0 {}`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query = pattern.MustParse(`
	(BinaryExpr
		(BinaryExpr
			x
			lhsOp@(Or "==" "!=")
			nilly)
		outerOp@(Or "&&" "||")
		(BinaryExpr
			(CallExpr (Builtin "len") [x])
			rhsOp
			k))`)

// run checks for the following redundant nil-checks:
//
//	if x == nil || len(x) == 0 {}
//	if x == nil || len(x) < N {} (where N != 0)
//	if x == nil || len(x) <= N {}
//	if x != nil && len(x) != 0 {}
//	if x != nil && len(x) == N {} (where N != 0)
//	if x != nil && len(x) > N {}
//	if x != nil && len(x) >= N {} (where N != 0)
func run(pass *analysis.Pass) (any, error) {
	isConstZero := func(expr ast.Expr) (isConst bool, isZero bool) {
		_, ok := expr.(*ast.BasicLit)
		if ok {
			return true, code.IsIntegerLiteral(pass, expr, constant.MakeInt64(0))
		}
		id, ok := expr.(*ast.Ident)
		if !ok {
			return false, false
		}
		c, ok := pass.TypesInfo.ObjectOf(id).(*types.Const)
		if !ok {
			return false, false
		}
		return true, c.Val().Kind() == constant.Int && c.Val().String() == "0"
	}

	for node, m := range code.Matches(pass, query) {
		x := m.State["x"].(ast.Expr)
		outerOp := m.State["outerOp"].(token.Token)
		lhsOp := m.State["lhsOp"].(token.Token)
		rhsOp := m.State["rhsOp"].(token.Token)
		nilly := m.State["nilly"].(ast.Expr)
		k := m.State["k"].(ast.Expr)
		eqNil := outerOp == token.LOR

		if code.MayHaveSideEffects(pass, x, nil) {
			continue
		}

		if eqNil && lhsOp != token.EQL {
			continue
		}
		if !eqNil && lhsOp != token.NEQ {
			continue
		}
		if !code.IsNil(pass, nilly) {
			continue
		}
		isConst, isZero := isConstZero(k)
		if !isConst {
			continue
		}

		if eqNil {
			switch rhsOp {
			case token.EQL:
				// avoid false positive for "xx == nil || len(xx) == <non-zero>"
				if !isZero {
					continue
				}
			case token.LEQ:
				// ok
			case token.LSS:
				// avoid false positive for "xx == nil || len(xx) < 0"
				if isZero {
					continue
				}
			default:
				continue
			}
		} else {
			switch rhsOp {
			case token.EQL:
				// avoid false positive for "xx != nil && len(xx) == 0"
				if isZero {
					continue
				}
			case token.GEQ:
				// avoid false positive for "xx != nil && len(xx) >= 0"
				if isZero {
					continue
				}
			case token.NEQ:
				// avoid false positive for "xx != nil && len(xx) != <non-zero>"
				if !isZero {
					continue
				}
			case token.GTR:
				// ok
			default:
				continue
			}
		}

		// finally check that xx type is one of array, slice, map or chan
		// this is to prevent false positive in case if xx is a pointer to an array
		typ := pass.TypesInfo.TypeOf(x)
		var nilType string
		ok := typeutil.All(typ, func(term *types.Term) bool {
			switch term.Type().Underlying().(type) {
			case *types.Slice:
				nilType = "nil slices"
				return true
			case *types.Map:
				nilType = "nil maps"
				return true
			case *types.Chan:
				nilType = "nil channels"
				return true
			case *types.Pointer:
				return false
			case *types.TypeParam:
				return false
			default:
				lint.ExhaustiveTypeSwitch(term.Type().Underlying())
				return false
			}
		})
		if !ok {
			continue
		}

		report.Report(pass, node,
			fmt.Sprintf("should omit nil check; len() for %s is defined as zero", nilType),
			report.FilterGenerated())
	}

	return nil, nil
}
