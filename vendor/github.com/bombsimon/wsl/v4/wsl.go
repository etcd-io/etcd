package wsl

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"sort"
	"strings"
)

// Error reason strings.
const (
	reasonAnonSwitchCuddled              = "anonymous switch statements should never be cuddled"
	reasonAppendCuddledWithoutUse        = "append only allowed to cuddle with appended value"
	reasonAssignsCuddleAssign            = "assignments should only be cuddled with other assignments"
	reasonBlockEndsWithWS                = "block should not end with a whitespace (or comment)"
	reasonBlockStartsWithWS              = "block should not start with a whitespace"
	reasonCaseBlockTooCuddly             = "case block should end with newline at this size"
	reasonDeferCuddledWithOtherVar       = "defer statements should only be cuddled with expressions on same variable"
	reasonExprCuddlingNonAssignedVar     = "only cuddled expressions if assigning variable or using from line above"
	reasonExpressionCuddledWithBlock     = "expressions should not be cuddled with blocks"
	reasonExpressionCuddledWithDeclOrRet = "expressions should not be cuddled with declarations or returns"
	reasonForCuddledAssignWithoutUse     = "for statements should only be cuddled with assignments used in the iteration"
	reasonForWithoutCondition            = "for statement without condition should never be cuddled"
	reasonGoFuncWithoutAssign            = "go statements can only invoke functions assigned on line above"
	reasonMultiLineBranchCuddle          = "branch statements should not be cuddled if block has more than two lines"
	reasonMustCuddleErrCheck             = "if statements that check an error must be cuddled with the statement that assigned the error"
	reasonNeverCuddleDeclare             = "declarations should never be cuddled"
	reasonOnlyCuddle2LineReturn          = "return statements should not be cuddled if block has more than two lines"
	reasonOnlyCuddleIfWithAssign         = "if statements should only be cuddled with assignments"
	reasonOnlyCuddleWithUsedAssign       = "if statements should only be cuddled with assignments used in the if statement itself"
	reasonOnlyOneCuddleBeforeDefer       = "only one cuddle assignment allowed before defer statement"
	reasonOnlyOneCuddleBeforeFor         = "only one cuddle assignment allowed before for statement"
	reasonOnlyOneCuddleBeforeGo          = "only one cuddle assignment allowed before go statement"
	reasonOnlyOneCuddleBeforeIf          = "only one cuddle assignment allowed before if statement"
	reasonOnlyOneCuddleBeforeRange       = "only one cuddle assignment allowed before range statement"
	reasonOnlyOneCuddleBeforeSwitch      = "only one cuddle assignment allowed before switch statement"
	reasonOnlyOneCuddleBeforeTypeSwitch  = "only one cuddle assignment allowed before type switch statement"
	reasonRangeCuddledWithoutUse         = "ranges should only be cuddled with assignments used in the iteration"
	reasonShortDeclNotExclusive          = "short declaration should cuddle only with other short declarations"
	reasonSwitchCuddledWithoutUse        = "switch statements should only be cuddled with variables switched"
	reasonTypeSwitchCuddledWithoutUse    = "type switch statements should only be cuddled with variables switched"
)

// Warning strings.
const (
	warnTypeNotImplement           = "type not implemented"
	warnStmtNotImplemented         = "stmt type not implemented"
	warnBodyStmtTypeNotImplemented = "body statement type not implemented "
	warnWSNodeTypeNotImplemented   = "whitespace node type not implemented "
	warnUnknownLHS                 = "UNKNOWN LHS"
	warnUnknownRHS                 = "UNKNOWN RHS"
)

// Configuration represents configurable settings for the linter.
type Configuration struct {
	// StrictAppend will do strict checking when assigning from append (x =
	// append(x, y)). If this is set to true the append call must append either
	// a variable assigned, called or used on the line above. Example on not
	// allowed when this is true:
	//
	//  x := []string{}
	//  y := "not going in X"
	//  x = append(x, "not y") // This is not allowed with StrictAppend
	//  z := "going in X"
	//
	//  x = append(x, z) // This is allowed with StrictAppend
	//
	//  m := transform(z)
	//  x = append(x, z) // So is this because Z is used above.
	StrictAppend bool

	// AllowAssignAndCallCuddle allows assignments to be cuddled with variables
	// used in calls on line above and calls to be cuddled with assignments of
	// variables used in call on line above.
	// Example supported with this set to true:
	//
	//  x.Call()
	//  x = Assign()
	//  x.AnotherCall()
	//  x = AnotherAssign()
	AllowAssignAndCallCuddle bool

	// AllowAssignAndAnythingCuddle allows assignments to be cuddled with anything.
	// Example supported with this set to true:
	//  if x == 1 {
	//  	x = 0
	//  }
	//  z := x + 2
	// 	fmt.Println("x")
	//  y := "x"
	AllowAssignAndAnythingCuddle bool

	// AllowMultiLineAssignCuddle allows cuddling to assignments even if they
	// span over multiple lines. This defaults to true which allows the
	// following example:
	//
	//  err := function(
	//  	"multiple", "lines",
	//  )
	//  if err != nil {
	//  	// ...
	//  }
	AllowMultiLineAssignCuddle bool

	// If the number of lines in a case block is equal to or lager than this
	// number, the case *must* end white a newline.
	ForceCaseTrailingWhitespaceLimit int

	// AllowTrailingComment will allow blocks to end with comments.
	AllowTrailingComment bool

	// AllowSeparatedLeadingComment will allow multiple comments in the
	// beginning of a block separated with newline. Example:
	//  func () {
	//		// Comment one
	//
	//		// Comment two
	// 		fmt.Println("x")
	//  }
	AllowSeparatedLeadingComment bool

	// AllowCuddleDeclaration will allow multiple var/declaration statements to
	// be cuddled. This defaults to false but setting it to true will enable the
	// following example:
	//  var foo bool
	//  var err error
	AllowCuddleDeclaration bool

	// AllowCuddleWithCalls is a list of call idents that everything can be
	// cuddled with. Defaults to calls looking like locks to support a flow like
	// this:
	//
	//  mu.Lock()
	//  allow := thisAssignment
	AllowCuddleWithCalls []string

	// AllowCuddleWithRHS is a list of right hand side variables that is allowed
	// to be cuddled with anything. Defaults to assignments or calls looking
	// like unlocks to support a flow like this:
	//
	//  allow := thisAssignment()
	//  mu.Unlock()
	AllowCuddleWithRHS []string

	// ForceCuddleErrCheckAndAssign will cause an error when an If statement that
	// checks an error variable doesn't cuddle with the assignment of that variable.
	// This defaults to false but setting it to true will cause the following
	// to generate an error:
	//
	// err := ProduceError()
	//
	// if err != nil {
	//     return err
	// }
	ForceCuddleErrCheckAndAssign bool

	// When ForceCuddleErrCheckAndAssign is enabled this is a list of names
	// used for error variables to check for in the conditional.
	// Defaults to just "err"
	ErrorVariableNames []string

	// ForceExclusiveShortDeclarations will cause an error if a short declaration
	// (:=) cuddles with anything other than another short declaration. For example
	//
	// a := 2
	// b := 3
	//
	// is allowed, but
	//
	// a := 2
	// b = 3
	//
	// is not allowed. This logic overrides ForceCuddleErrCheckAndAssign among others.
	ForceExclusiveShortDeclarations bool

	// IncludeGenerated will include generated files in the analysis and report
	// errors even for generated files. Can be useful when developing
	// generators.
	IncludeGenerated bool

	// AllowCuddleUsedInBlock will allowing cuddling of variables with block statements
	// if they are used anywhere in the block. This defaults to false but setting
	// it to true will allow the following example:
	//
	// var numbers []int
	// for i := 0; i < 10; i++ {
	// 	if 1 == 1 {
	// 		numbers = append(numbers, i)
	// 	}
	// }
	AllowCuddleUsedInBlock bool
}

// fix is a range to fixup.
type fix struct {
	fixRangeStart token.Pos
	fixRangeEnd   token.Pos
}

// result represents the result of one error.
type result struct {
	fixRanges []fix
	reason    string
}

// processor is the type that keeps track of the file and fileset and holds the
// results from parsing the AST.
type processor struct {
	config   *Configuration
	file     *ast.File
	fileSet  *token.FileSet
	result   map[token.Pos]result
	warnings []string
}

// newProcessorWithConfig will create a Processor with the passed configuration.
func newProcessorWithConfig(file *ast.File, fileSet *token.FileSet, cfg *Configuration) *processor {
	return &processor{
		config:  cfg,
		file:    file,
		fileSet: fileSet,
		result:  make(map[token.Pos]result),
	}
}

// parseAST will parse the AST attached to the Processor instance.
func (p *processor) parseAST() {
	for _, d := range p.file.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			p.parseBlockBody(v.Name, v.Body)
		case *ast.GenDecl:
			// `go fmt` will handle proper spacing for GenDecl such as imports,
			// constants etc.
		default:
			p.addWarning(warnTypeNotImplement, d.Pos(), v)
		}
	}
}

// parseBlockBody will parse any kind of block statements such as switch cases
// and if statements. A list of Result is returned.
func (p *processor) parseBlockBody(ident *ast.Ident, block *ast.BlockStmt) {
	// Nothing to do if there's no value.
	if reflect.ValueOf(block).IsNil() {
		return
	}

	// Start by finding leading and trailing whitespaces.
	p.findLeadingAndTrailingWhitespaces(ident, block, nil)

	// Parse the block body contents.
	p.parseBlockStatements(block.List)
}

// parseBlockStatements will parse all the statements found in the body of a
// node. A list of Result is returned.
func (p *processor) parseBlockStatements(statements []ast.Stmt) {
	for i, stmt := range statements {
		// Start by checking if this statement is another block (other than if,
		// for and range). This could be assignment to a function, defer or go
		// call with an inline function or similar. If this is found we start by
		// parsing this body block before moving on.
		for _, stmtBlocks := range p.findBlockStmt(stmt) {
			p.parseBlockBody(nil, stmtBlocks)
		}

		firstBodyStatement := p.firstBodyStatement(i, statements)

		// First statement, nothing to do.
		if i == 0 {
			continue
		}

		previousStatement := statements[i-1]
		previousStatementIsMultiline := p.nodeStart(previousStatement) != p.nodeEnd(previousStatement)
		cuddledWithLastStmt := p.nodeEnd(previousStatement) == p.nodeStart(stmt)-1

		// If we're not cuddled and we don't need to enforce err-check cuddling
		// then we can bail out here
		if !cuddledWithLastStmt && !p.config.ForceCuddleErrCheckAndAssign {
			continue
		}

		// We don't force error cuddling for multilines. (#86)
		if p.config.ForceCuddleErrCheckAndAssign && previousStatementIsMultiline && !cuddledWithLastStmt {
			continue
		}

		// Extract assigned variables on the line above
		// which is the only thing we allow cuddling with. If the assignment is
		// made over multiple lines we should not allow cuddling.
		var assignedOnLineAbove []string

		// We want to keep track of what was called on the line above to support
		// special handling of things such as mutexes.
		var calledOnLineAbove []string

		// Check if the previous statement spans over multiple lines.
		cuddledWithMultiLineAssignment := cuddledWithLastStmt && p.nodeStart(previousStatement) != p.nodeStart(stmt)-1

		// Ensure previous line is not a multi line assignment and if not get
		// rightAndLeftHandSide assigned variables.
		if !cuddledWithMultiLineAssignment {
			assignedOnLineAbove = p.findLHS(previousStatement)
			calledOnLineAbove = p.findRHS(previousStatement)
		}

		// If previous assignment is multi line and we allow it, fetch
		// assignments (but only assignments).
		if cuddledWithMultiLineAssignment && p.config.AllowMultiLineAssignCuddle {
			if _, ok := previousStatement.(*ast.AssignStmt); ok {
				assignedOnLineAbove = p.findLHS(previousStatement)
			}
		}

		// Contains the union of all variable names used anywhere
		// within the block body (if applicable) and is used to check
		// if a preceding statement's variables are actually used within
		// the block. This helps enforce rules about allowed cuddling.
		var identifiersUsedInBlock []string

		if firstBodyStatement != nil {
			if p.config.AllowCuddleUsedInBlock {
				identifiersUsedInBlock = p.findUsedVariablesInStatement(stmt)
			} else {
				identifiersUsedInBlock = append(p.findLHS(firstBodyStatement), p.findRHS(firstBodyStatement)...)
			}
		}

		var (
			leftHandSide                = p.findLHS(stmt)
			rightHandSide               = p.findRHS(stmt)
			rightAndLeftHandSide        = append(leftHandSide, rightHandSide...)
			calledOrAssignedOnLineAbove = append(calledOnLineAbove, assignedOnLineAbove...)
		)

		// If we called some kind of lock on the line above we allow cuddling
		// anything.
		if atLeastOneInListsMatch(calledOnLineAbove, p.config.AllowCuddleWithCalls) {
			continue
		}

		// If we call some kind of unlock on this line we allow cuddling with
		// anything.
		if atLeastOneInListsMatch(rightHandSide, p.config.AllowCuddleWithRHS) {
			continue
		}

		nStatementsBefore := func(n int) bool {
			if i < n {
				return false
			}

			for j := 1; j < n; j++ {
				s1 := statements[i-j]
				s2 := statements[i-(j+1)]

				if p.nodeStart(s1)-1 != p.nodeEnd(s2) {
					return false
				}
			}

			return true
		}

		nStatementsAfter := func(n int) bool {
			if len(statements)-1 < i+n {
				return false
			}

			for j := range n {
				s1 := statements[i+j]
				s2 := statements[i+j+1]

				if p.nodeEnd(s1)+1 != p.nodeStart(s2) {
					return false
				}
			}

			return true
		}

		isLastStatementInBlockOfOnlyTwoLines := func() bool {
			// If we're the last statement, check if there's no more than two
			// lines from the starting statement and the end of this statement.
			// This is to support short return functions such as:
			// func (t *Typ) X() {
			//     t.X = true
			//     return t
			// }
			if len(statements) == 2 && i == 1 {
				if p.nodeEnd(stmt)-p.nodeStart(previousStatement) <= 2 {
					return true
				}
			}

			return false
		}

		// If it's a short declaration we should not cuddle with anything else
		// if ForceExclusiveShortDeclarations is set on; either this or the
		// previous statement could be the short decl, so we'll find out which
		// it was and use *that* statement's position
		if p.config.ForceExclusiveShortDeclarations && cuddledWithLastStmt {
			if p.isShortDecl(stmt) && !p.isShortDecl(previousStatement) {
				var reportNode ast.Node = previousStatement

				cm := ast.NewCommentMap(p.fileSet, stmt, p.file.Comments)
				if cg, ok := cm[stmt]; ok && len(cg) > 0 {
					for _, c := range cg {
						if c.Pos() > previousStatement.End() && c.End() < stmt.Pos() {
							reportNode = c
						}
					}
				}

				p.addErrorRange(
					stmt.Pos(),
					reportNode.End(),
					reportNode.End(),
					reasonShortDeclNotExclusive,
				)
			} else if p.isShortDecl(previousStatement) && !p.isShortDecl(stmt) {
				p.addErrorRange(
					previousStatement.Pos(),
					stmt.Pos(),
					stmt.Pos(),
					reasonShortDeclNotExclusive,
				)
			}
		}

		// If it's not an if statement and we're not cuddled move on. The only
		// reason we need to keep going for if statements is to check if we
		// should be cuddled with an error check.
		if _, ok := stmt.(*ast.IfStmt); !ok {
			if !cuddledWithLastStmt {
				continue
			}
		}

		reportNewlineTwoLinesAbove := func(n1, n2 ast.Node, reason string) {
			if atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) ||
				atLeastOneInListsMatch(assignedOnLineAbove, identifiersUsedInBlock) {
				// If both the assignment on the line above _and_ the assignment
				// two lines above is part of line or first in block, add the
				// newline as if non were.
				_, isAssignmentTwoLinesAbove := statements[i-2].(*ast.AssignStmt)
				assignedTwoLinesAbove := p.findLHS(statements[i-2])

				if isAssignmentTwoLinesAbove &&
					(atLeastOneInListsMatch(rightAndLeftHandSide, assignedTwoLinesAbove) ||
						atLeastOneInListsMatch(assignedTwoLinesAbove, identifiersUsedInBlock)) {
					p.addWhitespaceBeforeError(n1, reason)
				} else {
					// If the variable on the line above is allowed to be
					// cuddled, break two lines above so we keep the proper
					// cuddling.
					p.addErrorRange(n1.Pos(), n2.Pos(), n2.Pos(), reason)
				}
			} else {
				// If not, break here so we separate the cuddled variable.
				p.addWhitespaceBeforeError(n1, reason)
			}
		}

		switch t := stmt.(type) {
		case *ast.IfStmt:
			checkingErrInitializedInline := func() bool {
				if t.Init == nil {
					return false
				}

				// Variables were initialized inline in the if statement
				// Let's make sure it's the err just to be safe
				return atLeastOneInListsMatch(p.findLHS(t.Init), p.config.ErrorVariableNames)
			}

			if !cuddledWithLastStmt {
				checkingErr := atLeastOneInListsMatch(rightAndLeftHandSide, p.config.ErrorVariableNames)
				if checkingErr {
					// We only want to enforce cuddling error checks if the
					// error was assigned on the line above. See
					// https://github.com/bombsimon/wsl/issues/78.
					// This is needed since `assignedOnLineAbove` is not
					// actually just assignments but everything from LHS in the
					// previous statement. This means that if previous line was
					// `if err ...`, `err` will now be in the list
					// `assignedOnLineAbove`.
					if _, ok := previousStatement.(*ast.AssignStmt); !ok {
						continue
					}

					if checkingErrInitializedInline() {
						continue
					}

					if atLeastOneInListsMatch(assignedOnLineAbove, p.config.ErrorVariableNames) {
						p.addErrorRange(
							stmt.Pos(),
							previousStatement.End(),
							stmt.Pos(),
							reasonMustCuddleErrCheck,
						)
					}
				}

				continue
			}

			if len(assignedOnLineAbove) == 0 {
				p.addWhitespaceBeforeError(t, reasonOnlyCuddleIfWithAssign)
				continue
			}

			if nStatementsBefore(2) {
				reportNewlineTwoLinesAbove(t, statements[i-1], reasonOnlyOneCuddleBeforeIf)
				continue
			}

			if atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) {
				continue
			}

			if atLeastOneInListsMatch(assignedOnLineAbove, identifiersUsedInBlock) {
				continue
			}

			p.addWhitespaceBeforeError(t, reasonOnlyCuddleWithUsedAssign)
		case *ast.ReturnStmt:
			if isLastStatementInBlockOfOnlyTwoLines() {
				continue
			}

			p.addWhitespaceBeforeError(t, reasonOnlyCuddle2LineReturn)
		case *ast.BranchStmt:
			if isLastStatementInBlockOfOnlyTwoLines() {
				continue
			}

			p.addWhitespaceBeforeError(t, reasonMultiLineBranchCuddle)
		case *ast.AssignStmt:
			// append is usually an assignment but should not be allowed to be
			// cuddled with anything not appended.
			if len(rightHandSide) > 0 && rightHandSide[len(rightHandSide)-1] == "append" {
				if p.config.StrictAppend {
					if !atLeastOneInListsMatch(calledOrAssignedOnLineAbove, rightHandSide) {
						p.addWhitespaceBeforeError(t, reasonAppendCuddledWithoutUse)
					}
				}

				continue
			}

			switch previousStatement.(type) {
			case *ast.AssignStmt, *ast.IncDecStmt:
				continue
			}

			if p.config.AllowAssignAndAnythingCuddle {
				continue
			}

			if _, ok := previousStatement.(*ast.DeclStmt); ok && p.config.AllowCuddleDeclaration {
				continue
			}

			// If the assignment is from a type or variable called on the line
			// above we can allow it by setting AllowAssignAndCallCuddle to
			// true.
			// Example (x is used):
			//  x.function()
			//  a.Field = x.anotherFunction()
			if p.config.AllowAssignAndCallCuddle {
				if atLeastOneInListsMatch(calledOrAssignedOnLineAbove, rightAndLeftHandSide) {
					continue
				}
			}

			p.addWhitespaceBeforeError(t, reasonAssignsCuddleAssign)
		case *ast.IncDecStmt:
			switch previousStatement.(type) {
			case *ast.AssignStmt, *ast.IncDecStmt:
				continue
			}

			p.addWhitespaceBeforeError(t, reasonAssignsCuddleAssign)

		case *ast.DeclStmt:
			if !p.config.AllowCuddleDeclaration {
				p.addWhitespaceBeforeError(t, reasonNeverCuddleDeclare)
			}
		case *ast.ExprStmt:
			switch previousStatement.(type) {
			case *ast.DeclStmt, *ast.ReturnStmt:
				if p.config.AllowAssignAndCallCuddle && p.config.AllowCuddleDeclaration {
					continue
				}

				p.addWhitespaceBeforeError(t, reasonExpressionCuddledWithDeclOrRet)
			case *ast.IfStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.ForStmt:
				p.addWhitespaceBeforeError(t, reasonExpressionCuddledWithBlock)
			}

			// If the expression is called on a type or variable used or
			// assigned on the line we can allow it by setting
			// AllowAssignAndCallCuddle to true.
			// Example of allowed cuddled (x is used):
			//  a.Field = x.func()
			//  x.function()
			if p.config.AllowAssignAndCallCuddle {
				if atLeastOneInListsMatch(calledOrAssignedOnLineAbove, rightAndLeftHandSide) {
					continue
				}
			}

			// If we assigned variables on the line above but didn't use them in
			// this expression there should probably be a newline between them.
			if len(assignedOnLineAbove) > 0 && !atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) {
				p.addWhitespaceBeforeError(t, reasonExprCuddlingNonAssignedVar)
			}
		case *ast.RangeStmt:
			if nStatementsBefore(2) {
				reportNewlineTwoLinesAbove(t, statements[i-1], reasonOnlyOneCuddleBeforeRange)
				continue
			}

			if !atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) {
				if !atLeastOneInListsMatch(assignedOnLineAbove, identifiersUsedInBlock) {
					p.addWhitespaceBeforeError(t, reasonRangeCuddledWithoutUse)
				}
			}
		case *ast.DeferStmt:
			if _, ok := previousStatement.(*ast.DeferStmt); ok {
				// We may cuddle multiple defers to group logic.
				continue
			}

			if nStatementsBefore(2) {
				// We allow cuddling defer if the defer references something
				// used two lines above.
				// There are several reasons to why we do this.
				// Originally there was a special use case only for "Close"
				//
				// https://github.com/bombsimon/wsl/issues/31 which links to
				//  resp, err := client.Do(req)
				//  if err != nil {
				//      return err
				//  }
				//  defer resp.Body.Close()
				//
				// After a discussion in a followup issue it makes sense to not
				// only hard code `Close` but for anything that's referenced two
				// statements above.
				//
				// https://github.com/bombsimon/wsl/issues/85
				//  db, err := OpenDB()
				//  require.NoError(t, err)
				//  defer db.Close()
				//
				// All of this is only allowed if there's exactly three cuddled
				// statements, otherwise the regular rules apply.
				if !nStatementsBefore(3) && !nStatementsAfter(1) {
					variablesTwoLinesAbove := append(p.findLHS(statements[i-2]), p.findRHS(statements[i-2])...)
					if atLeastOneInListsMatch(rightHandSide, variablesTwoLinesAbove) {
						continue
					}
				}

				reportNewlineTwoLinesAbove(t, statements[i-1], reasonOnlyOneCuddleBeforeDefer)

				continue
			}

			// Be extra nice with RHS, it's common to use this for locks:
			// m.Lock()
			// defer m.Unlock()
			previousRHS := p.findRHS(previousStatement)
			if atLeastOneInListsMatch(rightHandSide, previousRHS) {
				continue
			}

			// Allow use to cuddled defer func literals with usages on line
			// above. Example:
			// b := getB()
			// defer func() {
			//     makesSenseToUse(b)
			// }()
			if c, ok := t.Call.Fun.(*ast.FuncLit); ok {
				funcLitFirstStmt := append(p.findLHS(c.Body), p.findRHS(c.Body)...)

				if atLeastOneInListsMatch(assignedOnLineAbove, funcLitFirstStmt) {
					continue
				}
			}

			if atLeastOneInListsMatch(assignedOnLineAbove, identifiersUsedInBlock) {
				continue
			}

			if !atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) {
				p.addWhitespaceBeforeError(t, reasonDeferCuddledWithOtherVar)
			}
		case *ast.ForStmt:
			if len(rightAndLeftHandSide) == 0 && !p.config.AllowCuddleUsedInBlock {
				p.addWhitespaceBeforeError(t, reasonForWithoutCondition)
				continue
			}

			if nStatementsBefore(2) {
				reportNewlineTwoLinesAbove(t, statements[i-1], reasonOnlyOneCuddleBeforeFor)
				continue
			}

			// The same rule applies for ranges as for if statements, see
			// comments regarding variable usages on the line before or as the
			// first line in the block for details.
			if !atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) {
				if !atLeastOneInListsMatch(assignedOnLineAbove, identifiersUsedInBlock) {
					p.addWhitespaceBeforeError(t, reasonForCuddledAssignWithoutUse)
				}
			}
		case *ast.GoStmt:
			if _, ok := previousStatement.(*ast.GoStmt); ok {
				continue
			}

			if nStatementsBefore(2) {
				reportNewlineTwoLinesAbove(t, statements[i-1], reasonOnlyOneCuddleBeforeGo)
				continue
			}

			if c, ok := t.Call.Fun.(*ast.SelectorExpr); ok {
				goCallArgs := append(p.findLHS(c.X), p.findRHS(c.X)...)

				if atLeastOneInListsMatch(calledOnLineAbove, goCallArgs) {
					continue
				}
			}

			if c, ok := t.Call.Fun.(*ast.FuncLit); ok {
				goCallArgs := append(p.findLHS(c.Body), p.findRHS(c.Body)...)

				if atLeastOneInListsMatch(assignedOnLineAbove, goCallArgs) {
					continue
				}
			}

			if !atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) {
				p.addWhitespaceBeforeError(t, reasonGoFuncWithoutAssign)
			}
		case *ast.SwitchStmt:
			if nStatementsBefore(2) {
				reportNewlineTwoLinesAbove(t, statements[i-1], reasonOnlyOneCuddleBeforeSwitch)
				continue
			}

			if !atLeastOneInListsMatch(rightAndLeftHandSide, assignedOnLineAbove) {
				if len(rightAndLeftHandSide) == 0 {
					if p.config.AllowCuddleUsedInBlock {
						continue
					}

					p.addWhitespaceBeforeError(t, reasonAnonSwitchCuddled)
				} else {
					p.addWhitespaceBeforeError(t, reasonSwitchCuddledWithoutUse)
				}
			}
		case *ast.TypeSwitchStmt:
			if nStatementsBefore(2) {
				reportNewlineTwoLinesAbove(t, statements[i-1], reasonOnlyOneCuddleBeforeTypeSwitch)
				continue
			}

			// Allowed to type assert on variable assigned on line above.
			if !atLeastOneInListsMatch(rightHandSide, assignedOnLineAbove) {
				// Allow type assertion on variables used in the first case
				// immediately.
				if !atLeastOneInListsMatch(assignedOnLineAbove, identifiersUsedInBlock) {
					p.addWhitespaceBeforeError(t, reasonTypeSwitchCuddledWithoutUse)
				}
			}
		case *ast.CaseClause, *ast.CommClause:
			// Case clauses will be checked by not allowing leading of trailing
			// whitespaces within the block. There's nothing in the case itself
			// that may be cuddled.
		default:
			p.addWarning(warnStmtNotImplemented, t.Pos(), t)
		}
	}
}

// firstBodyStatement returns the first statement inside a body block. This is
// because variables may be cuddled with conditions or statements if it's used
// directly as the first argument inside a body.
// The body will then be parsed as a *ast.BlockStmt (regular block) or as a list
// of []ast.Stmt (case block).
func (p *processor) firstBodyStatement(i int, allStmt []ast.Stmt) ast.Node {
	stmt := allStmt[i]

	// Start by checking if the statement has a body (probably if-statement,
	// a range, switch case or similar. Whenever a body is found we start by
	// parsing it before moving on in the AST.
	statementBody := reflect.Indirect(reflect.ValueOf(stmt)).FieldByName("Body")

	// Some cases allow cuddling depending on the first statement in a body
	// of a block or case. If possible extract the first statement.
	var firstBodyStatement ast.Node

	if !statementBody.IsValid() {
		return firstBodyStatement
	}

	switch statementBodyContent := statementBody.Interface().(type) {
	case *ast.BlockStmt:
		if len(statementBodyContent.List) > 0 {
			firstBodyStatement = statementBodyContent.List[0]

			// If the first body statement is a *ast.CaseClause we're
			// actually interested in the **next** body to know what's
			// inside the first case.
			if x, ok := firstBodyStatement.(*ast.CaseClause); ok {
				if len(x.Body) > 0 {
					firstBodyStatement = x.Body[0]
				}
			}
		}

		// If statement bodies will be parsed already when finding block bodies.
		// The reason is because if/else-if/else chains is nested in the AST
		// where the else bit is a part of the if statement. Since if statements
		// is the only statement that can be chained like this we exclude it
		// from parsing it again here.
		if _, ok := stmt.(*ast.IfStmt); !ok {
			p.parseBlockBody(nil, statementBodyContent)
		}
	case []ast.Stmt:
		// The Body field for an *ast.CaseClause or *ast.CommClause is of type
		// []ast.Stmt. We must check leading and trailing whitespaces and then
		// pass the statements to parseBlockStatements to parse it's content.
		var nextStatement ast.Node

		// Check if there's more statements (potential cases) after the
		// current one.
		if len(allStmt)-1 > i {
			nextStatement = allStmt[i+1]
		}

		p.findLeadingAndTrailingWhitespaces(nil, stmt, nextStatement)
		p.parseBlockStatements(statementBodyContent)
	default:
		p.addWarning(
			warnBodyStmtTypeNotImplemented,
			stmt.Pos(), statementBodyContent,
		)
	}

	return firstBodyStatement
}

// findUsedVariablesInStatement processes a statement,
// returning a union of all variables used within it.
func (p *processor) findUsedVariablesInStatement(stmt ast.Stmt) []string {
	var (
		used []string
		seen = map[string]struct{}{}
	)

	// ast.Inspect walks the AST of the statement.
	ast.Inspect(stmt, func(n ast.Node) bool {
		// We're only interested in identifiers.
		if ident, ok := n.(*ast.Ident); ok {
			if _, exists := seen[ident.Name]; !exists {
				seen[ident.Name] = struct{}{}

				used = append(used, ident.Name)
			}
		}

		// Continue walking the AST.
		return true
	})

	return used
}

func (p *processor) findLHS(node ast.Node) []string {
	var lhs []string

	if node == nil {
		return lhs
	}

	switch t := node.(type) {
	case *ast.BasicLit, *ast.FuncLit, *ast.SelectStmt,
		*ast.LabeledStmt, *ast.ForStmt, *ast.SwitchStmt,
		*ast.ReturnStmt, *ast.GoStmt, *ast.CaseClause,
		*ast.CommClause, *ast.CallExpr, *ast.UnaryExpr,
		*ast.BranchStmt, *ast.TypeSpec, *ast.ChanType,
		*ast.DeferStmt, *ast.TypeAssertExpr, *ast.RangeStmt:
	// Nothing to add to LHS
	case *ast.IncDecStmt:
		return p.findLHS(t.X)
	case *ast.Ident:
		return []string{t.Name}
	case *ast.AssignStmt:
		for _, v := range t.Lhs {
			lhs = append(lhs, p.findLHS(v)...)
		}
	case *ast.GenDecl:
		for _, v := range t.Specs {
			lhs = append(lhs, p.findLHS(v)...)
		}
	case *ast.ValueSpec:
		for _, v := range t.Names {
			lhs = append(lhs, p.findLHS(v)...)
		}
	case *ast.BlockStmt:
		for _, v := range t.List {
			lhs = append(lhs, p.findLHS(v)...)
		}
	case *ast.BinaryExpr:
		return append(
			p.findLHS(t.X),
			p.findLHS(t.Y)...,
		)
	case *ast.DeclStmt:
		return p.findLHS(t.Decl)
	case *ast.IfStmt:
		return p.findLHS(t.Cond)
	case *ast.TypeSwitchStmt:
		return p.findLHS(t.Assign)
	case *ast.SendStmt:
		return p.findLHS(t.Chan)
	default:
		if x, ok := maybeX(t); ok {
			return p.findLHS(x)
		}

		p.addWarning(warnUnknownLHS, t.Pos(), t)
	}

	return lhs
}

func (p *processor) findRHS(node ast.Node) []string {
	var rhs []string

	if node == nil {
		return rhs
	}

	switch t := node.(type) {
	case *ast.BasicLit, *ast.SelectStmt, *ast.ChanType,
		*ast.LabeledStmt, *ast.DeclStmt, *ast.BranchStmt,
		*ast.TypeSpec, *ast.ArrayType, *ast.CaseClause,
		*ast.CommClause, *ast.MapType, *ast.FuncLit:
	// Nothing to add to RHS
	case *ast.Ident:
		return []string{t.Name}
	case *ast.SelectorExpr:
		// TODO: Should this be RHS?
		// t.X is needed for defer as of now and t.Sel needed for special
		// functions such as Lock()
		rhs = p.findRHS(t.X)
		rhs = append(rhs, p.findRHS(t.Sel)...)
	case *ast.AssignStmt:
		for _, v := range t.Rhs {
			rhs = append(rhs, p.findRHS(v)...)
		}
	case *ast.CallExpr:
		for _, v := range t.Args {
			rhs = append(rhs, p.findRHS(v)...)
		}

		rhs = append(rhs, p.findRHS(t.Fun)...)
	case *ast.CompositeLit:
		for _, v := range t.Elts {
			rhs = append(rhs, p.findRHS(v)...)
		}
	case *ast.IfStmt:
		rhs = append(rhs, p.findRHS(t.Cond)...)
		rhs = append(rhs, p.findRHS(t.Init)...)
	case *ast.BinaryExpr:
		return append(
			p.findRHS(t.X),
			p.findRHS(t.Y)...,
		)
	case *ast.TypeSwitchStmt:
		return p.findRHS(t.Assign)
	case *ast.ReturnStmt:
		for _, v := range t.Results {
			rhs = append(rhs, p.findRHS(v)...)
		}
	case *ast.BlockStmt:
		for _, v := range t.List {
			rhs = append(rhs, p.findRHS(v)...)
		}
	case *ast.SwitchStmt:
		return p.findRHS(t.Tag)
	case *ast.GoStmt:
		return p.findRHS(t.Call)
	case *ast.ForStmt:
		return p.findRHS(t.Cond)
	case *ast.DeferStmt:
		return p.findRHS(t.Call)
	case *ast.SendStmt:
		return p.findLHS(t.Value)
	case *ast.IndexExpr:
		rhs = append(rhs, p.findRHS(t.Index)...)
		rhs = append(rhs, p.findRHS(t.X)...)
	case *ast.SliceExpr:
		rhs = append(rhs, p.findRHS(t.X)...)
		rhs = append(rhs, p.findRHS(t.Low)...)
		rhs = append(rhs, p.findRHS(t.High)...)
	case *ast.KeyValueExpr:
		rhs = p.findRHS(t.Key)
		rhs = append(rhs, p.findRHS(t.Value)...)
	default:
		if x, ok := maybeX(t); ok {
			return p.findRHS(x)
		}

		p.addWarning(warnUnknownRHS, t.Pos(), t)
	}

	return rhs
}

func (p *processor) isShortDecl(node ast.Node) bool {
	if t, ok := node.(*ast.AssignStmt); ok {
		return t.Tok == token.DEFINE
	}

	return false
}

func (p *processor) findBlockStmt(node ast.Node) []*ast.BlockStmt {
	var blocks []*ast.BlockStmt

	switch t := node.(type) {
	case *ast.BlockStmt:
		return []*ast.BlockStmt{t}
	case *ast.AssignStmt:
		for _, x := range t.Rhs {
			blocks = append(blocks, p.findBlockStmt(x)...)
		}
	case *ast.CallExpr:
		blocks = append(blocks, p.findBlockStmt(t.Fun)...)

		for _, x := range t.Args {
			blocks = append(blocks, p.findBlockStmt(x)...)
		}
	case *ast.FuncLit:
		blocks = append(blocks, t.Body)
	case *ast.ExprStmt:
		blocks = append(blocks, p.findBlockStmt(t.X)...)
	case *ast.ReturnStmt:
		for _, x := range t.Results {
			blocks = append(blocks, p.findBlockStmt(x)...)
		}
	case *ast.DeferStmt:
		blocks = append(blocks, p.findBlockStmt(t.Call)...)
	case *ast.GoStmt:
		blocks = append(blocks, p.findBlockStmt(t.Call)...)
	case *ast.IfStmt:
		blocks = append([]*ast.BlockStmt{t.Body}, p.findBlockStmt(t.Else)...)
	}

	return blocks
}

// maybeX extracts the X field from an AST node and returns it with a true value
// if it exists. If the node doesn't have an X field nil and false is returned.
// Known fields with X that are handled:
// IndexExpr, ExprStmt, SelectorExpr, StarExpr, ParentExpr, TypeAssertExpr,
// RangeStmt, UnaryExpr, ParenExpr, SliceExpr, IncDecStmt.
func maybeX(node any) (ast.Node, bool) {
	maybeHasX := reflect.Indirect(reflect.ValueOf(node)).FieldByName("X")
	if !maybeHasX.IsValid() {
		return nil, false
	}

	n, ok := maybeHasX.Interface().(ast.Node)
	if !ok {
		return nil, false
	}

	return n, true
}

func atLeastOneInListsMatch(listOne, listTwo []string) bool {
	sliceToMap := func(s []string) map[string]struct{} {
		m := map[string]struct{}{}

		for _, v := range s {
			m[v] = struct{}{}
		}

		return m
	}

	m1 := sliceToMap(listOne)
	m2 := sliceToMap(listTwo)

	for k1 := range m1 {
		if _, ok := m2[k1]; ok {
			return true
		}
	}

	for k2 := range m2 {
		if _, ok := m1[k2]; ok {
			return true
		}
	}

	return false
}

// findLeadingAndTrailingWhitespaces will find leading and trailing whitespaces
// in a node. The method takes comments in consideration which will make the
// parser more gentle.
func (p *processor) findLeadingAndTrailingWhitespaces(ident *ast.Ident, stmt, nextStatement ast.Node) {
	var (
		commentMap      = ast.NewCommentMap(p.fileSet, stmt, p.file.Comments)
		blockStatements []ast.Stmt
		blockStartLine  int
		blockEndLine    int
		blockStartPos   token.Pos
		blockEndPos     token.Pos
		isCase          bool
	)

	// Depending on the block type, get the statements in the block and where
	// the block starts (and ends).
	switch t := stmt.(type) {
	case *ast.BlockStmt:
		blockStatements = t.List
		blockStartPos = t.Lbrace
		blockEndPos = t.Rbrace
	case *ast.CaseClause:
		blockStatements = t.Body
		blockStartPos = t.Colon
		isCase = true
	case *ast.CommClause:
		blockStatements = t.Body
		blockStartPos = t.Colon
		isCase = true
	default:
		p.addWarning(warnWSNodeTypeNotImplemented, stmt.Pos(), stmt)

		return
	}

	// Ignore empty blocks even if they have newlines or just comments.
	if len(blockStatements) < 1 {
		return
	}

	blockStartLine = p.fileSet.Position(blockStartPos).Line
	blockEndLine = p.fileSet.Position(blockEndPos).Line

	// No whitespace possible if LBrace and RBrace is on the same line.
	if blockStartLine == blockEndLine {
		return
	}

	var (
		firstStatement = blockStatements[0]
		lastStatement  = blockStatements[len(blockStatements)-1]
	)

	// Get the comment related to the first statement, we do allow comments in
	// the beginning of a block before the first statement.
	var (
		openingNodePos     = blockStartPos + 1
		lastLeadingComment ast.Node
	)

	var (
		firstStatementCommentGroups []*ast.CommentGroup
		lastStatementCommentGroups  []*ast.CommentGroup
	)

	if cg, ok := commentMap[firstStatement]; ok && !isCase {
		firstStatementCommentGroups = cg
	} else {
		// TODO: Just like with trailing whitespaces comments in a case block is
		// tied to the last token of the first statement. For now we iterate over
		// all comments in the stmt and grab those that's after colon and before
		// first statement.
		for _, cg := range commentMap {
			if len(cg) < 1 {
				continue
			}

			// If we have comments and the last comment ends before the first
			// statement and the node is after the colon, this must be the node
			// mapped to comments.
			for _, c := range cg {
				if c.End() < firstStatement.Pos() && c.Pos() > blockStartPos {
					firstStatementCommentGroups = append(firstStatementCommentGroups, c)
				}
			}

			// And same if we have comments where the first comment is after the
			// last statement but before the next statement (next case). As with
			// the other things, if there is not next statement it's no next
			// case and the logic will be handled when parsing the block.
			if nextStatement == nil {
				continue
			}

			for _, c := range cg {
				if c.Pos() > lastStatement.End() && c.End() < nextStatement.Pos() {
					lastStatementCommentGroups = append(lastStatementCommentGroups, c)
				}
			}
		}

		// Since the comments come from a map they might not be ordered meaning
		// that the last and first comment groups can be in the wrong order. We
		// fix this by sorting all comments by pos after adding them all to the
		// slice.
		sort.Slice(firstStatementCommentGroups, func(i, j int) bool {
			return firstStatementCommentGroups[i].Pos() < firstStatementCommentGroups[j].Pos()
		})

		sort.Slice(lastStatementCommentGroups, func(i, j int) bool {
			return lastStatementCommentGroups[i].Pos() < lastStatementCommentGroups[j].Pos()
		})
	}

	for _, commentGroup := range firstStatementCommentGroups {
		// If the comment group is on the same line as the block start
		// (LBrace) we should not consider it.
		if p.nodeEnd(commentGroup) == blockStartLine {
			openingNodePos = commentGroup.End()
			continue
		}

		// We only care about comments before our statement from the comment
		// map. As soon as we hit comments after our statement let's break
		// out!
		if commentGroup.Pos() > firstStatement.Pos() {
			break
		}

		// We never allow leading whitespace for the first comment.
		if lastLeadingComment == nil && p.nodeStart(commentGroup)-1 != blockStartLine {
			p.addErrorRange(
				openingNodePos,
				openingNodePos,
				commentGroup.Pos(),
				reasonBlockStartsWithWS,
			)
		}

		// If lastLeadingComment is set this is not the first comment so we
		// should remove whitespace between them if we don't explicitly
		// allow it.
		if lastLeadingComment != nil && !p.config.AllowSeparatedLeadingComment {
			if p.nodeStart(commentGroup)+1 != p.nodeEnd(lastLeadingComment) {
				p.addErrorRange(
					openingNodePos,
					lastLeadingComment.End(),
					commentGroup.Pos(),
					reasonBlockStartsWithWS,
				)
			}
		}

		lastLeadingComment = commentGroup
	}

	lastNodePos := openingNodePos
	if lastLeadingComment != nil {
		lastNodePos = lastLeadingComment.End()
		blockStartLine = p.nodeEnd(lastLeadingComment)
	}

	// Check if we have a whitespace between the last node which can be the
	// Lbrace, a comment on the same line or the last comment if we have
	// comments inside the actual block and the first statement. This is never
	// allowed.
	if p.nodeStart(firstStatement)-1 != blockStartLine {
		p.addErrorRange(
			openingNodePos,
			lastNodePos,
			firstStatement.Pos(),
			reasonBlockStartsWithWS,
		)
	}

	// If the blockEndLine is not 0 we're a regular block (not case).
	if blockEndLine != 0 {
		// We don't want to reject example functions since they have to end with
		// a comment.
		if isExampleFunc(ident) {
			return
		}

		var (
			lastNode         ast.Node = lastStatement
			trailingComments []ast.Node
		)

		// Check if we have an comments _after_ the last statement and update
		// the last node if so.
		if c, ok := commentMap[lastStatement]; ok {
			lastComment := c[len(c)-1]
			if lastComment.Pos() > lastStatement.End() && lastComment.Pos() < stmt.End() {
				lastNode = lastComment
			}
		}

		// TODO: This should be improved.
		// The trailing comments are mapped to the last statement item which can
		// be anything depending on what the last statement is.
		// In `fmt.Println("hello")`, trailing comments will be mapped to
		// `*ast.BasicLit` for the "hello" string.
		// A short term improvement can be to cache this but for now we naively
		// iterate over all items when we check a block.
		for _, commentGroups := range commentMap {
			for _, commentGroup := range commentGroups {
				if commentGroup.Pos() < lastNode.End() || commentGroup.End() > stmt.End() {
					continue
				}

				trailingComments = append(trailingComments, commentGroup)
			}
		}

		// TODO: Should this be relaxed?
		// Given the old code we only allowed trailing newline if it was
		// directly tied to the last statement so for backwards compatibility
		// we'll do the same. This means we fail all but the last whitespace
		// even when allowing trailing comments.
		for _, comment := range trailingComments {
			if p.nodeStart(comment)-p.nodeEnd(lastNode) > 1 {
				p.addErrorRange(
					blockEndPos,
					lastNode.End(),
					comment.Pos(),
					reasonBlockEndsWithWS,
				)
			}

			lastNode = comment
		}

		if !p.config.AllowTrailingComment && p.nodeEnd(stmt)-1 != p.nodeEnd(lastStatement) {
			p.addErrorRange(
				blockEndPos,
				lastNode.End(),
				stmt.End()-1,
				reasonBlockEndsWithWS,
			)
		}

		return
	}

	// Nothing to do if we're not looking for enforced newline.
	if p.config.ForceCaseTrailingWhitespaceLimit == 0 {
		return
	}

	// If we don't have any nextStatement the trailing whitespace will be
	// handled when parsing the switch. If we do have a next statement we can
	// see where it starts by getting it's colon position. We set the end of the
	// current case to the position of the next case.
	switch nextStatement.(type) {
	case *ast.CaseClause, *ast.CommClause:
	default:
		// No more cases
		return
	}

	var closingNode ast.Node = lastStatement
	for _, commentGroup := range lastStatementCommentGroups {
		// TODO: In future versions we might want to close the gaps between
		// comments. However this is not currently reported in v3 so we
		// won't add this for now.
		// if p.nodeStart(commentGroup)-1 != p.nodeEnd(closingNode) {}
		closingNode = commentGroup
	}

	totalRowsInCase := p.nodeEnd(closingNode) - blockStartLine
	if totalRowsInCase < p.config.ForceCaseTrailingWhitespaceLimit {
		return
	}

	if p.nodeEnd(closingNode)+1 == p.nodeStart(nextStatement) {
		p.addErrorRange(
			closingNode.Pos(),
			closingNode.End(),
			closingNode.End(),
			reasonCaseBlockTooCuddly,
		)
	}
}

func isExampleFunc(ident *ast.Ident) bool {
	return ident != nil && strings.HasPrefix(ident.Name, "Example")
}

func (p *processor) nodeStart(node ast.Node) int {
	return p.fileSet.Position(node.Pos()).Line
}

func (p *processor) nodeEnd(node ast.Node) int {
	line := p.fileSet.Position(node.End()).Line

	if isEmptyLabeledStmt(node) {
		return p.fileSet.Position(node.Pos()).Line
	}

	return line
}

func isEmptyLabeledStmt(node ast.Node) bool {
	v, ok := node.(*ast.LabeledStmt)
	if !ok {
		return false
	}

	_, empty := v.Stmt.(*ast.EmptyStmt)

	return empty
}

func (p *processor) addWhitespaceBeforeError(node ast.Node, reason string) {
	p.addErrorRange(node.Pos(), node.Pos(), node.Pos(), reason)
}

func (p *processor) addErrorRange(reportAt, start, end token.Pos, reason string) {
	report, ok := p.result[reportAt]
	if !ok {
		report = result{
			reason:    reason,
			fixRanges: []fix{},
		}
	}

	report.fixRanges = append(report.fixRanges, fix{
		fixRangeStart: start,
		fixRangeEnd:   end,
	})

	p.result[reportAt] = report
}

func (p *processor) addWarning(w string, pos token.Pos, t any) {
	position := p.fileSet.Position(pos)

	p.warnings = append(p.warnings,
		fmt.Sprintf("%s:%d: %s (%T)", position.Filename, position.Line, w, t),
	)
}
