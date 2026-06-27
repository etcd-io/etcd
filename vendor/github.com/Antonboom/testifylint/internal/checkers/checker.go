package checkers

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

// Checker describes named checker.
type Checker interface {
	Name() string
}

// RegularChecker check assertion call presented in CallMeta form.
type RegularChecker interface {
	Checker
	Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic
}

// AdvancedChecker implements complex Check logic different from trivial CallMeta check.
type AdvancedChecker interface {
	Checker
	Check(pass *analysis.Pass, insp *inspector.Inspector) []analysis.Diagnostic
}
