package syntax

import (
	"strings"
)

type Regexp struct {
	Pattern string
	Expr    Expr
}

type RegexpPCRE struct {
	Pattern string
	Expr    Expr

	Source    string
	Modifiers string
	Delim     [2]byte
}

func (re *RegexpPCRE) HasModifier(mod byte) bool {
	return strings.IndexByte(re.Modifiers, mod) >= 0
}

type Expr struct {
	// The operations that this expression performs. See `operation.go`.
	Op Operation

	Form Form

	_ [2]byte // Reserved

	// Pos describes a source location inside regexp pattern.
	Pos Position

	// Args is a list of sub-expressions of this expression.
	//
	// See Operation constants documentation to learn how to
	// interpret the particular expression args.
	Args []Expr

	// Value holds expression textual value.
	//
	// Usually, that value is identical to src[Begin():End()],
	// but this is not true for programmatically generated objects.
	Value string
}

// Begin returns expression leftmost offset.
func (e Expr) Begin() uint16 { return e.Pos.Begin }

// End returns expression rightmost offset.
func (e Expr) End() uint16 { return e.Pos.End }

// LastArg returns expression last argument.
//
// Should not be called on expressions that may have 0 arguments.
func (e Expr) LastArg() Expr {
	return e.Args[len(e.Args)-1]
}

type Operation byte

type Form byte
