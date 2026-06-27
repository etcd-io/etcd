package internal

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

func NewMisplacedEmbeddedFieldDiag(embeddedField *ast.Field) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos:     embeddedField.Pos(),
		Message: "embedded fields should be listed before regular fields",
	}
}

func NewMissingSpaceDiag(
	lastEmbeddedField *ast.Field,
	firstRegularField *ast.Field,
) analysis.Diagnostic {
	suggestedPos := firstRegularField.Pos()
	if firstRegularField.Doc != nil {
		suggestedPos = firstRegularField.Doc.Pos()
	}

	return analysis.Diagnostic{
		Pos:     lastEmbeddedField.Pos(),
		Message: "there must be an empty line separating embedded fields from regular fields",
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: "adding extra line separating embedded fields from regular fields",
				TextEdits: []analysis.TextEdit{
					{
						Pos:     suggestedPos,
						NewText: []byte("\n\n"),
					},
				},
			},
		},
	}
}

func NewForbiddenEmbeddedFieldDiag(forbidField *ast.SelectorExpr) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos:     forbidField.Pos(),
		Message: fmt.Sprintf("%s.%s should not be embedded", forbidField.X, forbidField.Sel.Name),
	}
}
