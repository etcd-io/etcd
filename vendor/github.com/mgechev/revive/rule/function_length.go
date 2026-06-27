package rule

import (
	"fmt"
	"go/ast"
	"reflect"

	"github.com/mgechev/revive/lint"
)

// FunctionLength lint.
type FunctionLength struct {
	maxStmt  int
	maxLines int
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *FunctionLength) Configure(arguments lint.Arguments) error {
	maxStmt, maxLines, err := r.parseArguments(arguments)
	if err != nil {
		return err
	}
	r.maxStmt = int(maxStmt)
	r.maxLines = int(maxLines)
	return nil
}

// Apply applies the rule to given file.
func (r *FunctionLength) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	for _, decl := range file.AST.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		body := funcDecl.Body
		emptyBody := body == nil || len(body.List) == 0
		if emptyBody {
			return nil
		}

		if r.maxStmt > 0 {
			stmtCount := r.countStmts(body.List)
			if stmtCount > r.maxStmt {
				failures = append(failures, lint.Failure{
					Confidence: 1,
					Failure:    fmt.Sprintf("maximum number of statements per function exceeded; max %d but got %d", r.maxStmt, stmtCount),
					Node:       funcDecl,
				})
			}
		}

		if r.maxLines > 0 {
			lineCount := r.countLines(body, file)
			if lineCount > r.maxLines {
				failures = append(failures, lint.Failure{
					Confidence: 1,
					Failure:    fmt.Sprintf("maximum number of lines per function exceeded; max %d but got %d", r.maxLines, lineCount),
					Node:       funcDecl,
				})
			}
		}
	}

	return failures
}

// Name returns the rule name.
func (*FunctionLength) Name() string {
	return "function-length"
}

const (
	defaultFuncStmtsLimit = 50
	defaultFuncLinesLimit = 75
)

func (*FunctionLength) parseArguments(arguments lint.Arguments) (maxStmt, maxLines int64, err error) {
	if len(arguments) == 0 {
		return defaultFuncStmtsLimit, defaultFuncLinesLimit, nil
	}

	const minArguments = 2
	if len(arguments) != minArguments {
		return 0, 0, fmt.Errorf(`invalid configuration for "function-length" rule, expected %d arguments but got %d`, minArguments, len(arguments))
	}

	maxStmt, maxStmtOk := arguments[0].(int64)
	if !maxStmtOk {
		return 0, 0, fmt.Errorf(`invalid configuration value for max statements in "function-length" rule; need int64 but got %T`, arguments[0])
	}
	if maxStmt < 0 {
		return 0, 0, fmt.Errorf(`the configuration value for max statements in "function-length" rule cannot be negative, got %d`, maxStmt)
	}

	maxLines, maxLinesOk := arguments[1].(int64)
	if !maxLinesOk {
		return 0, 0, fmt.Errorf(`invalid configuration value for max lines in "function-length" rule; need int64 but got %T`, arguments[1])
	}
	if maxLines < 0 {
		return 0, 0, fmt.Errorf(`the configuration value for max statements in "function-length" rule cannot be negative, got %d`, maxLines)
	}

	return maxStmt, maxLines, nil
}

func (*FunctionLength) countLines(b *ast.BlockStmt, file *lint.File) int {
	return file.ToPosition(b.End()).Line - file.ToPosition(b.Pos()).Line - 1
}

func (r *FunctionLength) countStmts(b []ast.Stmt) int {
	count := 0
	for _, s := range b {
		switch stmt := s.(type) {
		case *ast.BlockStmt:
			count += r.countStmts(stmt.List)
		case *ast.IfStmt:
			count += 1 + r.countBodyListStmts(stmt)
			if stmt.Else != nil {
				elseBody, ok := stmt.Else.(*ast.BlockStmt)
				if ok {
					count += r.countStmts(elseBody.List)
				}
			}
		case *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			count += 1 + r.countBodyListStmts(stmt)
		case *ast.CaseClause:
			count += r.countStmts(stmt.Body)
		case *ast.AssignStmt:
			count += 1 + r.countFuncLitStmts(stmt.Rhs[0])
		case *ast.GoStmt:
			count += 1 + r.countFuncLitStmts(stmt.Call.Fun)
		case *ast.DeferStmt:
			count += 1 + r.countFuncLitStmts(stmt.Call.Fun)
		default:
			count++
		}
	}

	return count
}

func (r *FunctionLength) countFuncLitStmts(stmt ast.Expr) int {
	if block, ok := stmt.(*ast.FuncLit); ok {
		return r.countStmts(block.Body.List)
	}

	return 0
}

func (r *FunctionLength) countBodyListStmts(t any) int {
	i := reflect.ValueOf(t).Elem().FieldByName(`Body`).Elem().FieldByName(`List`).Interface()
	return r.countStmts(i.([]ast.Stmt))
}
