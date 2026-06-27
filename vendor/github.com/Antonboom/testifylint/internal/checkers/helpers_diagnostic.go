package checkers

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

func newRemoveFnAndUseDiagnostic(
	pass *analysis.Pass,
	checker string,
	call *CallMeta,
	proposedFn string,
	removedFn string,
	removedFnPos analysis.Range,
	removedFnArgs ...ast.Expr,
) *analysis.Diagnostic {
	f := proposedFn
	if call.Fn.IsFmt {
		f += "f"
	}
	msg := fmt.Sprintf("remove unnecessary %s and use %s.%s", removedFn, call.SelectorXStr, f)

	return newDiagnostic(checker, call, msg,
		newSuggestedFuncRemoving(pass, removedFn, removedFnPos, removedFnArgs...),
		newSuggestedFuncReplacement(call, proposedFn),
	)
}

func newUseFunctionDiagnostic(
	checker string,
	call *CallMeta,
	proposedFn string,
	additionalEdits ...analysis.TextEdit,
) *analysis.Diagnostic {
	f := proposedFn
	if call.Fn.IsFmt {
		f += "f"
	}
	msg := fmt.Sprintf("use %s.%s", call.SelectorXStr, f)

	return newDiagnostic(checker, call, msg,
		newSuggestedFuncReplacement(call, proposedFn, additionalEdits...))
}

func newRemoveLenDiagnostic(
	pass *analysis.Pass,
	checker string,
	call *CallMeta,
	fnPos analysis.Range,
	fnArg ast.Expr,
) *analysis.Diagnostic {
	return newRemoveFnDiagnostic(pass, checker, call, "len", fnPos, fnArg)
}

func newRemoveMustCompileDiagnostic(
	pass *analysis.Pass,
	checker string,
	call *CallMeta,
	fnPos analysis.Range,
	fnArg ast.Expr,
) *analysis.Diagnostic {
	return newRemoveFnDiagnostic(pass, checker, call, "regexp.MustCompile", fnPos, fnArg)
}

func newRemoveSprintfDiagnostic(
	pass *analysis.Pass,
	checker string,
	call *CallMeta,
	fnPos analysis.Range,
	fnArgs []ast.Expr,
) *analysis.Diagnostic {
	return newRemoveFnDiagnostic(pass, checker, call, "fmt.Sprintf", fnPos, fnArgs...)
}

func newRemoveFnDiagnostic(
	pass *analysis.Pass,
	checker string,
	call *CallMeta,
	fnName string,
	fnPos analysis.Range,
	fnArgs ...ast.Expr,
) *analysis.Diagnostic {
	return newDiagnostic(checker, call, "remove unnecessary "+fnName,
		newSuggestedFuncRemoving(pass, fnName, fnPos, fnArgs...))
}

func newDiagnostic(
	checker string,
	rng analysis.Range,
	msg string,
	fixes ...analysis.SuggestedFix,
) *analysis.Diagnostic {
	d := analysis.Diagnostic{
		Pos:      rng.Pos(),
		End:      rng.End(),
		Category: checker,
		Message:  checker + ": " + msg,
	}
	if len(fixes) != 0 {
		d.SuggestedFixes = fixes
	}
	return &d
}

func newSuggestedFuncRemoving(
	pass *analysis.Pass,
	fnName string,
	fnPos analysis.Range,
	fnArgs ...ast.Expr,
) analysis.SuggestedFix {
	return analysis.SuggestedFix{
		Message: fmt.Sprintf("Remove `%s`", fnName),
		TextEdits: []analysis.TextEdit{
			{
				Pos:     fnPos.Pos(),
				End:     fnPos.End(),
				NewText: formatAsCallArgs(pass, fnArgs...),
			},
		},
	}
}

func newSuggestedFuncReplacement(
	call *CallMeta,
	proposedFn string,
	additionalEdits ...analysis.TextEdit,
) analysis.SuggestedFix {
	if call.Fn.IsFmt {
		proposedFn += "f"
	}
	return analysis.SuggestedFix{
		Message:   fmt.Sprintf("Replace `%s` with `%s`", call.Fn.Name, proposedFn),
		TextEdits: append([]analysis.TextEdit{newReplaceFnTextEdit(call.Fn, proposedFn)}, additionalEdits...),
	}
}

func newReplaceFnTextEdit(callFn analysis.Range, proposedFn string) analysis.TextEdit {
	return analysis.TextEdit{
		Pos:     callFn.Pos(),
		End:     callFn.End(),
		NewText: []byte(proposedFn),
	}
}

func newRemoveLastArgTextEdit(pass *analysis.Pass, callArgs []ast.Expr) analysis.TextEdit {
	return analysis.TextEdit{
		Pos:     callArgs[0].Pos(),
		End:     callArgs[len(callArgs)-1].End(),
		NewText: formatAsCallArgs(pass, callArgs[0:len(callArgs)-1]...),
	}
}
