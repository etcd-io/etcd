package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/mgechev/revive/lint"
)

// EpochNamingRule lints epoch time variable naming.
type EpochNamingRule struct{}

// Apply applies the rule to given file.
func (*EpochNamingRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	walker := lintEpochNaming{
		file: file,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	if err := file.Pkg.TypeCheck(); err != nil {
		return []lint.Failure{
			lint.NewInternalFailure(fmt.Sprintf("Unable to type check file %q: %v", file.Name, err)),
		}
	}
	ast.Walk(walker, file.AST)

	return failures
}

// Name returns the rule name.
func (*EpochNamingRule) Name() string {
	return "epoch-naming"
}

type lintEpochNaming struct {
	file      *lint.File
	onFailure func(lint.Failure)
}

var epochUnits = map[string][]string{
	"Unix":      {"Sec", "Second", "Seconds"},
	"UnixMilli": {"Milli", "Ms"},
	"UnixMicro": {"Micro", "Microsecond", "Microseconds", "Us"},
	"UnixNano":  {"Nano", "Ns"},
}

func (w lintEpochNaming) Visit(node ast.Node) ast.Visitor {
	switch v := node.(type) {
	case *ast.ValueSpec:
		// Handle var declarations
		valuesLen := len(v.Values)
		for i, name := range v.Names {
			if i >= valuesLen {
				break
			}

			w.check(name, v.Values[i])
		}
	case *ast.AssignStmt:
		// Handle both short variable declarations (:=) and regular assignments (=)
		if v.Tok != token.DEFINE && v.Tok != token.ASSIGN {
			return w
		}

		rhsLen := len(v.Rhs)

		for i, lhs := range v.Lhs {
			if i >= rhsLen {
				break
			}
			ident, ok := lhs.(*ast.Ident)
			if !ok || ident.Name == "_" {
				continue
			}
			w.check(ident, v.Rhs[i])
		}
	}

	return w
}

func (w lintEpochNaming) check(name *ast.Ident, value ast.Expr) {
	call, ok := value.(*ast.CallExpr)
	if !ok {
		return
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Check if the receiver is of type time.Time
	receiverType := w.file.Pkg.TypeOf(selector.X)
	if receiverType == nil {
		return
	}
	if !isTime(receiverType) {
		return
	}

	methodName := selector.Sel.Name
	suffixes, ok := epochUnits[methodName]
	if !ok {
		return
	}

	varName := name.Name
	if !hasAnySuffix(varName, suffixes) {
		w.onFailure(lint.Failure{
			Confidence: 0.9,
			Node:       name,
			Category:   lint.FailureCategoryNaming,
			Failure:    fmt.Sprintf("var %s should have one of these suffixes: %s", varName, strings.Join(suffixes, ", ")),
		})
	}
}

func isTime(typ types.Type) bool {
	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil {
		return false
	}

	pkg := obj.Pkg()
	return pkg != nil && pkg.Path() == "time" && obj.Name() == "Time"
}

func hasAnySuffix(s string, suffixes []string) bool {
	lowerName := strings.ToLower(s)
	for _, suffix := range suffixes {
		if strings.HasSuffix(lowerName, strings.ToLower(suffix)) {
			return true
		}
	}
	return false
}
