package stylecheck

import (
	"testing"

	"honnef.co/go/tools/lint/testutil"
)

func TestAll(t *testing.T) {
	checks := []testutil.Analyzer{
		{Analyzer: Analyzers["ST1000"], Tests: []testutil.Test{{Dir: "CheckPackageComment-1"}, {Dir: "CheckPackageComment-2"}}},
		{Analyzer: Analyzers["ST1001"], Tests: []testutil.Test{{Dir: "CheckDotImports"}}},
		{Analyzer: Analyzers["ST1003"], Tests: []testutil.Test{{Dir: "CheckNames"}, {Dir: "CheckNames_generated"}}},
		{Analyzer: Analyzers["ST1005"], Tests: []testutil.Test{{Dir: "CheckErrorStrings"}}},
		{Analyzer: Analyzers["ST1006"], Tests: []testutil.Test{{Dir: "CheckReceiverNames"}}},
		{Analyzer: Analyzers["ST1008"], Tests: []testutil.Test{{Dir: "CheckErrorReturn"}}},
		{Analyzer: Analyzers["ST1011"], Tests: []testutil.Test{{Dir: "CheckTimeNames"}}},
		{Analyzer: Analyzers["ST1012"], Tests: []testutil.Test{{Dir: "CheckErrorVarNames"}}},
		{Analyzer: Analyzers["ST1013"], Tests: []testutil.Test{{Dir: "CheckHTTPStatusCodes"}}},
		{Analyzer: Analyzers["ST1015"], Tests: []testutil.Test{{Dir: "CheckDefaultCaseOrder"}}},
		{Analyzer: Analyzers["ST1016"], Tests: []testutil.Test{{Dir: "CheckReceiverNamesIdentical"}}},
		{Analyzer: Analyzers["ST1017"], Tests: []testutil.Test{{Dir: "CheckYodaConditions"}}},
		{Analyzer: Analyzers["ST1018"], Tests: []testutil.Test{{Dir: "CheckInvisibleCharacters"}}},
	}

	testutil.Run(t, checks)
}
