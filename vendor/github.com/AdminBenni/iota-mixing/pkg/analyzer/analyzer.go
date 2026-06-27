package analyzer

import (
	"go/ast"
	"go/token"
	"log"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/AdminBenni/iota-mixing/pkg/analyzer/flags"
)

func GetIotaMixingAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "iotamixing",
		Doc:      "checks if iotas are being used in const blocks with other non-iota declarations.",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	ASTInspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:forcetypeassert // will always be correct type

	// we only need to check Generic Declarations
	nodeFilter := []ast.Node{
		(*ast.GenDecl)(nil),
	}

	ASTInspector.Preorder(nodeFilter, func(node ast.Node) { checkGenericDeclaration(node, pass) })

	return interface{}(nil), nil
}

func checkGenericDeclaration(node ast.Node, pass *analysis.Pass) {
	decl := node.(*ast.GenDecl) //nolint:forcetypeassert // filtered for this node, will always be this type

	if decl.Tok != token.CONST {
		return
	}

	checkConstDeclaration(decl, pass)
}

func checkConstDeclaration(decl *ast.GenDecl, pass *analysis.Pass) {
	iotaFound := false
	valued := make([]*ast.ValueSpec, 0, len(decl.Specs))

	// traverse specs inside const block
	for _, spec := range decl.Specs {
		if specVal, ok := spec.(*ast.ValueSpec); ok {
			iotaFound, valued = checkValueSpec(specVal, iotaFound, valued)
		}
	}

	if !iotaFound {
		return
	}

	// there was an iota, now depending on the report-individual flag we must either
	// report the const block or all regular valued specs that are mixing with the iota
	switch flags.ReportIndividualFlag() {
	case flags.TrueString:
		for _, value := range valued {
			pass.Reportf(
				value.Pos(),
				"%s is a const with r-val in same const block as iota. keep iotas in separate const blocks",
				getName(value),
			)
		}
	default: //nolint:gocritic // default logs error and falls through to "false" case, simplest in this order
		log.Printf(
			"warning: unsupported value '%s' for flag %s, assuming value 'false'.",
			flags.ReportIndividualFlag(), flags.ReportIndividualFlagName,
		)

		fallthrough
	case flags.FalseString:
		if len(valued) == 0 {
			return
		}

		pass.Reportf(decl.Pos(), "iota mixing. keep iotas in separate blocks to consts with r-val")
	}
}

func checkValueSpec(spec *ast.ValueSpec, iotaFound bool, valued []*ast.ValueSpec) (bool, []*ast.ValueSpec) {
	// traverse through values (r-val) of spec and look for iota
	for _, expr := range spec.Values {
		if idn, ok := expr.(*ast.Ident); ok && idn.Name == "iota" {
			return true, valued
		}
	}

	// iota wasn't found, add to valued spec list if there is an r-val
	if len(spec.Values) > 0 {
		return iotaFound, append(valued, spec)
	}

	return iotaFound, valued
}

func getName(spec *ast.ValueSpec) string {
	sb := strings.Builder{}

	for i, ident := range spec.Names {
		sb.WriteString(ident.Name)

		if i < len(spec.Names)-1 {
			sb.WriteString(", ")
		}
	}

	return sb.String()
}
