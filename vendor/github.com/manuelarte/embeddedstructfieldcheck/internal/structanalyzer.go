package internal

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

func Analyze(pass *analysis.Pass, st *ast.StructType, emptyLine, forbidMutex bool) {
	var firstEmbeddedField *ast.Field

	var lastEmbeddedField *ast.Field

	var firstNotEmbeddedField *ast.Field

	for _, field := range st.Fields.List {
		if isFieldEmbedded(field) {
			checkForbiddenEmbeddedField(pass, field, forbidMutex)

			if firstEmbeddedField == nil {
				firstEmbeddedField = field
			}

			if lastEmbeddedField == nil || lastEmbeddedField.Pos() < field.Pos() {
				lastEmbeddedField = field
			}

			if firstNotEmbeddedField != nil && firstNotEmbeddedField.Pos() < field.Pos() {
				pass.Report(NewMisplacedEmbeddedFieldDiag(field))
				return
			}
		} else if firstNotEmbeddedField == nil {
			firstNotEmbeddedField = field
		}
	}

	if emptyLine {
		checkMissingSpace(pass, lastEmbeddedField, firstNotEmbeddedField)
	}
}

func checkForbiddenEmbeddedField(pass *analysis.Pass, field *ast.Field, forbidMutex bool) {
	if !forbidMutex {
		return
	}

	switch e := field.Type.(type) {
	case *ast.StarExpr:
		if se, isSelectorExpr := e.X.(*ast.SelectorExpr); isSelectorExpr {
			reportSyncMutex(pass, se)
		}

	case *ast.SelectorExpr:
		reportSyncMutex(pass, e)
	}
}

func checkMissingSpace(pass *analysis.Pass, lastEmbeddedField, firstNotEmbeddedField *ast.Field) {
	if lastEmbeddedField != nil && firstNotEmbeddedField != nil {
		// check for missing space
		// TODO: isn't it easy to remove as many lines as comments between last embedded type and first not embedded
		line := pass.Fset.Position(lastEmbeddedField.End()).Line

		nextLine := pass.Fset.Position(firstNotEmbeddedField.Pos()).Line
		if firstNotEmbeddedField.Doc != nil {
			nextLine = pass.Fset.Position(firstNotEmbeddedField.Doc.Pos()).Line
		}

		if nextLine != line+2 {
			pass.Report(NewMissingSpaceDiag(lastEmbeddedField, firstNotEmbeddedField))
		}
	}
}

func isFieldEmbedded(field *ast.Field) bool {
	return len(field.Names) == 0
}

func reportSyncMutex(pass *analysis.Pass, se *ast.SelectorExpr) {
	packageName, isIdent := se.X.(*ast.Ident)
	if !isIdent {
		return
	}

	if packageName.Name == "sync" && (se.Sel.Name == "Mutex" || se.Sel.Name == "RWMutex") {
		pass.Report(NewForbiddenEmbeddedFieldDiag(se))
	}
}
