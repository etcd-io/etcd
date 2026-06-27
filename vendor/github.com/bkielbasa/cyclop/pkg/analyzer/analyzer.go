package analyzer

import (
	"flag"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

//nolint:gochecknoglobals
var flagSet flag.FlagSet

//nolint:gochecknoglobals
var (
	maxComplexity  int
	packageAverage float64
	skipTests      bool
)

const (
	defaultMaxComplexity = 10
)

//nolint:gochecknoinits
func init() {
	flagSet.IntVar(&maxComplexity, "maxComplexity", defaultMaxComplexity, "max complexity the function can have")
	flagSet.Float64Var(&packageAverage, "packageAverage", 0, "max average complexity in package")
	flagSet.BoolVar(&skipTests, "skipTests", false, "should the linter execute on test files as well")
}

func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:  "cyclop",
		Doc:   "checks function and package cyclomatic complexity",
		Run:   run,
		Flags: flagSet,
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	var sum, count float64
	var pkgName string
	var pkgPos token.Pos

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			funcDecl, ok := node.(*ast.FuncDecl)
			if !ok {
				if node == nil {
					return true
				}
				if file, ok := node.(*ast.File); ok {
					pkgName = file.Name.Name
					pkgPos = node.Pos()
				}
				// we check function by function
				return true
			}

			if skipTests && testFunc(funcDecl) {
				return true
			}

			count++
			comp := complexity(funcDecl)
			sum += float64(comp)
			if comp > maxComplexity {
				pass.Reportf(node.Pos(), "calculated cyclomatic complexity for function %s is %d, max is %d", funcDecl.Name.Name, comp, maxComplexity)
			}

			return true
		})
	}

	if packageAverage > 0 {
		avg := sum / count
		if avg > packageAverage {
			pass.Reportf(pkgPos, "the average complexity for the package %s is %f, max is %f", pkgName, avg, packageAverage)
		}
	}

	return nil, nil
}

func testFunc(f *ast.FuncDecl) bool {
	return strings.HasPrefix(f.Name.Name, "Test")
}

func complexity(fn *ast.FuncDecl) int {
	v := complexityVisitor{}
	ast.Walk(&v, fn)
	return v.Complexity
}

type complexityVisitor struct {
	Complexity int
}

func (v *complexityVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.FuncDecl, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
		v.Complexity++
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			v.Complexity++
		}
	}
	return v
}
