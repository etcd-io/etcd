package wsl

import (
	"go/ast"
)

// Cursor holds a list of statements and a pointer to where in the list we are.
// Each block gets a new cursor and can be used to check previous or coming
// statements.
type Cursor struct {
	currentIdx int
	statements []ast.Stmt
	checkType  CheckType
	// rbraceLine is the source line of the enclosing block's closing brace.
	// It is used to avoid false positives when checking for trailing comments:
	// an inline comment that sits on the same line as the closing brace belongs
	// to the brace itself, not to the last statement inside the block.
	// Zero means the enclosing block boundary is unknown (e.g. case clauses).
	rbraceLine int
}

// NewCursor creates a new cursor with a given list of statements.
func NewCursor(statements []ast.Stmt) *Cursor {
	return &Cursor{
		currentIdx: -1,
		statements: statements,
	}
}

// NewBlockCursor creates a cursor for the statements of a known block, recording
// the source line of the block's closing brace so that trailing-comment checks
// can ignore inline comments that sit on the brace itself.
func NewBlockCursor(statements []ast.Stmt, rbraceLine int) *Cursor {
	return &Cursor{
		currentIdx: -1,
		statements: statements,
		rbraceLine: rbraceLine,
	}
}

func (c *Cursor) SetChecker(ct CheckType) {
	c.checkType = ct
}

func (c *Cursor) NextNode() ast.Node {
	defer c.Save()()

	var nextNode ast.Node
	if c.Next() {
		nextNode = c.Stmt()
	}

	return nextNode
}

func (c *Cursor) Next() bool {
	if c.currentIdx >= len(c.statements)-1 {
		return false
	}

	c.currentIdx++

	return true
}

func (c *Cursor) Previous() bool {
	if c.currentIdx <= 0 {
		return false
	}

	c.currentIdx--

	return true
}

func (c *Cursor) PreviousNode() ast.Node {
	defer c.Save()()

	var previousNode ast.Node
	if c.Previous() {
		previousNode = c.Stmt()
	}

	return previousNode
}

func (c *Cursor) Stmt() ast.Stmt {
	return c.statements[c.currentIdx]
}

func (c *Cursor) Save() func() {
	idx := c.currentIdx

	return func() {
		c.currentIdx = idx
	}
}

func (c *Cursor) Len() int {
	return len(c.statements)
}

func (c *Cursor) Nth(n int) ast.Stmt {
	return c.statements[n]
}

func (c *Cursor) NthPrevious(n int) ast.Node {
	idx := c.currentIdx - n
	if idx < 0 {
		return nil
	}

	return c.statements[idx]
}
