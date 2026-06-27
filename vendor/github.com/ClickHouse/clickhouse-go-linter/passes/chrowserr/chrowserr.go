package chrowserr

import (
	"go/ast"
	"go/token"
	"log"
	"os"
	"strconv"

	"github.com/ClickHouse/clickhouse-go-linter/internal/util"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

type analyzer struct {
	// if true, report valid usages and log spurious but valid cases.
	debug bool
}

func NewAnalyzer() *analysis.Analyzer {
	debug, _ := strconv.ParseBool(os.Getenv("CH_GO_LINTER_DEBUG"))
	a := analyzer{
		debug: debug,
	}
	return &analysis.Analyzer{
		Name:     "chrowserrcheck",
		Doc:      "chrowserrcheck checks whether ClickHouse driver Rows.Err is called after Rows.Next()",
		Run:      a.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func (a *analyzer) run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		var body *ast.BlockStmt
		switch fn := n.(type) {
		case *ast.FuncDecl:
			body = fn.Body
		case *ast.FuncLit:
			body = fn.Body
		}

		if body == nil {
			return
		}
		a.checkFunc(pass, body)
	})

	return nil, nil
}

// rowsUsage tracks Next/Err usage observations for a single Rows variable within a function.
type rowsUsage struct {
	nextPos   token.Pos
	errCalled bool
}

func (r *rowsUsage) report(varName string, pass *analysis.Pass, debug bool) {
	if r.nextPos == token.NoPos {
		// no usage of rows.Next()
		return
	}
	// rows.Next() was called
	if !r.errCalled {
		pass.Reportf(r.nextPos,
			"clickhouse %s.Err() must be checked after %s.Next()",
			varName, varName)
	} else if debug {
		// for dev purpose - list valid usages
		pass.Reportf(r.nextPos,
			"clickhouse %s.Err() is properly called after %s.Next() [valid]",
			varName, varName)
	}
}

// checkFunc analyzes a single function/closure body.
// It does a single-pass collection of Next() and Err() calls,
// it does not descend into nested closures (they are handled as separate units by the Preorder visitor above).
func (a *analyzer) checkFunc(pass *analysis.Pass, body *ast.BlockStmt) {
	usages := map[string]*rowsUsage{}
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		// don't descend into nested closures
		if n != body {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
		}

		// if a tracked var appears in an assignment, it means it is re-assigned - run lint report and flush var from usages.
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				name := util.IdentName(lhs)
				if name == "" {
					continue
				}
				if s, ok := usages[name]; ok {
					s.report(name, pass, a.debug)
				}
				delete(usages, name)
			}
			// let ast.Inspect continue
			return true
		}

		// handle CH driver method calls Next() and Err() - skip everything else
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		varName := util.IdentName(sel.X)
		if varName == "" {
			return true
		}
		if !util.IsChObj(pass, sel.X, "Rows") {
			return true
		}

		// call is a CH driver Rows method call
		switch sel.Sel.Name {
		case "Next":
			if _, exists := usages[varName]; !exists {
				usages[varName] = &rowsUsage{nextPos: call.Pos()}
			} else if a.debug {
				log.Printf("Rows.Next() is written multiple times with no re-assignment. Valid but rare usage. If this observation is not correct, it is a bug in this linter library. Please reach out to maintainer.")
				// in particular could be a bug in the re-assignment detection
			}
		case "Err":
			if s, exists := usages[varName]; exists {
				s.errCalled = true
				s.report(varName, pass, a.debug)
				delete(usages, varName)
			} else if a.debug {
				log.Printf("%s.Err() is called on ClickHouse Rows %s but %s.Next() was never called. Valid but unexpected usage. If this observation is not correct, it is a bug in this linter library. Please reach out to maintainer.",
					varName, varName, varName)
				// in particular could be a bug in the re-assignment detection
			}

		}

		return true
	})

	// remaining usages that were not flushed (misusages only, as correct usages are flushed already)
	for varName, s := range usages {
		s.report(varName, pass, a.debug)
	}
}
