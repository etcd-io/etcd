// Package edit contains helpers for creating suggested fixes.
package edit

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/pattern"
)

// Ranger describes values that have a start and end position.
// In most cases these are either ast.Node or manually constructed ranges.
type Ranger interface {
	Pos() token.Pos
	End() token.Pos
}

// Range implements the Ranger interface.
type Range [2]token.Pos

func (r Range) Pos() token.Pos { return r[0] }
func (r Range) End() token.Pos { return r[1] }

// ReplaceWithString replaces a range with a string.
func ReplaceWithString(old Ranger, new string) analysis.TextEdit {
	return analysis.TextEdit{
		Pos:     old.Pos(),
		End:     old.End(),
		NewText: []byte(new),
	}
}

// ReplaceWithNode replaces a range with an AST node.
func ReplaceWithNode(fset *token.FileSet, old Ranger, new ast.Node) analysis.TextEdit {
	buf := &bytes.Buffer{}
	if err := format.Node(buf, fset, new); err != nil {
		panic("internal error: " + err.Error())
	}
	return analysis.TextEdit{
		Pos:     old.Pos(),
		End:     old.End(),
		NewText: buf.Bytes(),
	}
}

// ReplaceWithPattern replaces a range with the result of executing a pattern.
func ReplaceWithPattern(fset *token.FileSet, old Ranger, new pattern.Pattern, state pattern.State) analysis.TextEdit {
	r := pattern.NodeToAST(new.Root, state)
	buf := &bytes.Buffer{}
	format.Node(buf, fset, r)
	return analysis.TextEdit{
		Pos:     old.Pos(),
		End:     old.End(),
		NewText: buf.Bytes(),
	}
}

// Delete deletes a range of code.
func Delete(old Ranger) analysis.TextEdit {
	return analysis.TextEdit{
		Pos:     old.Pos(),
		End:     old.End(),
		NewText: nil,
	}
}

func Fix(msg string, edits ...analysis.TextEdit) analysis.SuggestedFix {
	return analysis.SuggestedFix{
		Message:   msg,
		TextEdits: edits,
	}
}

// Selector creates a new selector expression.
func Selector(x, sel string) *ast.SelectorExpr {
	return &ast.SelectorExpr{
		X:   &ast.Ident{Name: x},
		Sel: &ast.Ident{Name: sel},
	}
}
