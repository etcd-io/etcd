package wsl

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"math"
	"slices"

	"golang.org/x/tools/go/analysis"
)

const (
	messageMissingWhitespaceAbove = "missing whitespace above this line"
	messageMissingWhitespaceBelow = "missing whitespace below this line"
	messageRemoveWhitespace       = "unnecessary whitespace"
)

type fixRange struct {
	fixRangeStart token.Pos
	fixRangeEnd   token.Pos
	fix           []byte
}

type issue struct {
	message string
	// We can report multiple fixes at the same position. This happens e.g. when
	// we force error cuddling but the error assignment is already cuddled.
	// See `checkError` for examples.
	fixRanges []fixRange
}

type WSL struct {
	file     *ast.File
	fset     *token.FileSet
	typeInfo *types.Info
	issues   map[token.Pos]issue
	config   *Configuration
}

func New(file *ast.File, pass *analysis.Pass, cfg *Configuration) *WSL {
	return &WSL{
		fset:     pass.Fset,
		file:     file,
		typeInfo: pass.TypesInfo,
		issues:   make(map[token.Pos]issue),
		config:   cfg,
	}
}

// Run will run analysis on the file and pass passed to the constructor. It's
// typically only supposed to be used by [analysis.Analyzer].
func (w *WSL) Run() {
	ast.Inspect(w.file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			w.checkBlock(node.Body, NewCursor([]ast.Stmt{}))
		case *ast.FuncLit:
			w.checkBlock(node.Body, NewCursor([]ast.Stmt{}))
		}

		return true
	})
}

func (w *WSL) checkStmt(stmt ast.Stmt, cursor *Cursor) {
	//nolint:gocritic // This is not commented out code, it's examples
	switch s := stmt.(type) {
	// if a {} else if b {} else {}
	case *ast.IfStmt:
		w.checkIf(s, cursor, false)
	// for {} / for a; b; c {}
	case *ast.ForStmt:
		w.checkFor(s, cursor)
	// for _, _ = range a {}
	case *ast.RangeStmt:
		w.checkRange(s, cursor)
	// switch {} // switch a {}
	case *ast.SwitchStmt:
		w.checkSwitch(s, cursor)
	// switch a.(type) {}
	case *ast.TypeSwitchStmt:
		w.checkTypeSwitch(s, cursor)
	// return a
	case *ast.ReturnStmt:
		w.checkReturn(s, cursor)
	// continue / break
	case *ast.BranchStmt:
		w.checkBranch(s, cursor)
	// var a
	case *ast.DeclStmt:
		w.checkDeclStmt(s, cursor)
	// a := a
	case *ast.AssignStmt:
		w.checkAssign(s, cursor)
	// a++ / a--
	case *ast.IncDecStmt:
		w.checkIncDec(s, cursor)
	// defer func() {}
	case *ast.DeferStmt:
		w.checkDefer(s, cursor)
	// go func() {}
	case *ast.GoStmt:
		w.checkGo(s, cursor)
	// e.g. someFn()
	case *ast.ExprStmt:
		w.checkExprStmt(s, cursor)
	// case:
	case *ast.CaseClause:
		w.checkCaseClause(s, cursor)
	// case:
	case *ast.CommClause:
		w.checkCommClause(s, cursor)
	// { }
	case *ast.BlockStmt:
		w.checkBlock(s, cursor)
	// select { }
	case *ast.SelectStmt:
		w.checkSelect(s, cursor)
	// ch <- ...
	case *ast.SendStmt:
		w.checkSend(s, cursor)
	// LABEL:
	case *ast.LabeledStmt:
		w.checkLabel(s, cursor)
	case *ast.EmptyStmt:
	default:
	}
}

func (w *WSL) checkBodyBlock(block *ast.BlockStmt) {
	w.walkBody(NewBlockCursor(block.List, w.lineFor(block.Rbrace)))
}

func (w *WSL) checkBodyStmts(stmts []ast.Stmt) {
	w.walkBody(NewCursor(stmts))
}

func (w *WSL) walkBody(cursor *Cursor) {
	for cursor.Next() {
		w.checkStmt(cursor.Stmt(), cursor)
	}
}

func (w *WSL) checkCuddlingBlock(
	stmt ast.Node,
	blockList []ast.Stmt,
	allowedIdents []*ast.Ident,
	cursor *Cursor,
) {
	var firstBlockStmt ast.Node
	if len(blockList) > 0 {
		firstBlockStmt = blockList[0]
	}

	w.checkCuddlingMaxAllowed(stmt, firstBlockStmt, allowedIdents, cursor, true)
}

func (w *WSL) checkCuddling(stmt ast.Node, cursor *Cursor, enforceLimit bool) {
	w.checkCuddlingMaxAllowed(stmt, nil, []*ast.Ident{}, cursor, enforceLimit)
}

func (w *WSL) checkCuddlingMaxAllowed(
	stmt ast.Node,
	firstBlockStmt ast.Node,
	allowedIdents []*ast.Ident,
	cursor *Cursor,
	enforceLimit bool,
) {
	if _, ok := cursor.Stmt().(*ast.LabeledStmt); ok {
		return
	}

	previousNode := cursor.PreviousNode()
	numStmtsAbove := w.numberOfStatementsAbove(cursor)
	previousIdents := w.identsFromNode(previousNode, true)

	// If we don't have any statements above, we only care about potential error
	// cuddling (for if statements) so check that.
	if numStmtsAbove == 0 {
		w.checkError(numStmtsAbove, stmt, previousNode, cursor)
		return
	}

	if w.isLockOrUnlock(stmt, previousNode) {
		return
	}

	_, currIsDefer := stmt.(*ast.DeferStmt)
	_, currIsGo := stmt.(*ast.GoStmt)
	currRelaxesPrevType := currIsDefer || currIsGo

	// We're cuddled but not with an assign, declare, increment/decrement and
	// we're not a statement with relaxed check.
	if !isAssignDeclOrIncDec(previousNode) && !currRelaxesPrevType {
		w.addErrorInvalidTypeCuddle(cursor.Stmt().Pos(), cursor.checkType)
		return
	}

	targetIdents := w.cuddleTargetIdents(stmt, firstBlockStmt, allowedIdents)

	if !identsIntersect(previousIdents, targetIdents) {
		w.addErrorNoIntersection(stmt.Pos(), cursor.checkType)
		return
	}

	if !enforceLimit {
		return
	}

	// CheckErr always have precedence, allowing any permutation of max allowed
	// cuddled statements and CheckCuddleGroup to be configured but still
	// respect the requirement to use the idiomatic err checking and never
	// insert a newline between the err and if.
	_, errEnabled := w.config.Checks[CheckErr]
	errIdent := w.isErrNotNilCheck(stmt)

	if errEnabled && errIdent != nil && identsIntersect([]*ast.Ident{errIdent}, previousIdents) {
		if numStmtsAbove > 1 {
			if errorNode := cursor.NthPrevious(1); errorNode != nil {
				w.addErrorTooManyStatements(errorNode.Pos(), cursor.checkType)
			}
		}

		return
	}

	if _, ok := w.config.Checks[CheckCuddleGroup]; ok {
		// Treat the cuddled chain as a unit: any non-sharing stmt or too
		// many sharing stmts separates the whole group from the trigger.
		sharedCount, stoppedAtNonIntersection := w.countValidCuddledStatements(
			targetIdents,
			cursor,
			currRelaxesPrevType,
			math.MaxInt,
		)
		if stoppedAtNonIntersection || sharedCount > w.config.CuddleMaxStatements {
			w.addErrorTooManyStatements(cursor.Stmt().Pos(), cursor.checkType)
		}

		return
	}

	allowedCount, stoppedAtNonIntersection := w.countValidCuddledStatements(
		targetIdents,
		cursor,
		currRelaxesPrevType,
		w.config.CuddleMaxStatements,
	)
	if numStmtsAbove <= allowedCount {
		return
	}

	errorNode := cursor.NthPrevious(allowedCount)
	if errorNode == nil {
		return
	}

	if stoppedAtNonIntersection {
		w.addErrorVariableNotShared(errorNode.Pos(), cursor.checkType)
	} else {
		w.addErrorTooManyStatements(errorNode.Pos(), cursor.checkType)
	}
}

// cuddleTargetIdents builds the combined set of identifiers that a cuddled
// statement may reference. This respects AllowWholeBlock and AllowFirstInBlock.
func (w *WSL) cuddleTargetIdents(
	stmt ast.Node,
	firstBlockStmt ast.Node,
	allowedIdents []*ast.Ident,
) []*ast.Ident {
	var idents []*ast.Ident

	if w.config.AllowWholeBlock {
		idents = append(idents, w.identsFromNode(stmt, false)...)
	} else {
		idents = append(idents, w.identsFromNode(stmt, true)...)

		if w.config.AllowFirstInBlock {
			idents = append(idents, w.identsFromNode(firstBlockStmt, true)...)
		}
	}

	idents = append(idents, allowedIdents...)

	return idents
}

// countValidCuddledStatements walks backwards from the cursor and counts how
// many consecutive cuddled statements have a valid intersection with
// targetIdents. It stops at the first non-intersecting statement or when the
// limit is reached. Returns the count and whether the walk stopped because a
// non-intersecting statement was found (as opposed to the limit).
//
// When allowAnyStmtType is true any statement type is accepted as a cuddled
// neighbor.
func (w *WSL) countValidCuddledStatements(
	targetIdents []*ast.Ident,
	cursor *Cursor,
	allowAnyStmtType bool,
	limit int,
) (int, bool) {
	defer cursor.Save()()

	currentStmtStartLine := w.lineFor(cursor.Stmt().Pos())
	count := 0

	for cursor.Previous() {
		prevEndLine := w.lineFor(cursor.Stmt().End())
		if prevEndLine != currentStmtStartLine-1 {
			break
		}

		if count >= limit {
			break
		}

		prevNode := cursor.Stmt()
		if !isAssignDeclOrIncDec(prevNode) && !allowAnyStmtType {
			break
		}

		prevIdents := w.identsFromNode(prevNode, true)
		if !identsIntersect(prevIdents, targetIdents) {
			return count, true
		}

		count++
		currentStmtStartLine = w.lineFor(cursor.Stmt().Pos())
	}

	return count, false
}

func (w *WSL) checkCuddlingWithoutIntersection(stmt ast.Node, cursor *Cursor) {
	if w.numberOfStatementsAbove(cursor) == 0 {
		return
	}

	if _, ok := cursor.Stmt().(*ast.LabeledStmt); ok {
		return
	}

	previousNode := cursor.PreviousNode()

	currAssign, currIsAssign := stmt.(*ast.AssignStmt)
	previousAssign, prevIsAssign := previousNode.(*ast.AssignStmt)
	_, prevIsDecl := previousNode.(*ast.DeclStmt)
	_, prevIsIncDec := previousNode.(*ast.IncDecStmt)

	// Cuddling without intersection is allowed for assignments and inc/dec
	// statements. If however the check for declarations is disabled, we also
	// allow cuddling with them as well.
	//
	// var x string
	// x := ""
	// y++
	if _, ok := w.config.Checks[CheckDecl]; ok {
		prevIsDecl = false
	}

	// If we enable exclusive assign checks we only allow new declarations or
	// new assignments together but not mix and match.
	//
	// When this is enabled we also implicitly disable support to cuddle with
	// anything else.
	if _, ok := w.config.Checks[CheckAssignExclusive]; ok {
		prevIsDecl = false
		prevIsIncDec = false

		if prevIsAssign && currIsAssign {
			prevIsAssign = previousAssign.Tok == currAssign.Tok
		}
	}

	prevIsValidType := previousNode == nil || prevIsAssign || prevIsDecl || prevIsIncDec

	if _, ok := w.config.Checks[CheckAssignExpr]; !ok {
		if _, ok := previousNode.(*ast.ExprStmt); ok && w.hasIntersection(stmt, previousNode) {
			prevIsValidType = prevIsValidType || ok
		}
	}

	if prevIsValidType {
		return
	}

	if w.isLockOrUnlock(stmt, previousNode) {
		return
	}

	w.addErrorInvalidTypeCuddle(stmt.Pos(), cursor.checkType)
}

func (w *WSL) checkBlock(block *ast.BlockStmt, cursor *Cursor) {
	// Block can be nil for function declarations without a body.
	if block == nil {
		return
	}

	w.checkBlockLeadingNewline(block)
	w.checkTrailingNewline(block)
	w.checkNewlineAfterBlock(block, cursor)

	w.checkBodyBlock(block)
}

func (w *WSL) checkNewlineAfterBlock(block *ast.BlockStmt, cursor *Cursor) {
	if _, ok := w.config.Checks[CheckAfterBlock]; !ok {
		return
	}

	// For function blocks we don't have any statements in our cursor.
	if cursor.Len() == 0 {
		return
	}

	currentStmt := cursor.Stmt()

	w.checkNewlineAfter(
		block.Rbrace,
		block,
		currentStmt,
		cursor,
		CheckAfterBlock,
		func(nextStmt ast.Stmt, previousNode ast.Node) bool {
			// Exception: if err != nil { } followed by defer that references
			// a variable assigned above the if block.
			if w.isErrNotNilCheck(currentStmt) != nil {
				if deferStmt, ok := nextStmt.(*ast.DeferStmt); ok && previousNode != nil {
					if w.hasIntersection(previousNode, deferStmt) {
						return true
					}
				}
			}

			return false
		},
	)
}

// checkNewlineAfter verifies that a blank line separates the current statement
// (bounded by boundary) from whatever content comes next in its enclosing
// block. `reportPos` is where the diagnostic is attached, `boundary` is the
// node whose End() defines the line after which a newline is required, and
// `currentStmt` is the statement owning the check (used to discriminate
// comments that still belong to it, e.g. comments inside an else block when
// checking the if-body). When `checkTrailingComment` is true and currentStmt
// is the last statement in its block, a trailing comment on the following
// line (inside the enclosing block) also triggers a diagnostic. If
// `isException` returns true for the next statement and the previous node, no
// diagnostic is reported.
func (w *WSL) checkNewlineAfter(
	reportPos token.Pos,
	boundary ast.Node,
	currentStmt ast.Node,
	cursor *Cursor,
	check CheckType,
	isException func(nextStmt ast.Stmt, previousNode ast.Node) bool,
) {
	if _, ok := w.config.Checks[check]; !ok {
		return
	}

	if cursor.Len() == 0 {
		return
	}

	defer cursor.Save()()

	previousNode := cursor.PreviousNode()

	if !cursor.Next() {
		// No more statements after this one so check for comments after.
		// Skip comments that are inside the current statement (e.g., inside an else block).
		// Also skip comments that appear on the same line as the enclosing block's closing
		// brace — those are inline comments on the brace itself, not trailing comments
		// inside the block.
		if commentPos := w.commentOnLineAfterNodePos(boundary); commentPos != token.NoPos {
			isAfterStmt := commentPos >= currentStmt.End()
			isSameLine := w.lineFor(commentPos) == cursor.rbraceLine
			isUnknownOrNotSameLine := cursor.rbraceLine == 0 || !isSameLine

			if isAfterStmt && isUnknownOrNotSameLine {
				insertPos := w.lineStartOf(commentPos)
				w.addError(
					reportPos,
					insertPos,
					insertPos,
					messageMissingWhitespaceBelow,
					check,
				)
			}
		}

		return
	}

	nextStmt := cursor.Stmt()
	if isException != nil && isException(nextStmt, previousNode) {
		return
	}

	boundaryLine := w.lineFor(boundary.End())
	nextContentPos := nextStmt.Pos()
	nextContentLine := w.lineFor(nextContentPos)

	// Find the first comment between the boundary and the next statement.
	for _, cg := range w.file.Comments {
		if cg.End() <= boundary.End() {
			continue
		}

		// Skip comments that are inside the current statement but after the
		// boundary. This handles cases like comments inside an else block when
		// checking the if-body.
		if cg.Pos() < currentStmt.End() {
			continue
		}

		if w.lineFor(cg.End()) == boundaryLine {
			continue
		}

		commentLine := w.lineFor(cg.Pos())
		if commentLine > boundaryLine && commentLine < nextContentLine {
			nextContentPos = cg.Pos()
			nextContentLine = commentLine
		}

		break
	}

	if nextContentLine <= boundaryLine+1 {
		insertPos := w.lineStartOf(nextContentPos)
		w.addError(
			reportPos,
			insertPos,
			insertPos,
			messageMissingWhitespaceBelow,
			check,
		)
	}
}

func (w *WSL) checkCaseClause(stmt *ast.CaseClause, cursor *Cursor) {
	w.checkCaseLeadingNewline(stmt)

	if w.config.CaseMaxLines != 0 {
		w.checkCaseTrailingNewline(stmt.Body, cursor)
	}

	w.checkBodyStmts(stmt.Body)
}

func (w *WSL) checkCommClause(stmt *ast.CommClause, cursor *Cursor) {
	w.checkCommLeadingNewline(stmt)

	if w.config.CaseMaxLines != 0 {
		w.checkCaseTrailingNewline(stmt.Body, cursor)
	}

	w.checkBodyStmts(stmt.Body)
}

func (w *WSL) checkAssign(stmt *ast.AssignStmt, cursor *Cursor) {
	defer w.checkAppend(stmt, cursor)

	if _, ok := w.config.Checks[CheckAssign]; !ok {
		return
	}

	cursor.SetChecker(CheckAssign)

	w.checkCuddlingWithoutIntersection(stmt, cursor)
}

func (w *WSL) checkAppend(stmt *ast.AssignStmt, cursor *Cursor) {
	if _, ok := w.config.Checks[CheckAppend]; !ok {
		return
	}

	if w.numberOfStatementsAbove(cursor) == 0 {
		return
	}

	previousNode := cursor.PreviousNode()

	var appendNode *ast.CallExpr

	for _, expr := range stmt.Rhs {
		e, ok := expr.(*ast.CallExpr)
		if !ok {
			continue
		}

		if f, ok := e.Fun.(*ast.Ident); ok && f.Name == "append" {
			appendNode = e
			break
		}
	}

	if appendNode == nil {
		return
	}

	if !w.hasIntersection(appendNode, previousNode) {
		w.addErrorNoIntersection(stmt.Pos(), CheckAppend)
	}
}

func (w *WSL) checkBranch(stmt *ast.BranchStmt, cursor *Cursor) {
	if _, ok := w.config.Checks[CheckBranch]; !ok {
		return
	}

	cursor.SetChecker(CheckBranch)

	if w.numberOfStatementsAbove(cursor) == 0 {
		return
	}

	lastStmtInBlock := cursor.statements[len(cursor.statements)-1]
	firstStmts := cursor.Nth(0)

	if w.lineFor(lastStmtInBlock.End())-w.lineFor(firstStmts.Pos()) < w.config.BranchMaxLines {
		return
	}

	w.addErrorTooManyLines(stmt.Pos(), cursor.checkType)
}

func (w *WSL) checkDeclStmt(stmt *ast.DeclStmt, cursor *Cursor) {
	defer func() {
		// maybeGroupDecl can advance the cursor past consecutive decls, so
		// use the cursor's current position at defer-time to get the last
		// decl in the group.
		lastDecl, ok := cursor.Stmt().(*ast.DeclStmt)
		if !ok {
			lastDecl = stmt
		}

		w.checkAfterDecl(lastDecl, cursor)
	}()

	if _, ok := w.config.Checks[CheckDecl]; !ok {
		return
	}

	cursor.SetChecker(CheckDecl)

	if w.numberOfStatementsAbove(cursor) == 0 {
		return
	}

	// Try to do smart grouping and if we succeed return, otherwise do
	// line-by-line fixing.
	if w.maybeGroupDecl(stmt, cursor) {
		return
	}

	w.addErrorNeverAllow(stmt.Pos(), cursor.checkType)
}

func (w *WSL) checkAfterDecl(stmt *ast.DeclStmt, cursor *Cursor) {
	w.checkNewlineAfter(
		stmt.End(),
		stmt,
		stmt,
		cursor,
		CheckAfterDecl,
		func(nextStmt ast.Stmt, _ ast.Node) bool {
			nextDecl, ok := nextStmt.(*ast.DeclStmt)
			if !ok {
				return false
			}

			currGen, currOK := stmt.Decl.(*ast.GenDecl)
			nextGen, nextOK := nextDecl.Decl.(*ast.GenDecl)

			return currOK && nextOK && currGen.Tok == nextGen.Tok
		},
	)
}

func (w *WSL) checkDefer(stmt *ast.DeferStmt, cursor *Cursor) {
	defer w.checkAfterDefer(stmt, cursor)

	if _, ok := w.config.Checks[CheckDefer]; !ok {
		return
	}

	cursor.SetChecker(CheckDefer)

	previousNode := cursor.PreviousNode()
	_, previousIsDefer := previousNode.(*ast.DeferStmt)
	_, previousIsIf := previousNode.(*ast.IfStmt)

	// We allow defer as a third node only if we have an if statement
	// between, e.g.
	//
	// 	f, err := os.Open(file)
	// 	if err != nil {
	// 	    return err
	// 	}
	// defer f.Close()
	if previousIsIf && w.numberOfStatementsAbove(cursor) >= 2 {
		defer cursor.Save()()

		cursor.Previous()
		cursor.Previous()

		if w.hasIntersection(cursor.Stmt(), stmt) {
			return
		}
	}

	// Only check cuddling if previous statement isn't also a defer.
	if previousIsDefer {
		return
	}

	// If calling a function literal, inspect the function body for shared
	// variables, similar to how block statements inspect their body via
	// checkCuddlingBlock.
	if funcLit, ok := stmt.Call.Fun.(*ast.FuncLit); ok {
		w.checkCuddlingBlock(stmt, funcLit.Body.List, []*ast.Ident{}, cursor)
		return
	}

	w.checkCuddling(stmt, cursor, true)
}

func (w *WSL) checkAfterDefer(stmt *ast.DeferStmt, cursor *Cursor) {
	w.checkNewlineAfter(
		stmt.End(),
		stmt,
		stmt,
		cursor,
		CheckAfterDefer,
		func(nextStmt ast.Stmt, _ ast.Node) bool {
			_, ok := nextStmt.(*ast.DeferStmt)
			return ok
		},
	)
}

func (w *WSL) checkError(
	stmtsAbove int,
	ifStmt ast.Node,
	previousNode ast.Node,
	cursor *Cursor,
) {
	if _, ok := w.config.Checks[CheckErr]; !ok {
		return
	}

	if stmtsAbove > 0 {
		return
	}

	if _, ok := cursor.Stmt().(*ast.LabeledStmt); ok {
		return
	}

	defer cursor.Save()()

	errIdent := w.isErrNotNilCheck(ifStmt)
	if errIdent == nil {
		return
	}

	previousIdents := []*ast.Ident{}

	if assign, ok := previousNode.(*ast.AssignStmt); ok {
		for _, lhs := range assign.Lhs {
			previousIdents = append(previousIdents, w.identsFromNode(lhs, true)...)
		}
	}

	if decl, ok := previousNode.(*ast.DeclStmt); ok {
		if genDecl, ok := decl.Decl.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					previousIdents = append(previousIdents, vs.Names...)
				}
			}
		}
	}

	// Ensure that the error checked on this line was assigned or declared in
	// the previous statement.
	if !identsIntersect([]*ast.Ident{errIdent}, previousIdents) {
		return
	}

	cursor.SetChecker(CheckErr)

	previousEndLine := w.lineFor(previousNode.End())

	// Check for comments on the same line as the previous node (extends effective end line).
	for _, cg := range w.file.Comments {
		if cg.Pos() >= ifStmt.Pos() {
			break
		}

		if cg.Pos() < previousNode.End() || cg.End() > ifStmt.Pos() {
			continue
		}

		// There's a comment between the error variable and the if-statement.
		// If it's on a different line, we can't do much about this.
		if w.lineFor(cg.End()) != previousEndLine {
			return
		}

		// Comment is on the same line - no need to update since line stays the same.
	}

	ifStmtLine := w.lineFor(ifStmt.Pos())
	file := w.fset.File(ifStmt.Pos())

	// Remove blank lines between previous node and if statement.
	removeStart := file.LineStart(previousEndLine + 1)
	removeEnd := file.LineStart(ifStmtLine)
	w.addErrorRemoveNewline(removeStart, removeEnd, cursor.checkType)

	// If we add the error at the same position but with a different fix
	// range, only the fix range will be updated.
	//
	//   a := 1
	//   err := fn()
	//
	//   if err != nil {}
	//
	// Should become
	//
	//   a := 1
	//
	//   err := fn()
	//   if err != nil {}
	cursor.Previous()

	// Add whitespace above the error assignment if there's a statement above.
	if w.numberOfStatementsAbove(cursor) > 0 {
		insertPos := w.lineStartOf(previousNode.Pos())
		w.addError(removeStart, insertPos, insertPos, messageMissingWhitespaceAbove, cursor.checkType)
	}
}

func (w *WSL) checkExprStmt(stmt *ast.ExprStmt, cursor *Cursor) {
	defer w.checkAfterExpr(stmt, cursor)

	if _, ok := w.config.Checks[CheckExpr]; !ok {
		return
	}

	cursor.SetChecker(CheckExpr)

	// Consecutive expression statements don't need to be separated, so only
	// check cuddling if the previous statement isn't also an expression.
	if _, ok := cursor.PreviousNode().(*ast.ExprStmt); !ok {
		w.checkCuddling(stmt, cursor, false)
	}
}

func (w *WSL) checkAfterExpr(stmt *ast.ExprStmt, cursor *Cursor) {
	w.checkNewlineAfter(
		stmt.End(),
		stmt,
		stmt,
		cursor,
		CheckAfterExpr,
		func(nextStmt ast.Stmt, _ ast.Node) bool {
			// Consecutive expressions don't need a blank line between them.
			if _, ok := nextStmt.(*ast.ExprStmt); ok {
				return true
			}

			// Exception: expr followed by a defer that references the same
			// variable (e.g. mu.Lock() / defer mu.Unlock()).
			if deferStmt, ok := nextStmt.(*ast.DeferStmt); ok {
				return w.hasIntersection(stmt, deferStmt)
			}

			return false
		},
	)
}

func (w *WSL) checkFor(stmt *ast.ForStmt, cursor *Cursor) {
	w.maybeCheckBlock(stmt, stmt.Body, cursor, CheckFor)
}

func (w *WSL) checkGo(stmt *ast.GoStmt, cursor *Cursor) {
	defer w.checkAfterGo(stmt, cursor)

	if _, ok := w.config.Checks[CheckGo]; !ok {
		return
	}

	cursor.SetChecker(CheckGo)

	// We can cuddle any amount `go` statements so only check cuddling if
	// the previous one isn't a `go` call.
	if _, ok := cursor.PreviousNode().(*ast.GoStmt); ok {
		return
	}

	// If calling a function literal, inspect the function body for shared
	// variables, similar to how block statements inspect their body via
	// checkCuddlingBlock.
	if funcLit, ok := stmt.Call.Fun.(*ast.FuncLit); ok {
		w.checkCuddlingBlock(stmt, funcLit.Body.List, []*ast.Ident{}, cursor)
		return
	}

	w.checkCuddling(stmt, cursor, true)
}

func (w *WSL) checkAfterGo(stmt *ast.GoStmt, cursor *Cursor) {
	w.checkNewlineAfter(
		stmt.End(),
		stmt,
		stmt,
		cursor,
		CheckAfterGo,
		func(nextStmt ast.Stmt, _ ast.Node) bool {
			_, ok := nextStmt.(*ast.GoStmt)
			return ok
		},
	)
}

func (w *WSL) checkIf(stmt *ast.IfStmt, cursor *Cursor, isElse bool) {
	// if
	w.checkBlock(stmt.Body, cursor)

	switch v := stmt.Else.(type) {
	// else-if
	case *ast.IfStmt:
		w.checkIf(v, cursor, true)

	// else
	case *ast.BlockStmt:
		w.checkBlock(v, cursor)
	}

	if _, ok := w.config.Checks[CheckIf]; !isElse && ok {
		cursor.SetChecker(CheckIf)
		w.checkCuddlingBlock(stmt, stmt.Body.List, []*ast.Ident{}, cursor)
	} else if _, ok := w.config.Checks[CheckErr]; !isElse && ok {
		previousNode := cursor.PreviousNode()

		w.checkError(
			w.numberOfStatementsAbove(cursor),
			stmt,
			previousNode,
			cursor,
		)
	}
}

func (w *WSL) checkIncDec(stmt *ast.IncDecStmt, cursor *Cursor) {
	if _, ok := w.config.Checks[CheckIncDec]; !ok {
		return
	}

	cursor.SetChecker(CheckIncDec)

	w.checkCuddlingWithoutIntersection(stmt, cursor)
}

func (w *WSL) checkLabel(stmt *ast.LabeledStmt, cursor *Cursor) {
	// We check the statement last because the statement is the same node as the
	// label (it's a labeled statement). This means that we _first_ want to
	// check any violations of cuddling the label (never cuddle label) before we
	// actually check the inner statement.
	//
	// It's a subtle difference, but it makes the diagnostic make more sense.
	// We do this by deferring the statmenet check so it happens last no matter
	// if we have label checking enabled or not.
	defer w.checkStmt(stmt.Stmt, cursor)

	if _, ok := w.config.Checks[CheckLabel]; !ok {
		return
	}

	cursor.SetChecker(CheckLabel)

	if w.numberOfStatementsAbove(cursor) == 0 {
		return
	}

	w.addErrorNeverAllow(stmt.Pos(), cursor.checkType)
}

func (w *WSL) checkRange(stmt *ast.RangeStmt, cursor *Cursor) {
	w.maybeCheckBlock(stmt, stmt.Body, cursor, CheckRange)
}

func (w *WSL) checkReturn(stmt *ast.ReturnStmt, cursor *Cursor) {
	if _, ok := w.config.Checks[CheckReturn]; !ok {
		return
	}

	cursor.SetChecker(CheckReturn)

	// There's only a return statement.
	if cursor.Len() <= 1 {
		return
	}

	if w.numberOfStatementsAbove(cursor) == 0 {
		return
	}

	// If the distance between the first statement and the return statement is
	// less than `n` LOC we're allowed to cuddle.
	firstStmts := cursor.Nth(0)
	if w.lineFor(stmt.End())-w.lineFor(firstStmts.Pos()) < w.config.BranchMaxLines {
		return
	}

	w.addErrorTooManyLines(stmt.Pos(), cursor.checkType)
}

func (w *WSL) checkSelect(stmt *ast.SelectStmt, cursor *Cursor) {
	w.maybeCheckBlock(stmt, stmt.Body, cursor, CheckSelect)
}

func (w *WSL) checkSend(stmt *ast.SendStmt, cursor *Cursor) {
	if _, ok := w.config.Checks[CheckSend]; !ok {
		return
	}

	cursor.SetChecker(CheckSend)

	var stmts []ast.Stmt

	ast.Inspect(stmt.Value, func(n ast.Node) bool {
		if b, ok := n.(*ast.BlockStmt); ok {
			stmts = b.List
			return false
		}

		return true
	})

	w.checkCuddlingBlock(stmt, stmts, []*ast.Ident{}, cursor)
}

func (w *WSL) checkSwitch(stmt *ast.SwitchStmt, cursor *Cursor) {
	w.maybeCheckBlock(stmt, stmt.Body, cursor, CheckSwitch)
}

func (w *WSL) checkTypeSwitch(stmt *ast.TypeSwitchStmt, cursor *Cursor) {
	w.maybeCheckBlock(stmt, stmt.Body, cursor, CheckTypeSwitch)
}

func (w *WSL) checkCaseTrailingNewline(body []ast.Stmt, cursor *Cursor) {
	if len(body) == 0 {
		return
	}

	defer cursor.Save()()

	if !cursor.Next() {
		return
	}

	var nextCase ast.Node

	switch n := cursor.Stmt().(type) {
	case *ast.CaseClause:
		nextCase = n
	case *ast.CommClause:
		nextCase = n
	default:
		return
	}

	var (
		firstStmt  = body[0]
		lastStmt   = body[len(body)-1]
		totalLines = w.lineFor(nextCase.Pos()) - w.lineFor(firstStmt.Pos())
	)

	if totalLines < w.config.CaseMaxLines {
		return
	}

	var (
		lastStmtEndLine = w.lineFor(lastStmt.End())
		nextCaseLine    = w.lineFor(nextCase.Pos())
		nextCaseCol     = w.fset.PositionFor(nextCase.Pos(), false).Column
	)

	// Find transition point between trailing content (indented) and leading
	// content (left-aligned). Trailing comments belong to current case, leading
	// comments belong to next case. The blank line goes at the transition.
	var (
		lastStmtOrCommentEnd         = lastStmt.End()
		nextCaseOrLeftAlignedComment = nextCase.Pos()
		lastLeftAlignedCommentEnd    = token.NoPos
	)

	for _, commentGroup := range w.file.Comments {
		if commentGroup.Pos() >= nextCase.Pos() {
			break
		}

		if commentGroup.End() <= lastStmt.End() {
			continue
		}

		for _, comment := range commentGroup.List {
			commentLine := w.lineFor(comment.Pos())
			if commentLine <= lastStmtEndLine || commentLine >= nextCaseLine {
				continue
			}

			col := w.fset.PositionFor(comment.Pos(), false).Column
			if col <= nextCaseCol {
				// Left-aligned: first one marks transition point
				if lastLeftAlignedCommentEnd == token.NoPos {
					nextCaseOrLeftAlignedComment = comment.Pos()
				}

				lastLeftAlignedCommentEnd = comment.End()
			} else {
				// Indented: extend trailing content
				lastStmtOrCommentEnd = comment.End()
			}
		}
	}

	lastStmtOrCommentLine := w.lineFor(lastStmtOrCommentEnd)
	nextCaseOrLeadingCommentLine := w.lineFor(nextCaseOrLeftAlignedComment)

	// Check for unnecessary blank line before case (leading comments should be flush).
	if lastLeftAlignedCommentEnd != token.NoPos {
		lastLeadingEndLine := w.lineFor(lastLeftAlignedCommentEnd)

		if lastLeadingEndLine < nextCaseLine-1 {
			file := w.fset.File(nextCase.Pos())
			w.addErrorRemoveNewline(file.LineStart(lastLeadingEndLine+1), file.LineStart(nextCaseLine), CheckCaseTrailingNewline)
		}
	}

	// Already has a blank line at the boundary.
	if nextCaseOrLeadingCommentLine > lastStmtOrCommentLine+1 {
		return
	}

	insertPos := w.lineStartOf(nextCaseOrLeftAlignedComment)
	w.addError(lastStmtOrCommentEnd, insertPos, insertPos, messageMissingWhitespaceBelow, CheckCaseTrailingNewline)
}

func (w *WSL) checkBlockLeadingNewline(body *ast.BlockStmt) {
	w.checkLeadingNewline(body.Lbrace, body.List)
}

func (w *WSL) checkCaseLeadingNewline(caseClause *ast.CaseClause) {
	w.checkLeadingNewline(caseClause.Colon, caseClause.Body)
}

func (w *WSL) checkCommLeadingNewline(commClause *ast.CommClause) {
	w.checkLeadingNewline(commClause.Colon, commClause.Body)
}

func (w *WSL) checkLeadingNewline(startPos token.Pos, body []ast.Stmt) {
	if _, ok := w.config.Checks[CheckLeadingWhitespace]; !ok {
		return
	}

	if len(body) == 0 {
		return
	}

	var (
		openLine        = w.lineFor(startPos)
		firstStmtPos    = body[0].Pos()
		firstStmtLine   = w.lineFor(firstStmtPos)
		leadingComments []*ast.CommentGroup
	)

	for _, cg := range w.file.Comments {
		if cg.Pos() >= firstStmtPos {
			break
		}

		if cg.Pos() > startPos {
			leadingComments = append(leadingComments, cg)
		}
	}

	if len(leadingComments) == 0 {
		if firstStmtLine := w.lineFor(firstStmtPos); firstStmtLine > openLine+1 {
			file := w.fset.File(startPos)
			w.addErrorRemoveNewline(
				file.LineStart(openLine+1),
				file.LineStart(firstStmtLine),
				CheckLeadingWhitespace,
			)
		}

		return
	}

	var (
		firstContentLine   = firstStmtLine
		lastCommentEndLine = openLine
	)

	for _, comment := range leadingComments {
		startLine := w.lineFor(comment.Pos())
		endLine := w.lineFor(comment.End())

		if startLine > openLine && startLine < firstContentLine {
			firstContentLine = startLine
		}

		if endLine > lastCommentEndLine {
			lastCommentEndLine = endLine
		}
	}

	file := w.fset.File(startPos)

	// Empty line after opening brace.
	if firstContentLine > openLine+1 {
		w.addErrorRemoveNewline(
			file.LineStart(openLine+1),
			file.LineStart(firstContentLine),
			CheckLeadingWhitespace,
		)
	}

	// Empty line between comments and first statement.
	if lastCommentEndLine > openLine && firstStmtLine > lastCommentEndLine+1 {
		w.addErrorRemoveNewline(
			file.LineStart(lastCommentEndLine+1),
			file.LineStart(firstStmtLine),
			CheckLeadingWhitespace,
		)
	}
}

func (w *WSL) checkTrailingNewline(body *ast.BlockStmt) {
	if _, ok := w.config.Checks[CheckTrailingWhitespace]; !ok {
		return
	}

	if len(body.List) == 0 {
		return
	}

	lastStmt := body.List[len(body.List)-1]

	// We don't want to force removal of the empty line for the last case since
	// it can be used for consistency and readability.
	if _, ok := lastStmt.(*ast.CaseClause); ok {
		return
	}

	lastContentPos := lastStmt.End()

	// Empty label statements need positional adjustment. #92
	if l, ok := lastStmt.(*ast.LabeledStmt); ok {
		if _, ok := l.Stmt.(*ast.EmptyStmt); ok {
			lastContentPos = lastStmt.Pos()
		}
	}

	// Find the last comment after last statement using position comparison.
	for _, cg := range w.file.Comments {
		if cg.End() <= lastContentPos {
			continue
		}

		if cg.Pos() >= body.Rbrace {
			break
		}

		if cg.End() < body.Rbrace {
			lastContentPos = cg.End()
		}
	}

	closingLine := w.lineFor(body.Rbrace)
	lastContentLine := w.lineFor(lastContentPos)

	if closingLine > lastContentLine+1 {
		file := w.fset.File(body.Rbrace)
		removeStart := file.LineStart(lastContentLine + 1)
		removeEnd := file.LineStart(closingLine)
		w.addErrorRemoveNewline(removeStart, removeEnd, CheckTrailingWhitespace)
	}
}

func (w *WSL) maybeGroupDecl(stmt *ast.DeclStmt, cursor *Cursor) bool {
	firstNode := asGenDeclWithValueSpecs(cursor.PreviousNode())
	if firstNode == nil {
		return false
	}

	currentNode := asGenDeclWithValueSpecs(stmt)
	if currentNode == nil {
		return false
	}

	// Both are not same type, e.g. `const` or `var`
	if firstNode.Tok != currentNode.Tok {
		return false
	}

	specs := slices.Concat(firstNode.Specs, currentNode.Specs)

	reportNodes := []ast.Node{currentNode}
	lastNode := currentNode

	for {
		nextPeeked := cursor.NextNode()
		if nextPeeked == nil {
			break
		}

		if w.lineFor(lastNode.End()) < w.lineFor(nextPeeked.Pos())-1 {
			break
		}

		nextNode := asGenDeclWithValueSpecs(nextPeeked)
		if nextNode == nil {
			break
		}

		if nextNode.Tok != firstNode.Tok {
			break
		}

		cursor.Next()

		specs = append(specs, nextNode.Specs...)
		reportNodes = append(reportNodes, nextNode)
		lastNode = nextNode
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s (\n", firstNode.Tok)

	for _, spec := range specs {
		var specBuf bytes.Buffer
		if err := format.Node(&specBuf, w.fset, spec); err != nil {
			return false
		}

		buf.Write(specBuf.Bytes())
		buf.WriteByte('\n')
	}

	buf.WriteByte(')')

	// We add a diagnostic to every subsequent statement to properly represent
	// the violations. Duplicate fixes for the same range is fine.
	for _, n := range reportNodes {
		w.addErrorWithMessageAndFix(
			n.Pos(),
			firstNode.Pos(),
			lastNode.End(),
			fmt.Sprintf("%s (never cuddle %s)", messageMissingWhitespaceAbove, CheckDecl),
			buf.Bytes(),
		)
	}

	return true
}

func (w *WSL) maybeCheckBlock(
	node ast.Node,
	blockStmt *ast.BlockStmt,
	cursor *Cursor,
	check CheckType,
) {
	w.checkBlock(blockStmt, cursor)

	if _, ok := w.config.Checks[check]; ok {
		cursor.SetChecker(check)

		var (
			blockList     []ast.Stmt
			allowedIdents []*ast.Ident
		)

		if check != CheckSwitch && check != CheckTypeSwitch && check != CheckSelect {
			blockList = blockStmt.List
		} else {
			allowedIdents = w.identsFromCaseArms(node)
		}

		w.checkCuddlingBlock(node, blockList, allowedIdents, cursor)
	}
}

// numberOfStatementsAbove will find out how many lines above the cursor's
// current statement there is without any newlines between.
func (w *WSL) numberOfStatementsAbove(cursor *Cursor) int {
	defer cursor.Save()()

	statementsWithoutNewlines := 0
	currentStmtStartLine := w.lineFor(cursor.Stmt().Pos())

	for cursor.Previous() {
		previousStmtEndLine := w.lineFor(cursor.Stmt().End())
		if previousStmtEndLine != currentStmtStartLine-1 {
			break
		}

		currentStmtStartLine = w.lineFor(cursor.Stmt().Pos())
		statementsWithoutNewlines++
	}

	return statementsWithoutNewlines
}

func (w *WSL) lineFor(pos token.Pos) int {
	return w.fset.PositionFor(pos, false).Line
}

func (w *WSL) lineStartOf(pos token.Pos) token.Pos {
	return w.fset.File(pos).LineStart(w.lineFor(pos))
}

func (w *WSL) implementsErr(node *ast.Ident) bool {
	typeInfo := w.typeInfo.TypeOf(node)
	if typeInfo == nil {
		return false
	}

	errorType, ok := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	if !ok {
		return false
	}

	return types.Implements(typeInfo, errorType)
}

func (w *WSL) commentOnLineAfterNodePos(node ast.Node) token.Pos {
	nodeEndLine := w.lineFor(node.End())

	for _, cg := range w.file.Comments {
		if cg.End() <= node.End() {
			continue
		}

		commentLine := w.lineFor(cg.Pos())
		if commentLine == nodeEndLine {
			continue
		}

		if commentLine == nodeEndLine+1 {
			return cg.Pos()
		}

		break
	}

	return token.NoPos
}

func (w *WSL) addErrorInvalidTypeCuddle(pos token.Pos, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (invalid statement above %s)", messageMissingWhitespaceAbove, ct)
	insertPos := w.lineStartOf(pos)
	w.addErrorWithMessage(pos, insertPos, insertPos, reportMessage)
}

func (w *WSL) addErrorTooManyStatements(pos token.Pos, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (too many statements above %s)", messageMissingWhitespaceAbove, ct)
	insertPos := w.lineStartOf(pos)
	w.addErrorWithMessage(pos, insertPos, insertPos, reportMessage)
}

func (w *WSL) addErrorNoIntersection(pos token.Pos, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (no shared variables above %s)", messageMissingWhitespaceAbove, ct)
	insertPos := w.lineStartOf(pos)
	w.addErrorWithMessage(pos, insertPos, insertPos, reportMessage)
}

func (w *WSL) addErrorVariableNotShared(pos token.Pos, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (variable not shared with %s)", messageMissingWhitespaceAbove, ct)
	insertPos := w.lineStartOf(pos)
	w.addErrorWithMessage(pos, insertPos, insertPos, reportMessage)
}

func (w *WSL) addErrorTooManyLines(pos token.Pos, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (too many lines above %s)", messageMissingWhitespaceAbove, ct)
	insertPos := w.lineStartOf(pos)
	w.addErrorWithMessage(pos, insertPos, insertPos, reportMessage)
}

func (w *WSL) addErrorNeverAllow(pos token.Pos, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (never cuddle %s)", messageMissingWhitespaceAbove, ct)
	insertPos := w.lineStartOf(pos)
	w.addErrorWithMessage(pos, insertPos, insertPos, reportMessage)
}

func (w *WSL) addError(report, start, end token.Pos, message string, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (%s)", message, ct)
	w.addErrorWithMessage(report, start, end, reportMessage)
}

func (w *WSL) addErrorRemoveNewline(start, end token.Pos, ct CheckType) {
	reportMessage := fmt.Sprintf("%s (%s)", messageRemoveWhitespace, ct)
	w.addErrorWithMessageAndFix(start, start, end, reportMessage, []byte{})
}

func (w *WSL) addErrorWithMessage(report, start, end token.Pos, message string) {
	w.addErrorWithMessageAndFix(report, start, end, message, []byte("\n"))
}

func (w *WSL) addErrorWithMessageAndFix(report, start, end token.Pos, message string, fix []byte) {
	iss, ok := w.issues[report]
	if !ok {
		iss = issue{
			message:   message,
			fixRanges: []fixRange{},
		}
	}

	// Don't add a fix range that overlaps with an existing one — that would
	// produce conflicting TextEdits. The existing fix already covers this range,
	// so the diagnostic is surfaced via the first reporter.
	for _, existing := range iss.fixRanges {
		if start < existing.fixRangeEnd && end > existing.fixRangeStart {
			return
		}
	}

	iss.fixRanges = append(iss.fixRanges, fixRange{
		fixRangeStart: start,
		fixRangeEnd:   end,
		fix:           fix,
	})

	w.issues[report] = iss
}

func isAssignDeclOrIncDec(n ast.Node) bool {
	_, a := n.(*ast.AssignStmt)
	_, d := n.(*ast.DeclStmt)
	_, i := n.(*ast.IncDecStmt)

	return a || d || i
}

func asGenDeclWithValueSpecs(n ast.Node) *ast.GenDecl {
	decl, ok := n.(*ast.DeclStmt)
	if !ok {
		return nil
	}

	genDecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok {
		return nil
	}

	for _, spec := range genDecl.Specs {
		// We only care about value specs and not type specs or import
		// specs. We will never see any import specs but type specs we just
		// separate with an empty line as usual.
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			return nil
		}

		// It's very hard to get comments right in the ast and with the current
		// way the ast package works we simply don't support grouping at all if
		// there are any comments related to the node.
		if valueSpec.Doc != nil || valueSpec.Comment != nil {
			return nil
		}
	}

	return genDecl
}

func (w *WSL) hasIntersection(a, b ast.Node) bool {
	aI := w.identsFromNode(a, true)
	bI := w.identsFromNode(b, true)

	return identsIntersect(aI, bI)
}

func identsIntersect(a, b []*ast.Ident) bool {
	for _, as := range a {
		for _, bs := range b {
			if as.Name == bs.Name {
				return true
			}
		}
	}

	return false
}

func isTypeOrPredeclConst(obj types.Object) bool {
	switch o := obj.(type) {
	case *types.TypeName:
		// Covers predeclared types ("string", "int", ...) and user types.
		return true
	case *types.Const:
		// true/false/iota are universe consts.
		return o.Parent() == types.Universe
	case *types.Nil:
		return true
	case *types.PkgName:
		// Skip package qualifiers like "fmt" in fmt.Println
		return true
	default:
		return false
	}
}

// identsFromNode returns all *ast.Ident in a node except:
//   - type names (types.TypeName)
//   - builtin constants from the universe (true, false, iota)
//   - nil (*types.Nil)
//   - package names (types.PkgName)
//   - the blank identifier "_"
func (w *WSL) identsFromNode(node ast.Node, skipBlock bool) []*ast.Ident {
	var (
		idents []*ast.Ident
		seen   = map[string]struct{}{}
	)

	if node == nil {
		return idents
	}

	addIdent := func(ident *ast.Ident) {
		if ident == nil {
			return
		}

		name := ident.Name
		if name == "" || name == "_" {
			return
		}

		if _, ok := seen[name]; ok {
			return
		}

		idents = append(idents, ident)
		seen[name] = struct{}{}
	}

	ast.Inspect(node, func(n ast.Node) bool {
		if skipBlock {
			if _, ok := n.(*ast.BlockStmt); ok {
				return false
			}
		}

		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		// Prefer Uses over Defs; fall back to Defs if not a use site.
		var typesObject types.Object
		if obj := w.typeInfo.Uses[ident]; obj != nil {
			typesObject = obj
		} else if obj := w.typeInfo.Defs[ident]; obj != nil {
			typesObject = obj
		}

		// Unresolved (could be a build-tag or syntax artifact). Keep it.
		if typesObject == nil {
			addIdent(ident)
			return true
		}

		if isTypeOrPredeclConst(typesObject) {
			return true
		}

		addIdent(ident)

		return true
	})

	return idents
}

func (w *WSL) identsFromCaseArms(node ast.Node) []*ast.Ident {
	var (
		idents []*ast.Ident
		nodes  []ast.Stmt
		seen   = map[string]struct{}{}

		addUnseen = func(node ast.Node) {
			for _, ident := range w.identsFromNode(node, true) {
				if _, ok := seen[ident.Name]; ok {
					continue
				}

				seen[ident.Name] = struct{}{}
				idents = append(idents, ident)
			}
		}
	)

	switch v := node.(type) {
	case *ast.SwitchStmt:
		nodes = v.Body.List
	case *ast.TypeSwitchStmt:
		nodes = v.Body.List
	case *ast.SelectStmt:
		nodes = v.Body.List
	default:
		return idents
	}

	for _, node := range nodes {
		switch n := node.(type) {
		case *ast.CommClause:
			addUnseen(n.Comm)
		case *ast.CaseClause:
			for _, n := range n.List {
				addUnseen(n)
			}
		default:
			continue
		}
	}

	return idents
}

// hasSelectorCall checks if node contains a selector call with one of the given names.
func hasSelectorCall(node ast.Node, selectorNames []string) bool {
	var found bool

	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false // Already found
		}

		if _, ok := n.(*ast.BlockStmt); ok {
			return false
		}

		if sel, ok := n.(*ast.SelectorExpr); ok {
			found = slices.Contains(selectorNames, sel.Sel.Name)
			return false
		}

		return true
	})

	return found
}

func (w *WSL) isLockOrUnlock(current, previous ast.Node) bool {
	// If we're an ExprStmt (e.g. X()), we check if we're calling `Unlock` or
	// `RWUnlock`. No matter how deep this is or what previous statement was, we
	// allow this.
	//
	// mu.Lock()
	// [ANY BLOCK]
	// mu.Unlock()
	if _, ok := current.(*ast.ExprStmt); ok {
		return hasSelectorCall(current, []string{"Unlock", "RWUnlock"})
	}

	if previous != nil {
		return hasSelectorCall(previous, []string{"Lock", "RWLock", "TryLock"})
	}

	return false
}

// isErrNotNilCheck returns the error identifier if stmt is an `if err != nil`
// or `if err == nil` check without an init statement, nil otherwise.
func (w *WSL) isErrNotNilCheck(stmt ast.Node) *ast.Ident {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok {
		return nil
	}

	// If the error checking has an init condition (e.g. if err := f();) we
	// don't consider it an error check since the error is assigned on this row.
	if ifStmt.Init != nil {
		return nil
	}

	// The condition must be a binary expression (X OP Y)
	binaryExpr, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return nil
	}

	// We must do not equal or equal comparison (!= or ==)
	if binaryExpr.Op != token.NEQ && binaryExpr.Op != token.EQL {
		return nil
	}

	xIdent, ok := binaryExpr.X.(*ast.Ident)
	if !ok {
		return nil
	}

	// X is not an error so it's not error checking
	if !w.implementsErr(xIdent) {
		return nil
	}

	yIdent, ok := binaryExpr.Y.(*ast.Ident)
	if !ok {
		return nil
	}

	// Y is not compared with `nil`
	if yIdent.Name != "nil" {
		return nil
	}

	return xIdent
}
