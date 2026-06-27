package checkers

import (
	"fmt"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/Antonboom/testifylint/internal/testify"
)

// BlankImport detects useless blank imports of testify's packages.
// These imports are useless since testify doesn't do any magic with init() function.
//
// The checker detects situations like
//
//	import (
//		"testing"
//
//		_ "github.com/stretchr/testify"
//		_ "github.com/stretchr/testify/assert"
//		_ "github.com/stretchr/testify/http"
//		_ "github.com/stretchr/testify/mock"
//		_ "github.com/stretchr/testify/require"
//		_ "github.com/stretchr/testify/suite"
//	)
//
// and requires
//
//	import (
//		"testing"
//	)
type BlankImport struct{}

// NewBlankImport constructs BlankImport checker.
func NewBlankImport() BlankImport { return BlankImport{} }
func (BlankImport) Name() string  { return "blank-import" }

func (checker BlankImport) Check(pass *analysis.Pass, _ *inspector.Inspector) (diagnostics []analysis.Diagnostic) {
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			if imp.Name == nil || imp.Name.Name != "_" {
				continue
			}

			pkg, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if _, ok := packagesNotIntendedForBlankImport[pkg]; !ok {
				continue
			}

			msg := fmt.Sprintf("avoid blank import of %s as it does nothing", pkg)
			diagnostics = append(diagnostics, *newDiagnostic(checker.Name(), imp, msg))
		}
	}
	return diagnostics
}

var packagesNotIntendedForBlankImport = map[string]struct{}{
	testify.ModulePath:     {},
	testify.AssertPkgPath:  {},
	testify.HTTPPkgPath:    {},
	testify.MockPkgPath:    {},
	testify.RequirePkgPath: {},
	testify.SuitePkgPath:   {},
}
