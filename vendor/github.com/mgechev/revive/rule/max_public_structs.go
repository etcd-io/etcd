package rule

import (
	"errors"
	"fmt"
	"go/ast"
	"strings"

	"github.com/mgechev/revive/lint"
)

// MaxPublicStructsRule lints the number of public structs in a file.
type MaxPublicStructsRule struct {
	max int64
}

const defaultMaxPublicStructs = 5

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *MaxPublicStructsRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		r.max = defaultMaxPublicStructs
		return nil
	}

	maxStructs, ok := arguments[0].(int64) // Alt. non panicking version
	if !ok {
		return errors.New(`invalid value passed as argument number to the "max-public-structs" rule`)
	}
	r.max = maxStructs
	return nil
}

// Apply applies the rule to given file.
func (r *MaxPublicStructsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	if r.max < 1 {
		return failures
	}

	fileAst := file.AST

	walker := &lintMaxPublicStructs{
		fileAst: fileAst,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	ast.Walk(walker, fileAst)

	if walker.current > r.max {
		walker.onFailure(lint.Failure{
			Failure:    fmt.Sprintf("you have exceeded the maximum number (%d) of public struct declarations", r.max),
			Confidence: 1,
			Node:       fileAst,
			Category:   lint.FailureCategoryStyle,
		})
	}

	return failures
}

// Name returns the rule name.
func (*MaxPublicStructsRule) Name() string {
	return "max-public-structs"
}

type lintMaxPublicStructs struct {
	current   int64
	fileAst   *ast.File
	onFailure func(lint.Failure)
}

func (w *lintMaxPublicStructs) Visit(n ast.Node) ast.Visitor {
	if v, ok := n.(*ast.TypeSpec); ok {
		name := v.Name.Name
		first := string(name[0])
		if strings.ToUpper(first) == first {
			w.current++
		}
	}
	return w
}
