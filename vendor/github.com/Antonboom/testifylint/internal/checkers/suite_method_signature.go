package checkers

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/Antonboom/testifylint/internal/analysisutil"
	"github.com/Antonboom/testifylint/internal/testify"
)

// SuiteMethodSignature warns, when
//   - suite's test methods have arguments or returning values;
//   - suite's custom (user) methods conflicts with (shadows/overrides) suite functionality methods.
type SuiteMethodSignature struct{}

// NewSuiteMethodSignature constructs SuiteMethodSignature checker.
func NewSuiteMethodSignature() SuiteMethodSignature { return SuiteMethodSignature{} }
func (SuiteMethodSignature) Name() string           { return "suite-method-signature" }

func (checker SuiteMethodSignature) Check(pass *analysis.Pass, insp *inspector.Inspector) (diags []analysis.Diagnostic) {
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(node ast.Node) {
		fd := node.(*ast.FuncDecl)
		if !isSuiteMethod(pass, fd) {
			return
		}

		methodName := fd.Name.Name
		rcv := fd.Recv.List[0]

		if isSuiteTestMethod(methodName) {
			if fd.Type.Params.NumFields() > 0 || fd.Type.Results.NumFields() > 0 {
				const msg = "test method should not have any arguments or returning values"
				diags = append(diags, *newDiagnostic(checker.Name(), fd, msg))
				return
			}
		}

		iface, ok := suiteMethodToInterface[methodName]
		if !ok {
			return
		}
		ifaceObj := analysisutil.ObjectOf(pass.Pkg, testify.SuitePkgPath, iface)
		if ifaceObj == nil {
			return
		}
		if !implements(pass, rcv.Type, ifaceObj) {
			msg := fmt.Sprintf("method conflicts with %s.%s interface", testify.SuitePkgName, iface)
			diags = append(diags, *newDiagnostic(checker.Name(), fd, msg))
		}
	})
	return diags
}

// https://github.com/stretchr/testify/blob/master/suite/interfaces.go
var suiteMethodToInterface = map[string]string{
	// NOTE(a.telyshev): We ignore suite.TestingSuite interface, because
	//  - suite.Run will not work for suite-types that do not implement it;
	//  - this interface has several methods, which may cause a false positive for one of the methods in the suite-type.
	"SetupSuite":      "SetupAllSuite",
	"SetupTest":       "SetupTestSuite",
	"TearDownSuite":   "TearDownAllSuite",
	"TearDownTest":    "TearDownTestSuite",
	"BeforeTest":      "BeforeTest",
	"AfterTest":       "AfterTest",
	"HandleStats":     "WithStats",
	"SetupSubTest":    "SetupSubTest",
	"TearDownSubTest": "TearDownSubTest",
}
