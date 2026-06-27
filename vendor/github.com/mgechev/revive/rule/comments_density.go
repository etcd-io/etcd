package rule

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/mgechev/revive/lint"
)

// CommentsDensityRule enforces a minimum comment / code relation.
type CommentsDensityRule struct {
	minimumCommentsDensity int64
}

const defaultMinimumCommentsPercentage = 0

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *CommentsDensityRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		r.minimumCommentsDensity = defaultMinimumCommentsPercentage
		return nil
	}

	var ok bool
	r.minimumCommentsDensity, ok = arguments[0].(int64)
	if !ok {
		return fmt.Errorf("invalid argument for %q rule: argument should be an int, got %T", r.Name(), arguments[0])
	}
	return nil
}

// Apply applies the rule to given file.
func (r *CommentsDensityRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	commentsLines := countDocLines(file.AST.Comments)
	statementsCount := countStatements(file.AST)
	density := (float32(commentsLines) / float32(statementsCount+commentsLines)) * 100

	if density < float32(r.minimumCommentsDensity) {
		return []lint.Failure{
			{
				Node:       file.AST,
				Confidence: 1,
				Failure: fmt.Sprintf("the file has a comment density of %2.f%% (%d comment lines for %d code lines) but expected a minimum of %d%%",
					density, commentsLines, statementsCount, r.minimumCommentsDensity),
			},
		}
	}

	return nil
}

// Name returns the rule name.
func (*CommentsDensityRule) Name() string {
	return "comments-density"
}

// countStatements counts the number of program statements in the given AST.
func countStatements(node ast.Node) int {
	counter := 0

	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ExprStmt, *ast.AssignStmt, *ast.ReturnStmt, *ast.GoStmt, *ast.DeferStmt,
			*ast.BranchStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt,
			*ast.SelectStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause,
			*ast.DeclStmt, *ast.FuncDecl:
			counter++
		}
		return true
	})

	return counter
}

func countDocLines(comments []*ast.CommentGroup) int {
	acc := 0
	for _, c := range comments {
		lines := strings.Split(c.Text(), "\n")
		acc += len(lines) - 1
	}

	return acc
}
