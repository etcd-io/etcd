// Package usetesting It is an analyzer that detects when some calls can be replaced by methods from the testing package.
package usetesting

import (
	"go/ast"
	"go/build"
	"os"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	chdirName      = "Chdir"
	mkdirTempName  = "MkdirTemp"
	createTempName = "CreateTemp"
	setenvName     = "Setenv"
	tempDirName    = "TempDir"
	backgroundName = "Background"
	todoName       = "TODO"
	contextName    = "Context"
)

const (
	osPkgName      = "os"
	contextPkgName = "context"
	testingPkgName = "testing"
)

// FuncInfo information about the test function.
type FuncInfo struct {
	Name    string
	ArgName string
}

// analyzer is the UseTesting linter.
type analyzer struct {
	contextBackground bool
	contextTodo       bool
	osChdir           bool
	osMkdirTemp       bool
	osTempDir         bool
	osSetenv          bool
	osCreateTemp      bool

	fieldNames []string

	skipGoVersionDetection bool
}

// NewAnalyzer create a new UseTesting.
func NewAnalyzer() *analysis.Analyzer {
	_, skip := os.LookupEnv("USETESTING_SKIP_GO_VERSION_CHECK") // TODO should be removed when go1.25 will be released.

	l := &analyzer{
		fieldNames: []string{
			chdirName,
			mkdirTempName,
			tempDirName,
			setenvName,
			backgroundName,
			todoName,
			createTempName,
		},
		skipGoVersionDetection: skip,
	}

	a := &analysis.Analyzer{
		Name:     "usetesting",
		Doc:      "Reports uses of functions with replacement inside the testing package.",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      l.run,
	}

	a.Flags.BoolVar(&l.contextBackground, "contextbackground", false, "Enable/disable context.Background() detections")
	a.Flags.BoolVar(&l.contextTodo, "contexttodo", false, "Enable/disable context.TODO() detections")
	a.Flags.BoolVar(&l.osChdir, "oschdir", true, "Enable/disable os.Chdir() detections")
	a.Flags.BoolVar(&l.osMkdirTemp, "osmkdirtemp", true, "Enable/disable os.MkdirTemp() detections")
	a.Flags.BoolVar(&l.osSetenv, "ossetenv", false, "Enable/disable os.Setenv() detections")
	a.Flags.BoolVar(&l.osTempDir, "ostempdir", false, "Enable/disable os.TempDir() detections")
	a.Flags.BoolVar(&l.osCreateTemp, "oscreatetemp", true, `Enable/disable os.CreateTemp("", ...) detections`)

	return a
}

func (a *analyzer) run(pass *analysis.Pass) (any, error) {
	if !a.contextBackground && !a.contextTodo && !a.osChdir && !a.osMkdirTemp && !a.osSetenv && !a.osTempDir && !a.osCreateTemp {
		return nil, nil
	}

	geGo124 := a.isGoSupported(pass)

	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	insp.WithStack(nodeFilter, func(node ast.Node, push bool, stack []ast.Node) (proceed bool) {
		if !push {
			return false
		}

		switch fn := node.(type) {
		case *ast.FuncDecl:
			a.checkFunc(pass, fn.Type, fn.Body, fn.Name.Name, geGo124)

		case *ast.FuncLit:
			if hasParentFunc(stack) {
				return true
			}

			a.checkFunc(pass, fn.Type, fn.Body, "anonymous function", geGo124)
		}

		return true
	})

	return nil, nil
}

func (a *analyzer) checkFunc(pass *analysis.Pass, ft *ast.FuncType, block *ast.BlockStmt, fnName string, geGo124 bool) {
	if len(ft.Params.List) < 1 {
		return
	}

	fnInfo := checkTestFunctionSignature(ft.Params.List[0], fnName)
	if fnInfo == nil {
		return
	}

	ast.Inspect(block, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			return !a.reportSelector(pass, v, fnInfo, geGo124)

		case *ast.Ident:
			return !a.reportIdent(pass, v, fnInfo, geGo124)

		case *ast.CallExpr:
			return !a.reportCallExpr(pass, v, fnInfo)
		}

		return true
	})
}

func (a *analyzer) isGoSupported(pass *analysis.Pass) bool {
	if a.skipGoVersionDetection {
		return true
	}

	// Prior to go1.22, versions.FileVersion returns only the toolchain version,
	// which is of no use to us,
	// so disable this analyzer on earlier versions.
	if !slices.Contains(build.Default.ReleaseTags, "go1.22") {
		return false
	}

	pkgVersion := pass.Pkg.GoVersion()
	if pkgVersion == "" {
		// Empty means Go devel.
		return true
	}

	raw := strings.TrimPrefix(pkgVersion, "go")

	// prerelease version (go1.24rc1)
	idx := strings.IndexFunc(raw, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})

	if idx != -1 {
		raw = raw[:idx]
	}

	vParts := strings.Split(raw, ".")

	v, err := strconv.Atoi(strings.Join(vParts[:2], ""))
	if err != nil {
		v = 116
	}

	return v >= 124
}

func hasParentFunc(stack []ast.Node) bool {
	// -2 because the last parent is the node.
	const skipSelf = 2

	// skip 0 because it's always [*ast.File].
	for i := len(stack) - skipSelf; i > 0; i-- {
		s := stack[i]

		switch fn := s.(type) {
		case *ast.FuncDecl:
			if len(fn.Type.Params.List) < 1 {
				continue
			}

			if checkTestFunctionSignature(fn.Type.Params.List[0], fn.Name.Name) != nil {
				return true
			}

		case *ast.FuncLit:
			if len(fn.Type.Params.List) < 1 {
				continue
			}

			if checkTestFunctionSignature(fn.Type.Params.List[0], "anonymous function") != nil {
				return true
			}
		}
	}

	return false
}

func checkTestFunctionSignature(arg *ast.Field, fnName string) *FuncInfo {
	switch at := arg.Type.(type) {
	case *ast.StarExpr:
		if se, ok := at.X.(*ast.SelectorExpr); ok {
			return createFuncInfo(arg, "<t/b>", se, testingPkgName, fnName, "T", "B")
		}

	case *ast.SelectorExpr:
		return createFuncInfo(arg, "tb", at, testingPkgName, fnName, "TB")
	}

	return nil
}

func createFuncInfo(arg *ast.Field, defaultName string, se *ast.SelectorExpr, pkgName, fnName string, selectorNames ...string) *FuncInfo {
	ok := checkSelectorName(se, pkgName, selectorNames...)
	if !ok {
		return nil
	}

	return &FuncInfo{
		Name:    fnName,
		ArgName: getTestArgName(arg, defaultName),
	}
}

func checkSelectorName(se *ast.SelectorExpr, pkgName string, selectorNames ...string) bool {
	if ident, ok := se.X.(*ast.Ident); ok {
		return pkgName == ident.Name && slices.Contains(selectorNames, se.Sel.Name)
	}

	return false
}

func getTestArgName(arg *ast.Field, defaultName string) string {
	if len(arg.Names) > 0 && arg.Names[0].Name != "_" {
		return arg.Names[0].Name
	}

	return defaultName
}
