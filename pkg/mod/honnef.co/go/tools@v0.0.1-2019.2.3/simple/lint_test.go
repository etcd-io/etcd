package simple

import (
	"testing"

	"honnef.co/go/tools/lint/testutil"
)

func TestAll(t *testing.T) {
	checks := []testutil.Analyzer{
		{Analyzer: Analyzers["S1000"], Tests: []testutil.Test{{Dir: "single-case-select"}}},
		{Analyzer: Analyzers["S1001"], Tests: []testutil.Test{{Dir: "copy"}}},
		{Analyzer: Analyzers["S1002"], Tests: []testutil.Test{{Dir: "bool-cmp"}}},
		{Analyzer: Analyzers["S1003"], Tests: []testutil.Test{{Dir: "contains"}}},
		{Analyzer: Analyzers["S1004"], Tests: []testutil.Test{{Dir: "compare"}}},
		{Analyzer: Analyzers["S1005"], Tests: []testutil.Test{{Dir: "LintBlankOK"}, {Dir: "receive-blank"}, {Dir: "range_go13", Version: "1.3"}, {Dir: "range_go14", Version: "1.4"}}},
		{Analyzer: Analyzers["S1006"], Tests: []testutil.Test{{Dir: "for-true"}, {Dir: "generated"}}},
		{Analyzer: Analyzers["S1007"], Tests: []testutil.Test{{Dir: "regexp-raw"}}},
		{Analyzer: Analyzers["S1008"], Tests: []testutil.Test{{Dir: "if-return"}}},
		{Analyzer: Analyzers["S1009"], Tests: []testutil.Test{{Dir: "nil-len"}}},
		{Analyzer: Analyzers["S1010"], Tests: []testutil.Test{{Dir: "slicing"}}},
		{Analyzer: Analyzers["S1011"], Tests: []testutil.Test{{Dir: "loop-append"}}},
		{Analyzer: Analyzers["S1012"], Tests: []testutil.Test{{Dir: "time-since"}}},
		{Analyzer: Analyzers["S1016"], Tests: []testutil.Test{{Dir: "convert"}, {Dir: "convert_go17", Version: "1.7"}, {Dir: "convert_go18", Version: "1.8"}}},
		{Analyzer: Analyzers["S1017"], Tests: []testutil.Test{{Dir: "trim"}}},
		{Analyzer: Analyzers["S1018"], Tests: []testutil.Test{{Dir: "LintLoopSlide"}}},
		{Analyzer: Analyzers["S1019"], Tests: []testutil.Test{{Dir: "LintMakeLenCap"}}},
		{Analyzer: Analyzers["S1020"], Tests: []testutil.Test{{Dir: "LintAssertNotNil"}}},
		{Analyzer: Analyzers["S1021"], Tests: []testutil.Test{{Dir: "LintDeclareAssign"}}},
		{Analyzer: Analyzers["S1023"], Tests: []testutil.Test{{Dir: "LintRedundantBreak"}, {Dir: "LintRedundantReturn"}}},
		{Analyzer: Analyzers["S1024"], Tests: []testutil.Test{{Dir: "LimeTimeUntil_go17", Version: "1.7"}, {Dir: "LimeTimeUntil_go18", Version: "1.8"}}},
		{Analyzer: Analyzers["S1025"], Tests: []testutil.Test{{Dir: "LintRedundantSprintf"}}},
		{Analyzer: Analyzers["S1028"], Tests: []testutil.Test{{Dir: "LintErrorsNewSprintf"}}},
		{Analyzer: Analyzers["S1029"], Tests: []testutil.Test{{Dir: "LintRangeStringRunes"}}},
		{Analyzer: Analyzers["S1030"], Tests: []testutil.Test{{Dir: "LintBytesBufferConversions"}}},
		{Analyzer: Analyzers["S1031"], Tests: []testutil.Test{{Dir: "LintNilCheckAroundRange"}}},
		{Analyzer: Analyzers["S1032"], Tests: []testutil.Test{{Dir: "LintSortHelpers"}}},
		{Analyzer: Analyzers["S1033"], Tests: []testutil.Test{{Dir: "LintGuardedDelete"}}},
		{Analyzer: Analyzers["S1034"], Tests: []testutil.Test{{Dir: "LintSimplifyTypeSwitch"}}},
	}

	testutil.Run(t, checks)
}
