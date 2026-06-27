package analyzer

import (
	"golang.org/x/tools/go/analysis"
)

func newAnalysisDiagnostic(
	checker string,
	analysisRange analysis.Range,
	message string,
	suggestedFixes []analysis.SuggestedFix,
) *analysis.Diagnostic {
	if checker != "" {
		message = checker + ": " + message
	}

	return &analysis.Diagnostic{
		Pos:            analysisRange.Pos(),
		End:            analysisRange.End(),
		SuggestedFixes: suggestedFixes,
		Message:        message,
		Category:       checker, // Possible hashtag available on the documentation
	}
}
