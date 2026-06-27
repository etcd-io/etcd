package asasalint

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const BuiltinExclusions = `^(fmt|log|logger|t|)\.(Print|Fprint|Sprint|Fatal|Panic|Error|Warn|Warning|Info|Debug|Log)(|f|ln)$`

type LinterSetting struct {
	Exclude             []string
	NoBuiltinExclusions bool
	IgnoreTest          bool
}

func NewAnalyzer(setting LinterSetting) (*analysis.Analyzer, error) {
	a, err := newAnalyzer(setting)
	if err != nil {
		return nil, err
	}

	return &analysis.Analyzer{
		Name:     "asasalint",
		Doc:      "check for pass []any as any in variadic func(...any)",
		Run:      a.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}, nil
}

type analyzer struct {
	excludes []*regexp.Regexp
	setting  LinterSetting
}

func newAnalyzer(setting LinterSetting) (*analyzer, error) {
	a := &analyzer{
		setting: setting,
	}

	if !a.setting.NoBuiltinExclusions {
		a.excludes = append(a.excludes, regexp.MustCompile(BuiltinExclusions))
	}

	for _, exclude := range a.setting.Exclude {
		if exclude != "" {
			exp, err := regexp.Compile(exclude)
			if err != nil {
				return nil, err
			}

			a.excludes = append(a.excludes, exp)
		}
	}

	return a, nil
}

func (a *analyzer) run(pass *analysis.Pass) (interface{}, error) {
	inspectorInfo := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{(*ast.CallExpr)(nil)}

	inspectorInfo.Preorder(nodeFilter, a.AsCheckVisitor(pass))
	return nil, nil
}

func (a *analyzer) AsCheckVisitor(pass *analysis.Pass) func(ast.Node) {
	return func(n ast.Node) {
		if a.setting.IgnoreTest {
			pos := pass.Fset.Position(n.Pos())
			if strings.HasSuffix(pos.Filename, "_test.go") {
				return
			}
		}

		caller, ok := n.(*ast.CallExpr)
		if !ok {
			return
		}
		if caller.Ellipsis != token.NoPos {
			return
		}
		if len(caller.Args) == 0 {
			return
		}

		fnName, err := getFuncName(pass.Fset, caller)
		if err != nil {
			log.Println(err)
			return
		}

		for _, exclude := range a.excludes {
			if exclude.MatchString(fnName) {
				return
			}
		}

		fnType := pass.TypesInfo.TypeOf(caller.Fun)
		if !isSliceAnyVariadicFuncType(fnType) {
			return
		}

		fnSign := fnType.(*types.Signature)
		if len(caller.Args) != fnSign.Params().Len() {
			return
		}

		lastArg := caller.Args[len(caller.Args)-1]
		argType := pass.TypesInfo.TypeOf(lastArg)
		if !isSliceAnyType(argType) {
			return
		}
		node := lastArg

		d := analysis.Diagnostic{
			Pos:      node.Pos(),
			End:      node.End(),
			Message:  fmt.Sprintf("pass []any as any to func %s %s", fnName, fnType.String()),
			Category: "asasalint",
		}
		pass.Report(d)
	}
}

func getFuncName(fset *token.FileSet, caller *ast.CallExpr) (string, error) {
	buf := new(bytes.Buffer)
	if err := printer.Fprint(buf, fset, caller.Fun); err != nil {
		return "", fmt.Errorf("unable to print node at %s: %w", fset.Position(caller.Fun.Pos()), err)
	}

	return buf.String(), nil
}

func isSliceAnyVariadicFuncType(typ types.Type) (r bool) {
	fnSign, ok := typ.(*types.Signature)
	if !ok || !fnSign.Variadic() {
		return false
	}

	params := fnSign.Params()
	lastParam := params.At(params.Len() - 1)
	return isSliceAnyType(lastParam.Type())
}

func isSliceAnyType(typ types.Type) (r bool) {
	sliceType, ok := typ.(*types.Slice)
	if !ok {
		return
	}
	elemType, ok := sliceType.Elem().(*types.Interface)
	if !ok {
		return
	}
	return elemType.NumMethods() == 0
}
