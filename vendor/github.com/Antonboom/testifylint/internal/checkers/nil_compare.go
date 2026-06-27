package checkers

import (
	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// NilCompare detects situations like
//
//	assert.Equal(t, nil, value)
//	assert.EqualValues(t, nil, value)
//	assert.Exactly(t, nil, value)
//
//	assert.NotEqual(t, nil, value)
//	assert.NotEqualValues(t, nil, value)
//
// and requires
//
//	assert.Nil(t, value)
//	assert.NotNil(t, value)
type NilCompare struct{}

// NewNilCompare constructs NilCompare checker.
func NewNilCompare() NilCompare { return NilCompare{} }
func (NilCompare) Name() string { return "nil-compare" }

func (checker NilCompare) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if len(call.Args) < 2 {
		return nil
	}

	survivingArg, ok := xorNil(call.Args[0], call.Args[1])
	if !ok {
		return nil
	}

	var proposedFn string

	switch call.Fn.NameFTrimmed {
	case "Equal", "EqualValues", "Exactly":
		proposedFn = "Nil"
	case "NotEqual", "NotEqualValues":
		proposedFn = "NotNil"
	default:
		return nil
	}

	return newUseFunctionDiagnostic(checker.Name(), call, proposedFn,
		analysis.TextEdit{
			Pos:     call.Args[0].Pos(),
			End:     call.Args[1].End(),
			NewText: analysisutil.NodeBytes(pass.Fset, survivingArg),
		})
}
