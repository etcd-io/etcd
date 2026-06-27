package ifelse

import "go/ast"

// Target decides what line/column should be indicated by the rule in question.
type Target int

const (
	// TargetIf means the text refers to the "if".
	TargetIf Target = iota

	// TargetElse means the text refers to the "else".
	TargetElse
)

func (t Target) node(ifStmt *ast.IfStmt) ast.Node {
	switch t {
	case TargetIf:
		return ifStmt
	case TargetElse:
		return ifStmt.Else
	}
	panic("bad target")
}
