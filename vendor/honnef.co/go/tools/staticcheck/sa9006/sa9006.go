package sa9006

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA9006",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `Dubious bit shifting of a fixed size integer value`,
		Text: `Bit shifting a value past its size will always clear the value.

For instance:

    v := int8(42)
    v >>= 8

will always result in 0.

This check flags bit shifting operations on fixed size integer values only.
That is, int, uint and uintptr are never flagged to avoid potential false
positives in somewhat exotic but valid bit twiddling tricks:

    // Clear any value above 32 bits if integers are more than 32 bits.
    func f(i int) int {
        v := i >> 32
        v = v << 32
        return i-v
    }`,
		Since:    "2020.2",
		Severity: lint.SeverityWarning,
		// Technically this should be MergeIfAll, because the type of
		// v might be different for different build tags. Practically,
		// don't write code that depends on that.
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer
var (
	checkFixedLengthTypeShiftQ = pattern.MustParse(`
		(Or
			(AssignStmt _ (Or ">>=" "<<=") _)
			(BinaryExpr _ (Or ">>" "<<") _))
	`)
)

func run(pass *analysis.Pass) (any, error) {
	isDubiousShift := func(x, y ast.Expr) (int64, int64, bool) {
		typ, ok := pass.TypesInfo.TypeOf(x).Underlying().(*types.Basic)
		if !ok {
			return 0, 0, false
		}
		switch typ.Kind() {
		case types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			// We're only interested in fixed–size types.
		default:
			return 0, 0, false
		}

		const bitsInByte = 8
		typeBits := pass.TypesSizes.Sizeof(typ) * bitsInByte

		shiftLength, ok := code.ExprToInt(pass, y)
		if !ok {
			return 0, 0, false
		}

		return typeBits, shiftLength, shiftLength >= typeBits
	}

	for node := range code.Matches(pass, checkFixedLengthTypeShiftQ) {
		switch e := node.(type) {
		case *ast.AssignStmt:
			if size, shift, yes := isDubiousShift(e.Lhs[0], e.Rhs[0]); yes {
				report.Report(pass, e, fmt.Sprintf("shifting %d-bit value by %d bits will always clear it", size, shift))
			}
		case *ast.BinaryExpr:
			if size, shift, yes := isDubiousShift(e.X, e.Y); yes {
				report.Report(pass, e, fmt.Sprintf("shifting %d-bit value by %d bits will always clear it", size, shift))
			}
		}
	}

	return nil, nil
}
