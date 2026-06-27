package ifelse

import (
	"fmt"
	"go/ast"
	"go/token"
)

// Branch contains information about a branch within an if-else chain.
type Branch struct {
	BranchKind
	Call // The function called at the end for kind Panic or Exit.

	block []ast.Stmt
}

// BlockBranch gets the Branch of an [ast.BlockStmt].
func BlockBranch(block *ast.BlockStmt) Branch {
	blockLen := len(block.List)
	if blockLen == 0 {
		return Empty.Branch()
	}

	branch := StmtBranch(block.List[blockLen-1])
	branch.block = block.List
	return branch
}

// StmtBranch gets the Branch of an [ast.Stmt].
func StmtBranch(stmt ast.Stmt) Branch {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return Return.Branch()
	case *ast.BlockStmt:
		return BlockBranch(stmt)
	case *ast.BranchStmt:
		switch stmt.Tok {
		case token.BREAK:
			return Break.Branch()
		case token.CONTINUE:
			return Continue.Branch()
		case token.GOTO:
			return Goto.Branch()
		}
	case *ast.ExprStmt:
		fn, ok := ExprCall(stmt)
		if !ok {
			break
		}
		kind, ok := DeviatingFuncs[fn]
		if ok {
			return Branch{BranchKind: kind, Call: fn}
		}
	case *ast.EmptyStmt:
		return Empty.Branch()
	case *ast.LabeledStmt:
		return StmtBranch(stmt.Stmt)
	}
	return Regular.Branch()
}

// String returns a brief string representation.
func (b Branch) String() string {
	switch b.BranchKind {
	case Empty:
		return "{ }"
	case Regular:
		return "{ ... }"
	case Panic, Exit:
		return fmt.Sprintf("{ ... %v() }", b.Call)
	}
	return fmt.Sprintf("{ ... %v }", b.BranchKind)
}

// LongString returns a longer form string representation.
func (b Branch) LongString() string {
	switch b.BranchKind {
	case Panic, Exit:
		return fmt.Sprintf("call to %v function", b.Call)
	}
	return b.BranchKind.LongString()
}

// HasDecls returns whether the branch has any top-level declarations.
func (b Branch) HasDecls() bool {
	for _, stmt := range b.block {
		switch stmt := stmt.(type) {
		case *ast.DeclStmt:
			return true
		case *ast.AssignStmt:
			if stmt.Tok == token.DEFINE {
				return true
			}
		}
	}
	return false
}

// IsShort returns whether the branch is empty or consists of a single statement.
func (b Branch) IsShort() bool {
	switch len(b.block) {
	case 0:
		return true
	case 1:
		return isShortStmt(b.block[0])
	case 2:
		return isShortStmt(b.block[1])
	}
	return false
}

func isShortStmt(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.BlockStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.ForStmt, *ast.RangeStmt:
		return false
	}
	return true
}
