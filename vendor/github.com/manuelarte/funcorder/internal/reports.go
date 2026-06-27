package internal

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

func reportConstructorNotAfterStructType(pass *analysis.Pass, structSpec *ast.TypeSpec, constructor *ast.FuncDecl) {
	pass.Report(analysis.Diagnostic{
		Pos: constructor.Pos(),
		Message: fmt.Sprintf("constructor %q for struct %q should be placed after the struct declaration",
			constructor.Name, structSpec.Name),
		URL: "https://github.com/manuelarte/funcorder?tab=readme-ov-file#check-constructors-functions-are-placed-after-struct-declaration", //nolint:lll // url
	})
}

func reportConstructorNotBeforeStructMethod(
	pass *analysis.Pass,
	structSpec *ast.TypeSpec,
	constructor, method *ast.FuncDecl,
) {
	pass.Report(analysis.Diagnostic{
		Pos: constructor.Pos(),
		URL: "https://github.com/manuelarte/funcorder?tab=readme-ov-file#check-constructors-functions-are-placed-after-struct-declaration", //nolint:lll // url
		Message: fmt.Sprintf("constructor %q for struct %q should be placed before struct method %q",
			constructor.Name, structSpec.Name, method.Name),
	})
}

func reportAdjacentConstructorsNotSortedAlphabetically(
	pass *analysis.Pass,
	structSpec *ast.TypeSpec,
	constructorNotSorted, otherConstructorNotSorted *ast.FuncDecl,
) {
	pass.Report(analysis.Diagnostic{
		Pos: otherConstructorNotSorted.Pos(),
		URL: "https://github.com/manuelarte/funcorder?tab=readme-ov-file#check-constructorsmethods-are-sorted-alphabetically",
		Message: fmt.Sprintf("constructor %q for struct %q should be placed before constructor %q",
			otherConstructorNotSorted.Name, structSpec.Name, constructorNotSorted.Name),
	})
}

func reportUnexportedMethodBeforeExportedForStruct(
	pass *analysis.Pass,
	structSpec *ast.TypeSpec,
	privateMethod, publicMethod *ast.FuncDecl,
) {
	pass.Report(analysis.Diagnostic{
		Pos: privateMethod.Pos(),
		URL: "https://github.com/manuelarte/funcorder?tab=readme-ov-file#check-exported-methods-are-placed-before-unexported-methods", //nolint:lll // url
		Message: fmt.Sprintf("unexported method %q for struct %q should be placed after the exported method %q",
			privateMethod.Name, structSpec.Name, publicMethod.Name),
	})
}

func reportAdjacentStructMethodsNotSortedAlphabetically(
	pass *analysis.Pass,
	structSpec *ast.TypeSpec,
	method, otherMethod *ast.FuncDecl,
) {
	pass.Report(analysis.Diagnostic{
		Pos: otherMethod.Pos(),
		URL: "https://github.com/manuelarte/funcorder?tab=readme-ov-file#check-constructorsmethods-are-sorted-alphabetically",
		Message: fmt.Sprintf("method %q for struct %q should be placed before method %q",
			otherMethod.Name, structSpec.Name, method.Name),
	})
}

func reportUnexportedFuncBeforeExportedFunc(pass *analysis.Pass, unexportedFunc, exportedFunc *ast.FuncDecl) {
	pass.Report(analysis.Diagnostic{
		Pos: unexportedFunc.Pos(),
		URL: "https://github.com/manuelarte/funcorder?tab=readme-ov-file#check-exported-functions-are-placed-before-unexported-functions", //nolint:lll // url
		Message: fmt.Sprintf("unexported function %q should be placed after the exported function %q",
			unexportedFunc.Name, exportedFunc.Name),
	})
}
