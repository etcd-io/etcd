package astwalk

import (
	"go/ast"
)

// DocCommentVisitor visits every doc-comment.
// Does not visit doc-comments for function-local definitions (types, etc).
// Also does not visit package doc-comment (file-level doc-comments).
type DocCommentVisitor interface {
	VisitDocComment(*ast.CommentGroup)
}

// FuncDeclVisitor visits every top-level function declaration.
type FuncDeclVisitor interface {
	walkerEvents
	VisitFuncDecl(*ast.FuncDecl)
}

// ExprVisitor visits every expression inside AST file.
type ExprVisitor interface {
	walkerEvents
	VisitExpr(ast.Expr)
}

// LocalExprVisitor visits every expression inside function body.
type LocalExprVisitor interface {
	walkerEvents
	VisitLocalExpr(ast.Expr)
}

// StmtListVisitor visits every statement list inside function body.
// This includes block statement bodies as well as implicit blocks
// introduced by case clauses and alike.
type StmtListVisitor interface {
	walkerEvents
	VisitStmtList(ast.Node, []ast.Stmt)
}

// StmtVisitor visits every statement inside function body.
type StmtVisitor interface {
	walkerEvents
	VisitStmt(ast.Stmt)
}

// TypeExprVisitor visits every type describing expression.
// It also traverses struct types and interface types to run
// checker over their fields/method signatures.
type TypeExprVisitor interface {
	walkerEvents
	VisitTypeExpr(ast.Expr)
}

// LocalCommentVisitor visits every comment inside function body.
type LocalCommentVisitor interface {
	walkerEvents
	VisitLocalComment(*ast.CommentGroup)
}

// CommentVisitor visits every comment.
type CommentVisitor interface {
	walkerEvents
	VisitComment(*ast.CommentGroup)
}

// walkerEvents describes common hooks available for most visitor types.
type walkerEvents interface {
	// EnterFile is called for every file that is about to be traversed.
	// If false is returned, file is not visited.
	EnterFile(*ast.File) bool

	// EnterFunc is called for every function declaration that is about
	// to be traversed. If false is returned, function is not visited.
	EnterFunc(*ast.FuncDecl) bool

	skipChilds() bool
}
