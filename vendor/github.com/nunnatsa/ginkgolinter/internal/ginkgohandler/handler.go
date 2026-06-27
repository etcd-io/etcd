package ginkgohandler

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/config"
)

const (
	importPath   = `"github.com/onsi/ginkgo"`
	importPathV2 = `"github.com/onsi/ginkgo/v2"`

	focusSpec = "Focus"
)

// Handler provide different handling, depend on the way ginkgo was imported, whether
// in imported with "." name, custom name or without any name.
type Handler interface {
	HandleGinkgoSpecs(ast.Expr, config.Config, *analysis.Pass) bool
	getFocusContainerName(*ast.CallExpr) (bool, *ast.Ident)
	isWrapContainer(*ast.CallExpr) bool
	isFocusSpec(ident ast.Expr) bool
}

// GetGinkgoHandler returns a ginkgor handler according to the way ginkgo was imported in the specific file
func GetGinkgoHandler(file *ast.File) Handler {
	for _, imp := range file.Imports {
		switch imp.Path.Value {
		case importPath, importPathV2:
			switch name := imp.Name.String(); name {
			case ".":
				return dotHandler{}
			case "<nil>": // import with no local name
				return nameHandler("ginkgo")
			default:
				return nameHandler(name)
			}

		default:
			continue
		}
	}

	return nil
}
