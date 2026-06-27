package funlen

import (
	"go/ast"
	"go/token"
	"reflect"

	"golang.org/x/tools/go/analysis"
)

const (
	defaultLineLimit = 60
	defaultStmtLimit = 40
)

func NewAnalyzer(lineLimit int, stmtLimit int, ignoreComments bool) *analysis.Analyzer {
	if lineLimit == 0 {
		lineLimit = defaultLineLimit
	}

	if stmtLimit == 0 {
		stmtLimit = defaultStmtLimit
	}

	return &analysis.Analyzer{
		Name: "funlen",
		Doc:  "Checks for long functions.",
		URL:  "https://github.com/ultraware/funlen",
		Run: func(pass *analysis.Pass) (any, error) {
			run(pass, lineLimit, stmtLimit, ignoreComments)
			return nil, nil
		},
	}
}

func run(pass *analysis.Pass, lineLimit int, stmtLimit int, ignoreComments bool) {
	for _, file := range pass.Files {
		cmap := ast.NewCommentMap(pass.Fset, file, file.Comments)

		for _, f := range file.Decls {
			decl, ok := f.(*ast.FuncDecl)
			if !ok || decl.Body == nil { // decl.Body can be nil for e.g. cgo
				continue
			}

			if stmtLimit > 0 {
				if stmts := parseStmts(decl.Body.List); stmts > stmtLimit {
					pass.Reportf(decl.Name.Pos(), "Function '%s' has too many statements (%d > %d)", decl.Name.Name, stmts, stmtLimit)
					continue
				}
			}

			if lineLimit > 0 {
				if lines := getLines(pass.Fset, decl, cmap.Filter(decl), ignoreComments); lines > lineLimit {
					pass.Reportf(decl.Name.Pos(), "Function '%s' is too long (%d > %d)", decl.Name.Name, lines, lineLimit)
				}
			}
		}
	}
}

func getLines(fset *token.FileSet, f *ast.FuncDecl, cmap ast.CommentMap, ignoreComments bool) int {
	lineCount := fset.Position(f.End()).Line - fset.Position(f.Pos()).Line - 1

	if !ignoreComments {
		return lineCount
	}

	var commentCount int

	for _, c := range cmap.Comments() {
		// If the CommentGroup's lines are inside the function
		// count how many comments are in the CommentGroup
		if (fset.Position(c.Pos()).Line > fset.Position(f.Pos()).Line) &&
			(fset.Position(c.End()).Line < fset.Position(f.End()).Line) {
			commentCount += len(c.List)
		}
	}

	return lineCount - commentCount
}

func parseStmts(s []ast.Stmt) (total int) {
	for _, v := range s {
		total++
		switch stmt := v.(type) {
		case *ast.BlockStmt:
			total += parseStmts(stmt.List) - 1
		case *ast.ForStmt, *ast.RangeStmt, *ast.IfStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			total += parseBodyListStmts(stmt)
		case *ast.CaseClause:
			total += parseStmts(stmt.Body)
		case *ast.AssignStmt:
			total += checkInlineFunc(stmt.Rhs[0])
		case *ast.GoStmt:
			total += checkInlineFunc(stmt.Call.Fun)
		case *ast.DeferStmt:
			total += checkInlineFunc(stmt.Call.Fun)
		}
	}
	return
}

func checkInlineFunc(stmt ast.Expr) int {
	if block, ok := stmt.(*ast.FuncLit); ok {
		return parseStmts(block.Body.List)
	}
	return 0
}

func parseBodyListStmts(t any) int {
	i := reflect.ValueOf(t).Elem().FieldByName(`Body`).Elem().FieldByName(`List`).Interface()
	return parseStmts(i.([]ast.Stmt))
}
