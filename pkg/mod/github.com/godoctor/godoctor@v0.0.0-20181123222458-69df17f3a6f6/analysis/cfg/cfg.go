// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cfg provides intraprocedural control flow graphs (CFGs) with
// statement-level granularity, i.e., CFGs whose nodes correspond 1-1 to the
// Stmt nodes from an abstract syntax tree.
package cfg

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// This package can be used to construct a control flow graph from an abstract syntax tree (go/ast).
// This is done by traversing a list of statements (likely from a block)
// depth-first and creating an adjacency list, implemented as a map of blocks.
// Adjacent blocks are stored as predecessors and successors separately for
// control flow information. Any defers encountered while traversing the ast
// will be added to a slice that can be accessed from CFG. Their behavior is such
// that they may or may not be flowed to, potentially multiple times, after Exit.
// This behavior is dependant upon in what control structure they were found,
// i.e. if/for body may never be flowed to.

// TODO(you): defers are lazily done currently. If needed, could likely use a more robust
//  implementation wherein they are represented as a graph after Exit.
// TODO(reed): closures, go func() ?

// CFG defines a control flow graph with statement-level granularity, in which
// there is a 1-1 correspondence between a block in the CFG and an ast.Stmt.
type CFG struct {
	// Sentinel nodes for single-entry, single-exit CFG. Not in original AST.
	Entry, Exit *ast.BadStmt
	// All defers found in CFG, disjoint from blocks. May be flowed to after Exit.
	Defers []*ast.DeferStmt
	blocks map[ast.Stmt]*block
}

type block struct {
	stmt  ast.Stmt
	preds []ast.Stmt
	succs []ast.Stmt
}

// FromStmts returns the control-flow graph for the given sequence of statements.
func FromStmts(s []ast.Stmt) *CFG {
	return newBuilder().build(s)
}

// FromFunc is a convenience function for creating a CFG from a given function declaration.
func FromFunc(f *ast.FuncDecl) *CFG {
	return FromStmts(f.Body.List)
}

// Preds returns a slice of all immediate predecessors for the given statement.
// May include Entry node.
func (c *CFG) Preds(s ast.Stmt) []ast.Stmt {
	return c.blocks[s].preds
}

// Succs returns a slice of all immediate successors to the given statement.
// May include Exit node.
func (c *CFG) Succs(s ast.Stmt) []ast.Stmt {
	return c.blocks[s].succs
}

// Blocks returns a slice of all blocks in a CFG, including the Entry and Exit nodes.
// The blocks are roughly in the order they appear in the source code.
func (c *CFG) Blocks() []ast.Stmt {
	blocks := make([]ast.Stmt, 0, len(c.blocks))
	for s := range c.blocks {
		blocks = append(blocks, s)
	}
	return blocks
}

// type for sorting statements by their starting positions in the source code
type stmtSlice []ast.Stmt

func (n stmtSlice) Len() int      { return len(n) }
func (n stmtSlice) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n stmtSlice) Less(i, j int) bool {
	return n[i].Pos() < n[j].Pos()
}

func (c *CFG) Sort(stmts []ast.Stmt) {
	sort.Sort(stmtSlice(stmts))
}

func (c *CFG) PrintDot(f io.Writer, fset *token.FileSet, addl func(n ast.Stmt) string) {
	fmt.Fprintf(f, `digraph mgraph {
mode="heir";
splines="ortho";

`)
	blocks := c.Blocks()
	c.Sort(blocks)
	for _, from := range blocks {
		succs := c.Succs(from)
		c.Sort(succs)
		for _, to := range succs {
			fmt.Fprintf(f, "\t\"%s\" -> \"%s\"\n",
				c.printVertex(from, fset, addl(from)),
				c.printVertex(to, fset, addl(to)))
		}
	}
	fmt.Fprintf(f, "}\n")
}

func (c *CFG) printVertex(stmt ast.Stmt, fset *token.FileSet, addl string) string {
	switch stmt {
	case c.Entry:
		return "ENTRY"
	case c.Exit:
		return "EXIT"
	case nil:
		return ""
	}
	addl = strings.Replace(addl, "\n", "\\n", -1)
	if addl != "" {
		addl = "\\n" + addl
	}
	return fmt.Sprintf("%s - line %d%s",
		astutil.NodeDescription(stmt),
		fset.Position(stmt.Pos()).Line,
		addl)
}

type builder struct {
	blocks      map[ast.Stmt]*block
	prev        []ast.Stmt        // blocks to hook up to current block
	branches    []*ast.BranchStmt // accumulated branches from current inner blocks
	entry, exit *ast.BadStmt      // single-entry, single-exit nodes
	defers      []*ast.DeferStmt  // all defers encountered
}

func newBuilder() *builder {
	// The ENTRY and EXIT nodes are given positions -2 and -1 so cfg.Sort
	// will work correct: ENTRY will always be first, followed by EXIT,
	// followed by the other CFG nodes.
	return &builder{
		blocks: map[ast.Stmt]*block{},
		entry:  &ast.BadStmt{-2, -2},
		exit:   &ast.BadStmt{-1, -1},
	}
}

// build runs buildBlock on the given block (traversing nested statements), and
// adds entry and exit nodes.
func (b *builder) build(s []ast.Stmt) *CFG {
	b.prev = []ast.Stmt{b.entry}
	b.buildBlock(s)
	b.addSucc(b.exit)

	return &CFG{
		blocks: b.blocks,
		Entry:  b.entry,
		Exit:   b.exit,
		Defers: b.defers,
	}
}

// addSucc adds a control flow edge from all previous blocks to the block for
// the given statement.
func (b *builder) addSucc(current ast.Stmt) {
	cur := b.block(current)

	for _, p := range b.prev {
		p := b.block(p)
		p.succs = appendNoDuplicates(p.succs, cur.stmt)
		cur.preds = appendNoDuplicates(cur.preds, p.stmt)
	}
}

func appendNoDuplicates(list []ast.Stmt, stmt ast.Stmt) []ast.Stmt {
	for _, s := range list {
		if s == stmt {
			return list
		}
	}
	return append(list, stmt)
}

// block returns a block for the given statement, creating one and inserting it
// into the CFG if it doesn't already exist.
func (b *builder) block(s ast.Stmt) *block {
	bl, ok := b.blocks[s]
	if !ok {
		bl = &block{stmt: s}
		b.blocks[s] = bl
	}
	return bl
}

// buildStmt adds the given statement and all nested statements to the control
// flow graph under construction. Upon completion, b.prev is set to all
// control flow exits generated from traversing cur.
func (b *builder) buildStmt(cur ast.Stmt) {
	if dfr, ok := cur.(*ast.DeferStmt); ok {
		b.defers = append(b.defers, dfr)
		return // never flow to or from defer
	}

	// Each buildXxx method will flow the previous blocks to itself appropiately and also
	// set the appropriate blocks to flow from at the end of the method.
	switch cur := cur.(type) {
	case *ast.BlockStmt:
		b.buildBlock(cur.List)
	case *ast.IfStmt:
		b.buildIf(cur)
	case *ast.ForStmt, *ast.RangeStmt:
		b.buildLoop(cur)
	case *ast.SwitchStmt, *ast.SelectStmt, *ast.TypeSwitchStmt:
		b.buildSwitch(cur)
	case *ast.BranchStmt:
		b.buildBranch(cur)
	case *ast.LabeledStmt:
		b.addSucc(cur)
		b.prev = []ast.Stmt{cur}
		b.buildStmt(cur.Stmt)
	case *ast.ReturnStmt:
		b.addSucc(cur)
		b.prev = []ast.Stmt{cur}
		b.addSucc(b.exit)
		b.prev = nil
	default: // most statements have straight-line control flow
		b.addSucc(cur)
		b.prev = []ast.Stmt{cur}
	}
}

func (b *builder) buildBranch(br *ast.BranchStmt) {
	b.addSucc(br)
	b.prev = []ast.Stmt{br}

	switch br.Tok {
	case token.FALLTHROUGH:
		// successors handled in buildSwitch, so skip this here
	case token.GOTO:
		b.addSucc(br.Label.Obj.Decl.(ast.Stmt)) // flow to label
	case token.BREAK, token.CONTINUE:
		b.branches = append(b.branches, br) // to handle at switch/for/etc level
	}
	b.prev = nil // successors handled elsewhere
}

func (b *builder) buildIf(f *ast.IfStmt) {
	if f.Init != nil {
		b.addSucc(f.Init)
		b.prev = []ast.Stmt{f.Init}
	}
	b.addSucc(f)

	b.prev = []ast.Stmt{f}
	b.buildBlock(f.Body.List) // build then

	ctrlExits := b.prev // aggregate of b.prev from each condition

	switch s := f.Else.(type) {
	case *ast.BlockStmt: // build else
		b.prev = []ast.Stmt{f}
		b.buildBlock(s.List)
		ctrlExits = append(ctrlExits, b.prev...)
	case *ast.IfStmt: // build else if
		b.prev = []ast.Stmt{f}
		b.addSucc(s)
		b.buildIf(s)
		ctrlExits = append(ctrlExits, b.prev...)
	case nil: // no else
		ctrlExits = append(ctrlExits, f)
	}

	b.prev = ctrlExits
}

// buildLoop builds CFG blocks for a ForStmt or RangeStmt, including nested statements.
// Upon return, b.prev set to for and any appropriate breaks.
func (b *builder) buildLoop(stmt ast.Stmt) {
	// flows as such (range same w/o init & post):
	// previous -> [ init -> ] for -> body -> [ post -> ] for -> next

	var post ast.Stmt = stmt // post in for loop, or for stmt itself; body flows to this

	switch stmt := stmt.(type) {
	case *ast.ForStmt:
		if stmt.Init != nil {
			b.addSucc(stmt.Init)
			b.prev = []ast.Stmt{stmt.Init}
		}
		b.addSucc(stmt)

		if stmt.Post != nil {
			post = stmt.Post
			b.prev = []ast.Stmt{post}
			b.addSucc(stmt)
		}

		b.prev = []ast.Stmt{stmt}
		b.buildBlock(stmt.Body.List)
	case *ast.RangeStmt:
		b.addSucc(stmt)
		b.prev = []ast.Stmt{stmt}
		b.buildBlock(stmt.Body.List)
	}

	b.addSucc(post)

	ctrlExits := []ast.Stmt{stmt}

	// handle any branches; if no label or for me: handle and remove from branches.
	for i := 0; i < len(b.branches); i++ {
		br := b.branches[i]
		if br.Label == nil || br.Label.Obj.Decl.(*ast.LabeledStmt).Stmt == stmt {
			switch br.Tok { // can only be one of these two cases
			case token.CONTINUE:
				b.prev = []ast.Stmt{br}
				b.addSucc(post) // connect to .Post statement if present, for stmt otherwise
			case token.BREAK:
				ctrlExits = append(ctrlExits, br)
			}
			b.branches = append(b.branches[:i], b.branches[i+1:]...)
			i-- // removed in place, so go back to this i
		}
	}

	b.prev = ctrlExits // for stmt and any appropriate break statements
}

// buildSwitch builds a multi-way branch statement, i.e. switch, type switch or select.
// Upon return, each case's control exits set as b.prev.
func (b *builder) buildSwitch(sw ast.Stmt) {
	// composition of statement sw:
	//
	//    sw: *ast.SwitchStmt || *ast.TypeSwitchStmt || *ast.SelectStmt
	//      Body.List: []*ast.CaseClause || []ast.CommClause
	//        clause: []ast.Stmt

	var cases []ast.Stmt // case 1:, case 2:, ...

	switch sw := sw.(type) {
	case *ast.SwitchStmt: // i.e. switch [ x := 0; ] [ x ] { }
		if sw.Init != nil {
			b.addSucc(sw.Init)
			b.prev = []ast.Stmt{sw.Init}
		}
		b.addSucc(sw)
		b.prev = []ast.Stmt{sw}

		cases = sw.Body.List
	case *ast.TypeSwitchStmt: // i.e. switch [ x := 0; ] t := x.(type) { }
		if sw.Init != nil {
			b.addSucc(sw.Init)
			b.prev = []ast.Stmt{sw.Init}
		}
		b.addSucc(sw)
		b.prev = []ast.Stmt{sw}
		b.addSucc(sw.Assign)
		b.prev = []ast.Stmt{sw.Assign}

		cases = sw.Body.List
	case *ast.SelectStmt: // i.e. select { }
		b.addSucc(sw)
		b.prev = []ast.Stmt{sw}

		cases = sw.Body.List
	}

	var caseExits []ast.Stmt // aggregate of b.prev's resulting from each case
	swPrev := b.prev         // save for each case's previous; Switch or Assign
	var ft *ast.BranchStmt   // fallthrough to handle from previous case, if any
	defaultCase := false

	for _, clause := range cases {
		b.prev = swPrev
		b.addSucc(clause)
		b.prev = []ast.Stmt{clause}
		if ft != nil {
			b.prev = append(b.prev, ft)
		}

		var caseBody []ast.Stmt

		// both of the following cases are guaranteed in spec
		switch clause := clause.(type) {
		case *ast.CaseClause: // i.e. case: [expr,expr,...]:
			if clause.List == nil {
				defaultCase = true
			}
			caseBody = clause.Body
		case *ast.CommClause: // i.e. case c <- chan:
			if clause.Comm == nil {
				defaultCase = true
			} else {
				b.addSucc(clause.Comm)
				b.prev = []ast.Stmt{clause.Comm}
			}
			caseBody = clause.Body
		}

		b.buildBlock(caseBody)

		if ft = fallThrough(caseBody); ft == nil {
			caseExits = append(caseExits, b.prev...)
		}
	}

	if !defaultCase {
		caseExits = append(caseExits, swPrev...)
	}

	// handle any breaks that are unlabeled or for me
	for i := 0; i < len(b.branches); i++ {
		br := b.branches[i]
		if br.Tok == token.BREAK && (br.Label == nil || br.Label.Obj.Decl.(*ast.LabeledStmt).Stmt == sw) {
			caseExits = append(caseExits, br)
			b.branches = append(b.branches[:i], b.branches[i+1:]...)
			i-- // we removed in place, so go back to this index
		}
	}

	b.prev = caseExits // control exits of each case and breaks
}

// fallThrough returns the fallthrough stmt at the end of stmts, if one exists,
// and nil otherwise.
func fallThrough(stmts []ast.Stmt) *ast.BranchStmt {
	if len(stmts) < 1 {
		return nil
	}

	// fallthrough can only be last statement in clause (possibly labeled)
	ft := stmts[len(stmts)-1]

	for { // recursively descend LabeledStmts.
		switch s := ft.(type) {
		case *ast.BranchStmt:
			if s.Tok == token.FALLTHROUGH {
				return s
			}
		case *ast.LabeledStmt:
			ft = s.Stmt
			continue
		}
		break
	}
	return nil
}

// buildBlock iterates over a slice of statements (typically the statements
// from an ast.BlockStmt), adding them successively to the CFG.  Upon return,
// b.prev is set to the control exits of the last statement.
func (b *builder) buildBlock(block []ast.Stmt) {
	for _, stmt := range block {
		b.buildStmt(stmt)
	}
}
