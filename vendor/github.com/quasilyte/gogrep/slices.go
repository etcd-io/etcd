package gogrep

import (
	"go/ast"
	"go/token"
)

type NodeSliceKind uint32

const (
	ExprNodeSlice NodeSliceKind = iota
	StmtNodeSlice
	FieldNodeSlice
	IdentNodeSlice
	SpecNodeSlice
	DeclNodeSlice
)

type NodeSlice struct {
	Kind NodeSliceKind

	exprSlice  []ast.Expr
	stmtSlice  []ast.Stmt
	fieldSlice []*ast.Field
	identSlice []*ast.Ident
	specSlice  []ast.Spec
	declSlice  []ast.Decl
}

func (s *NodeSlice) GetExprSlice() []ast.Expr    { return s.exprSlice }
func (s *NodeSlice) GetStmtSlice() []ast.Stmt    { return s.stmtSlice }
func (s *NodeSlice) GetFieldSlice() []*ast.Field { return s.fieldSlice }
func (s *NodeSlice) GetIdentSlice() []*ast.Ident { return s.identSlice }
func (s *NodeSlice) GetSpecSlice() []ast.Spec    { return s.specSlice }
func (s *NodeSlice) GetDeclSlice() []ast.Decl    { return s.declSlice }

func (s *NodeSlice) assignExprSlice(xs []ast.Expr) {
	s.Kind = ExprNodeSlice
	s.exprSlice = xs
}

func (s *NodeSlice) assignStmtSlice(xs []ast.Stmt) {
	s.Kind = StmtNodeSlice
	s.stmtSlice = xs
}

func (s *NodeSlice) assignFieldSlice(xs []*ast.Field) {
	s.Kind = FieldNodeSlice
	s.fieldSlice = xs
}

func (s *NodeSlice) assignIdentSlice(xs []*ast.Ident) {
	s.Kind = IdentNodeSlice
	s.identSlice = xs
}

func (s *NodeSlice) assignSpecSlice(xs []ast.Spec) {
	s.Kind = SpecNodeSlice
	s.specSlice = xs
}

func (s *NodeSlice) assignDeclSlice(xs []ast.Decl) {
	s.Kind = DeclNodeSlice
	s.declSlice = xs
}

func (s *NodeSlice) Len() int {
	switch s.Kind {
	case ExprNodeSlice:
		return len(s.exprSlice)
	case StmtNodeSlice:
		return len(s.stmtSlice)
	case FieldNodeSlice:
		return len(s.fieldSlice)
	case IdentNodeSlice:
		return len(s.identSlice)
	case SpecNodeSlice:
		return len(s.specSlice)
	default:
		return len(s.declSlice)
	}
}

func (s *NodeSlice) At(i int) ast.Node {
	switch s.Kind {
	case ExprNodeSlice:
		return s.exprSlice[i]
	case StmtNodeSlice:
		return s.stmtSlice[i]
	case FieldNodeSlice:
		return s.fieldSlice[i]
	case IdentNodeSlice:
		return s.identSlice[i]
	case SpecNodeSlice:
		return s.specSlice[i]
	default:
		return s.declSlice[i]
	}
}

func (s *NodeSlice) SliceInto(dst *NodeSlice, i, j int) {
	switch s.Kind {
	case ExprNodeSlice:
		dst.assignExprSlice(s.exprSlice[i:j])
	case StmtNodeSlice:
		dst.assignStmtSlice(s.stmtSlice[i:j])
	case FieldNodeSlice:
		dst.assignFieldSlice(s.fieldSlice[i:j])
	case IdentNodeSlice:
		dst.assignIdentSlice(s.identSlice[i:j])
	case SpecNodeSlice:
		dst.assignSpecSlice(s.specSlice[i:j])
	default:
		dst.assignDeclSlice(s.declSlice[i:j])
	}
}

func (s *NodeSlice) Pos() token.Pos {
	switch s.Kind {
	case ExprNodeSlice:
		return s.exprSlice[0].Pos()
	case StmtNodeSlice:
		return s.stmtSlice[0].Pos()
	case FieldNodeSlice:
		return s.fieldSlice[0].Pos()
	case IdentNodeSlice:
		return s.identSlice[0].Pos()
	case SpecNodeSlice:
		return s.specSlice[0].Pos()
	default:
		return s.declSlice[0].Pos()
	}
}

func (s *NodeSlice) End() token.Pos {
	switch s.Kind {
	case ExprNodeSlice:
		return s.exprSlice[len(s.exprSlice)-1].End()
	case StmtNodeSlice:
		return s.stmtSlice[len(s.stmtSlice)-1].End()
	case FieldNodeSlice:
		return s.fieldSlice[len(s.fieldSlice)-1].End()
	case IdentNodeSlice:
		return s.identSlice[len(s.identSlice)-1].End()
	case SpecNodeSlice:
		return s.specSlice[len(s.specSlice)-1].End()
	default:
		return s.declSlice[len(s.declSlice)-1].End()
	}
}
