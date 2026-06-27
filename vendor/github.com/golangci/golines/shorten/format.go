package shorten

import (
	"go/token"
	"log/slog"
	"reflect"

	"github.com/dave/dst"
	"github.com/golangci/golines/shorten/internal/annotation"
	"github.com/golangci/golines/shorten/internal/tags"
)

// formatFile formats the provided AST file starting at the top-level declarations.
func (s *Shortener) formatFile(file *dst.File) {
	for _, decl := range file.Decls {
		s.formatNode(decl)
	}
}

// formatNode formats the provided AST node.
// The appropriate helper function is called based on
// whether the node is a declaration, expression, statement, or spec.
func (s *Shortener) formatNode(node dst.Node) {
	switch n := node.(type) {
	case dst.Decl:
		s.logger.Debug("processing declaration", slog.Any("node", n))
		s.formatDecl(n)

	case dst.Expr:
		s.logger.Debug("processing expression", slog.Any("node", n))
		s.formatExpr(n, false, false)

	case dst.Stmt:
		s.logger.Debug("processing statement", slog.Any("node", n))
		s.formatStmt(n, false)

	case dst.Spec:
		s.logger.Debug("processing spec", slog.Any("node", n))
		s.formatSpec(n, false)

	default:
		s.logger.Debug(
			"got a node type that can't be shortened",
			slog.Any("node_type", reflect.TypeOf(n)),
		)
	}
}

// formatDecl formats an AST declaration node.
// These include function declarations, imports, and constants.
func (s *Shortener) formatDecl(decl dst.Decl) {
	switch d := decl.(type) {
	case *dst.FuncDecl:
		if d.Type != nil && d.Type.Params != nil && annotation.HasRecursive(d) {
			s.formatFieldList(d.Type.Params)
		}

		s.formatStmt(d.Body, false)

	case *dst.GenDecl:
		shouldShorten := annotation.Has(d)

		for _, spec := range d.Specs {
			s.formatSpec(spec, shouldShorten)
		}

	default:
		s.logger.Debug(
			"got a declaration type that can't be shortened",
			slog.Any("decl_type", reflect.TypeOf(d)),
		)
	}
}

// formatStmt formats an AST statement node.
// Among other examples, these include assignments, case clauses,
// for statements, if statements, and select statements.
//
//nolint:funlen // the number of statements is expected.
func (s *Shortener) formatStmt(stmt dst.Stmt, force bool) {
	stmtType := reflect.TypeOf(stmt)

	// Explicitly check for nil statements
	if reflect.ValueOf(stmt) == reflect.Zero(stmtType) {
		return
	}

	shouldShorten := force || annotation.Has(stmt)

	switch st := stmt.(type) {
	case *dst.AssignStmt:
		s.formatExprs(st.Rhs, shouldShorten, false)

	case *dst.BlockStmt:
		s.formatStmts(st.List, false)

	case *dst.CaseClause:
		if shouldShorten {
			for _, arg := range st.List {
				arg.Decorations().After = dst.NewLine

				s.formatExpr(arg, false, false)
			}
		}

		s.formatStmts(st.Body, false)

	case *dst.CommClause:
		s.formatStmts(st.Body, false)

	case *dst.DeclStmt:
		s.formatDecl(st.Decl)

	case *dst.DeferStmt:
		s.formatExpr(st.Call, shouldShorten, false)

	case *dst.ExprStmt:
		s.formatExpr(st.X, shouldShorten, false)

	case *dst.ForStmt:
		s.formatStmt(st.Body, false)

	case *dst.GoStmt:
		s.formatExpr(st.Call, shouldShorten, false)

	case *dst.IfStmt:
		s.formatExpr(st.Cond, shouldShorten, false)
		s.formatStmt(st.Body, false)

		if st.Init != nil {
			s.formatStmt(st.Init, shouldShorten)
		}

		if st.Else != nil {
			s.formatStmt(st.Else, shouldShorten)
		}

	case *dst.RangeStmt:
		s.formatStmt(st.Body, false)

	case *dst.ReturnStmt:
		s.formatExprs(st.Results, shouldShorten, false)

	case *dst.SelectStmt:
		s.formatStmt(st.Body, false)

	case *dst.SwitchStmt:
		s.formatStmt(st.Body, false)

		// Ignored: st.Init

		if st.Tag != nil {
			s.formatExpr(st.Tag, shouldShorten, false)
		}

	case *dst.TypeSwitchStmt:
		s.formatStmt(st.Body, false)

	case *dst.BadStmt, *dst.EmptyStmt, *dst.LabeledStmt,
		*dst.SendStmt, *dst.IncDecStmt, *dst.BranchStmt:
		// These statements are explicitly defined to improve switch cases exhaustiveness.
		// They may be handled in the future.
		if shouldShorten {
			s.logger.Debug(
				"got a statement type that is not shortened",
				slog.Any("stmt_type", stmtType),
			)
		}

	default:
		if shouldShorten {
			s.logger.Debug(
				"got an unknown statement type",
				slog.Any("stmt_type", stmtType),
			)
		}
	}
}

// formatExpr formats an AST expression node.
// These include uniary and binary expressions, function literals,
// and key/value pair statements, among others.
func (s *Shortener) formatExpr(expr dst.Expr, force, isChain bool) {
	shouldShorten := force || annotation.Has(expr)

	switch e := expr.(type) {
	case *dst.BinaryExpr:
		if (e.Op == token.LAND || e.Op == token.LOR) && shouldShorten {
			if e.Y.Decorations().Before == dst.NewLine {
				s.formatExpr(e.X, force, isChain)
			} else {
				e.Y.Decorations().Before = dst.NewLine
			}
		} else {
			s.formatExpr(e.X, shouldShorten, isChain)
			s.formatExpr(e.Y, shouldShorten, isChain)
		}

	case *dst.CallExpr:
		shortenChildArgs := shouldShorten || annotation.HasRecursive(e)

		_, ok := e.Fun.(*dst.SelectorExpr)

		if ok && shortenChildArgs &&
			s.config.ChainSplitDots && (isChain || chainLength(e) > 1) {
			e.Decorations().After = dst.NewLine

			s.formatExprs(e.Args, false, true)
			s.formatExpr(e.Fun, shouldShorten, true)
		} else {
			for i, arg := range e.Args {
				if shortenChildArgs {
					formatList(arg, i)
				}

				s.formatExpr(arg, false, isChain)
			}

			s.formatExpr(e.Fun, shouldShorten, isChain)
		}

	case *dst.CompositeLit:
		if shouldShorten || annotation.HasRecursive(e) {
			for i, element := range e.Elts {
				if i == 0 {
					element.Decorations().Before = dst.NewLine
				}

				element.Decorations().After = dst.NewLine
			}
		}

		s.formatExprs(e.Elts, false, isChain)

	case *dst.FuncLit:
		s.formatStmt(e.Body, false)

	case *dst.FuncType:
		if shouldShorten {
			s.formatFieldList(e.Params)
		}

	case *dst.InterfaceType:
		for _, method := range e.Methods.List {
			if annotation.Has(method) {
				s.formatExpr(method.Type, true, isChain)
			}
		}

	case *dst.KeyValueExpr:
		s.formatExpr(e.Value, shouldShorten, isChain)

	case *dst.SelectorExpr:
		s.formatExpr(e.X, shouldShorten, isChain)

	case *dst.StructType:
		if s.config.ReformatTags {
			tags.FormatStructTags(e.Fields)
		}

	case *dst.UnaryExpr:
		s.formatExpr(e.X, shouldShorten, isChain)

	default:
		if shouldShorten {
			s.logger.Debug(
				"got an expression type that can't be shortened",
				slog.Any("expr_type", reflect.TypeOf(e)),
			)
		}
	}
}

// formatSpec formats an AST spec node.
// These include type specifications, among other things.
func (s *Shortener) formatSpec(spec dst.Spec, force bool) {
	shouldShorten := annotation.Has(spec) || force

	switch sp := spec.(type) {
	case *dst.ValueSpec:
		s.formatExprs(sp.Values, shouldShorten, false)

	case *dst.TypeSpec:
		s.formatExpr(sp.Type, false, false)

	default:
		if shouldShorten {
			s.logger.Debug(
				"got a spec type that can't be shortened",
				slog.Any("spec_type", reflect.TypeOf(sp)),
			)
		}
	}
}

func (s *Shortener) formatStmts(stmts []dst.Stmt, force bool) {
	for _, stmt := range stmts {
		s.formatStmt(stmt, force)
	}
}

func (s *Shortener) formatExprs(exprs []dst.Expr, force, isChain bool) {
	for _, expr := range exprs {
		s.formatExpr(expr, force, isChain)
	}
}

// formatFieldList formats a field list in a function declaration.
func (s *Shortener) formatFieldList(fieldList *dst.FieldList) {
	for i, field := range fieldList.List {
		formatList(field, i)
	}
}

func formatList(node dst.Node, index int) {
	decorations := node.Decorations()

	if index == 0 {
		decorations.Before = dst.NewLine
	} else {
		decorations.Before = dst.None
	}

	decorations.After = dst.NewLine
}

// chainLength determines the length of the function call chain in an expression.
func chainLength(callExpr *dst.CallExpr) int {
	numCalls := 1
	currCall := callExpr

	for {
		selectorExpr, ok := currCall.Fun.(*dst.SelectorExpr)
		if !ok {
			break
		}

		currCall, ok = selectorExpr.X.(*dst.CallExpr)
		if !ok {
			break
		}

		numCalls++
	}

	return numCalls
}
