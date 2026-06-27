package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// TxLeakSeverity represents the severity level of a transaction leak.
type TxLeakSeverity string

const (
	TxLeakSeverityCritical TxLeakSeverity = "critical" // Begin without any Commit/Rollback
	TxLeakSeverityHigh     TxLeakSeverity = "high"     // Begin with Commit but no Rollback in error path
	TxLeakSeverityMedium   TxLeakSeverity = "medium"   // Begin with Rollback but no Commit
	TxLeakSeverityLow      TxLeakSeverity = "low"      // Informational - potential issue
)

// TxLeakViolation represents a detected unclosed transaction.
type TxLeakViolation struct {
	Pos           token.Pos
	End           token.Pos
	Message       string
	Severity      TxLeakSeverity
	ViolationType string // violation type identifier
	TxVarName     string // Name of the transaction variable
	Suggestion    string
}

// TxState tracks the state of a transaction variable within a function.
type TxState struct {
	VarName               string
	BeginPos              token.Pos
	BeginEnd              token.Pos
	HasCommit             bool
	HasRollback           bool
	HasDefer              bool      // Rollback/Commit in defer
	HasDeferredCommit     bool      // Commit() is in defer - antipattern
	IsReturned            bool      // Transaction returned to caller
	IsReturnedInClosure   bool      // Transaction captured by returned closure
	IsCallback            bool      // Transaction used in callback pattern
	IsPassedToFunc        bool      // Transaction passed to another function
	IsSentToChannel       bool      // Transaction sent through channel (ch <- tx)
	IsStoredInStruct      bool      // Transaction stored in struct field
	IsStoredInCollection  bool      // Transaction stored in map or slice
	IsCapturedByGoroutine bool      // Transaction captured by goroutine
	IsShadowed            bool      // Variable is shadowed in inner scope
	ShadowedBy            token.Pos // Position where shadowing occurs
	HasPanicPath          bool      // Function has panic() without deferred rollback
	HasFatalPath          bool      // Function has os.Exit/log.Fatal without deferred rollback
	HasEarlyReturn        bool      // Has return before commit without defer
	CommitInConditional   bool      // Commit is inside conditional block
	CommitInSwitch        bool      // Commit is inside switch/case that might not execute
	CommitInSelect        bool      // Commit is inside select/case that might not execute
	CommitInLoop          bool      // Commit is inside loop that might not iterate
	IsReassigned          bool      // Transaction variable is reassigned
	CommitErrorIgnored    bool      // Commit() error is ignored with blank identifier
	RollbackErrorIgnored  bool      // Rollback() error is ignored with blank identifier
	HasDeferInLoop        bool      // Transaction has defer inside a loop (antipattern)
	Scope                 int       // Scope depth where transaction was created
}

// TxLeakDetector detects unclosed SQL transactions.
type TxLeakDetector struct {
	// beginMethods are methods that start a transaction
	beginMethods map[string]bool
	// commitMethods are methods that commit a transaction
	commitMethods map[string]bool
	// rollbackMethods are methods that rollback a transaction
	rollbackMethods map[string]bool
	// callbackMethods are methods that handle tx lifecycle automatically
	callbackMethods map[string]bool
	// txStates tracks all transaction states in current function
	txStates map[string]*TxState
	// scopeDepth tracks current scope depth for shadowing detection
	scopeDepth int
	// txScopes maps variable names to their scope depths for shadowing detection
	txScopes map[string][]int
}

// NewTxLeakDetector creates a new transaction leak detector.
func NewTxLeakDetector() *TxLeakDetector {
	return &TxLeakDetector{
		beginMethods: map[string]bool{
			// database/sql
			"Begin":   true,
			"BeginTx": true,
			// sqlx
			"BeginTxx":    true,
			"Beginx":      true,
			"MustBegin":   true,
			"MustBeginTx": true,
			// pgx
			"BeginFunc": true, // callback pattern - also in callbackMethods
			// bun
			"RunInTx": true, // callback pattern - also in callbackMethods
			// ent ORM
			"Tx": true,
			// General
			"NewTx": true,
		},
		commitMethods: map[string]bool{
			"Commit": true,
		},
		rollbackMethods: map[string]bool{
			"Rollback": true,
		},
		callbackMethods: map[string]bool{
			// These handle tx lifecycle automatically
			"Transaction":      true,
			"RunInTransaction": true,
			"BeginFunc":        true,
			"BeginTxFunc":      true, // pgx conn.BeginTxFunc
			"RunInTx":          true,
			"WithTx":           true,
			"InTransaction":    true,
			"Transactional":    true,
			"ExecTx":           true,
			"DoInTx":           true,
		},
		txStates: make(map[string]*TxState),
		txScopes: make(map[string][]int),
	}
}

// CheckTxLeaks analyzes a file for unclosed transaction patterns.
func (d *TxLeakDetector) CheckTxLeaks(pass *analysis.Pass, file *ast.File) []TxLeakViolation {
	var violations []TxLeakViolation

	// Analyze each function declaration
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			continue
		}

		// Skip test functions (TestXxx, BenchmarkXxx, FuzzXxx, ExampleXxx)
		if isTestFunction(funcDecl) {
			continue
		}

		// Skip test helper functions (functions with *testing.T as first param)
		if isTestHelperFunction(funcDecl) {
			continue
		}

		violations = append(violations, d.analyzeFunction(funcDecl)...)
	}

	return violations
}

// isTestFunction checks if a function declaration is a test function.
// A test function has a name starting with Test/Benchmark/Fuzz/Example AND
// the appropriate parameter signature (or no params for Example).
func isTestFunction(funcDecl *ast.FuncDecl) bool {
	name := funcDecl.Name.Name

	// Check for Example functions (no required parameters)
	if strings.HasPrefix(name, "Example") {
		return true
	}

	// Check for Test, Benchmark, Fuzz functions
	isTestPrefix := strings.HasPrefix(name, "Test") ||
		strings.HasPrefix(name, "Benchmark") ||
		strings.HasPrefix(name, "Fuzz")

	if !isTestPrefix {
		return false
	}

	// Must have at least one parameter
	if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) == 0 {
		return false
	}

	// Check first parameter type
	firstParam := funcDecl.Type.Params.List[0]
	if firstParam.Type == nil {
		return false
	}

	// Check for *testing.T, *testing.B, *testing.F
	starExpr, ok := firstParam.Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	selExpr, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name != "testing" {
		return false
	}

	expectedType := ""
	if strings.HasPrefix(name, "Test") {
		expectedType = "T"
	} else if strings.HasPrefix(name, "Benchmark") {
		expectedType = "B"
	} else if strings.HasPrefix(name, "Fuzz") {
		expectedType = "F"
	}

	return selExpr.Sel.Name == expectedType
}

// isTestHelperFunction checks if a function is a test helper by looking at its parameters.
// Test helpers typically take *testing.T, *testing.B, or *testing.F as the first parameter.
func isTestHelperFunction(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) == 0 {
		return false
	}

	// Check first parameter
	firstParam := funcDecl.Type.Params.List[0]
	if firstParam.Type == nil {
		return false
	}

	// Check for *testing.T, *testing.B, *testing.F
	starExpr, ok := firstParam.Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	selExpr, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name == "testing" {
		switch selExpr.Sel.Name {
		case "T", "B", "F", "M":
			return true
		}
	}

	return false
}

// analyzeFunction analyzes a single function for transaction leaks.
func (d *TxLeakDetector) analyzeFunction(funcDecl *ast.FuncDecl) []TxLeakViolation {
	// Reset state for each function
	d.txStates = make(map[string]*TxState)
	d.txScopes = make(map[string][]int)
	d.scopeDepth = 0

	// Phase 1: Find transaction Begin points with scope tracking
	d.findTransactionBeginsWithScope(funcDecl)

	if len(d.txStates) == 0 {
		return nil
	}

	// Phase 2: Check for callback patterns (handled automatically)
	d.detectCallbackPatterns(funcDecl)

	// Phase 3: Track Commit/Rollback calls
	d.trackCommitRollback(funcDecl)

	// Phase 4: Check for returned transactions
	d.checkReturnStatements(funcDecl)

	// Phase 5: Check for transactions passed to other functions
	d.checkFunctionParameters(funcDecl)

	// Phase 6: Check for transactions stored in struct fields
	d.checkStructFieldStorage(funcDecl)

	// Phase 6.5: Check for transactions sent through channels
	d.checkChannelSend(funcDecl)

	// Phase 7: Check for goroutine captures
	d.checkGoroutineCaptures(funcDecl)

	// Phase 8: Check for panic paths without defer
	d.checkPanicPaths(funcDecl)

	// Phase 9: Check for early returns without defer
	d.checkEarlyReturns(funcDecl)

	// Phase 10: Check for conditional commits
	d.checkConditionalCommits(funcDecl)

	// Phase 11: Check for switch/case commits
	d.checkSwitchCaseCommits(funcDecl)

	// Phase 12: Check for select/case commits
	d.checkSelectCaseCommits(funcDecl)

	// Phase 13: Check for os.Exit/log.Fatal paths
	d.checkFatalPaths(funcDecl)

	// Phase 14: Check for commit in loops
	d.checkLoopCommits(funcDecl)

	// Phase 15: Check for variable reassignment
	d.checkVariableReassignment(funcDecl)

	// Phase 16: Check for ignored commit errors
	d.checkCommitErrorIgnored(funcDecl)

	// Phase 17: Check for ignored rollback errors
	d.checkRollbackErrorIgnored(funcDecl)

	// Phase 18: Check for defer inside loop (antipattern)
	d.checkDeferInLoop(funcDecl)

	// Phase 19: Generate violations
	return d.generateViolations()
}

// findTransactionBeginsWithScope finds all transaction Begin calls with scope tracking.
func (d *TxLeakDetector) findTransactionBeginsWithScope(funcDecl *ast.FuncDecl) {
	// First pass: find all transaction begins and their scopes
	d.findBeginsRecursive(funcDecl.Body, 0)
}

// findBeginsRecursive recursively finds Begin calls with scope tracking.
func (d *TxLeakDetector) findBeginsRecursive(node ast.Node, depth int) {
	if node == nil {
		return
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.BlockStmt:
			// Process block with increased depth
			for _, s := range stmt.List {
				d.findBeginsRecursive(s, depth+1)
			}
			return false // Don't recurse further, we handled it

		case *ast.IfStmt:
			// Process if statement parts with proper scoping
			// The if body creates a new scope
			if stmt.Init != nil {
				d.findBeginsRecursive(stmt.Init, depth)
			}
			d.findBeginsRecursive(stmt.Body, depth+1) // if body is a new scope
			if stmt.Else != nil {
				d.findBeginsRecursive(stmt.Else, depth+1) // else is also a new scope
			}
			return false

		case *ast.ForStmt:
			if stmt.Init != nil {
				d.findBeginsRecursive(stmt.Init, depth)
			}
			d.findBeginsRecursive(stmt.Body, depth+1) // for body is a new scope
			return false

		case *ast.RangeStmt:
			d.findBeginsRecursive(stmt.Body, depth+1) // range body is a new scope
			return false

		case *ast.AssignStmt:
			d.processAssignmentWithDepth(stmt, depth)
			return true
		}
		return true
	})
}

// processAssignmentWithDepth processes an assignment with explicit depth.
func (d *TxLeakDetector) processAssignmentWithDepth(assignStmt *ast.AssignStmt, depth int) {
	for i, rhs := range assignStmt.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}

		if d.isBeginCall(call) {
			// Skip if this is a callback pattern method
			if d.isCallbackMethod(call) {
				continue
			}

			// Extract variable name from LHS
			if i < len(assignStmt.Lhs) {
				if ident, ok := assignStmt.Lhs[i].(*ast.Ident); ok {
					// Skip error variables
					if ident.Name == "err" || ident.Name == "_" {
						continue
					}

					varName := ident.Name

					// Check for shadowing - if variable exists in outer scope
					if existingScopes, exists := d.txScopes[varName]; exists {
						for _, existingScope := range existingScopes {
							if existingScope < depth {
								// This is shadowing - mark the outer transaction
								// Find the state for the outer variable
								for key, state := range d.txStates {
									if state.VarName == varName && state.Scope == existingScope {
										state.IsShadowed = true
										state.ShadowedBy = assignStmt.Pos()
										_ = key // used in iteration
										break
									}
								}
							}
						}
					}

					// Track scope for this variable
					d.txScopes[varName] = append(d.txScopes[varName], depth)

					// Create unique key for this transaction
					key := d.createTxKey(varName, depth, assignStmt.Pos())
					d.txStates[key] = &TxState{
						VarName:  varName,
						BeginPos: assignStmt.Pos(),
						BeginEnd: assignStmt.End(),
						Scope:    depth,
					}
				}
			}
		}
	}
}

// createTxKey creates a unique key for a transaction state.
func (d *TxLeakDetector) createTxKey(varName string, scope int, pos token.Pos) string {
	// Use position to create unique key for shadowed variables
	return fmt.Sprintf("%s_%d_%d", varName, scope, pos)
}

// isBeginCall checks if a call expression is a transaction Begin method.
func (d *TxLeakDetector) isBeginCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return d.beginMethods[sel.Sel.Name]
}

// isCallbackMethod checks if a call expression is a callback-based transaction method.
func (d *TxLeakDetector) isCallbackMethod(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return d.callbackMethods[sel.Sel.Name]
}

// detectCallbackPatterns marks transactions that are used in callback patterns.
func (d *TxLeakDetector) detectCallbackPatterns(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check if this is a callback transaction method
		if d.callbackMethods[sel.Sel.Name] {
			// Mark any tx parameters in the callback as handled
			for _, arg := range call.Args {
				if funcLit, ok := arg.(*ast.FuncLit); ok {
					// Check parameters of the callback function
					for _, param := range funcLit.Type.Params.List {
						for _, name := range param.Names {
							d.markAllStatesWithName(name.Name, func(s *TxState) {
								s.IsCallback = true
							})
						}
					}
				}
			}
		}

		return true
	})
}

// markAllStatesWithName applies a function to all states with the given variable name.
func (d *TxLeakDetector) markAllStatesWithName(varName string, fn func(*TxState)) {
	for _, state := range d.txStates {
		if state.VarName == varName {
			fn(state)
		}
	}
}

// trackCommitRollback scans for Commit and Rollback calls on tracked transaction variables.
func (d *TxLeakDetector) trackCommitRollback(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GoStmt:
			// Skip goroutines - defers inside goroutines don't protect the main function
			// We track goroutine captures separately in checkGoroutineCaptures
			return false

		case *ast.DeferStmt:
			// Check defer statements
			d.checkDeferStatement(node)
			return true

		case *ast.CallExpr:
			// Check direct Commit/Rollback calls
			d.checkCommitRollbackCall(node, false)
			return true
		}
		return true
	})
}

// checkDeferStatement checks a defer statement for Commit/Rollback calls.
func (d *TxLeakDetector) checkDeferStatement(deferStmt *ast.DeferStmt) {
	call := deferStmt.Call
	// Check for direct defer tx.Rollback()
	d.checkCommitRollbackCall(call, true)

	// Check for defer func() { ... }()
	if funcLit, ok := call.Fun.(*ast.FuncLit); ok {
		d.checkDeferredClosure(funcLit)
	}

	// Check for defer someFunc(tx) - function call with tx as argument
	d.checkDeferredFunctionCall(call)
}

// checkDeferredFunctionCall checks if a deferred function call takes a transaction as argument.
// This handles patterns like: defer cleanup(tx) where cleanup() might do Rollback.
func (d *TxLeakDetector) checkDeferredFunctionCall(call *ast.CallExpr) {
	// Check each argument to see if it's a tracked transaction variable
	for _, arg := range call.Args {
		switch a := arg.(type) {
		case *ast.Ident:
			// Direct variable: defer cleanup(tx)
			d.markAllStatesWithName(a.Name, func(s *TxState) {
				s.HasDefer = true
				// We assume the function handles rollback since tx is passed to deferred call
			})
		case *ast.UnaryExpr:
			// Pointer: defer cleanup(&tx)
			if ident, ok := a.X.(*ast.Ident); ok {
				d.markAllStatesWithName(ident.Name, func(s *TxState) {
					s.HasDefer = true
				})
			}
		}
	}
}

// checkDeferredClosure checks a deferred closure for Commit/Rollback calls.
func (d *TxLeakDetector) checkDeferredClosure(funcLit *ast.FuncLit) {
	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			d.checkCommitRollbackCall(call, true)
		}
		return true
	})
}

// checkCommitRollbackCall checks if a call is Commit or Rollback on a tracked transaction.
func (d *TxLeakDetector) checkCommitRollbackCall(call *ast.CallExpr, inDefer bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	methodName := sel.Sel.Name

	// Get the variable name being called on
	var varName string
	if ident, ok := sel.X.(*ast.Ident); ok {
		varName = ident.Name
	}

	if varName == "" {
		return
	}

	// Mark all states with this variable name
	d.markAllStatesWithName(varName, func(state *TxState) {
		if d.commitMethods[methodName] {
			state.HasCommit = true
			// Deferred commit is an antipattern
			if inDefer {
				state.HasDeferredCommit = true
			}
		}

		if d.rollbackMethods[methodName] {
			state.HasRollback = true
			if inDefer {
				state.HasDefer = true
			}
		}
	})
}

// checkReturnStatements checks if any transaction variable is returned.
func (d *TxLeakDetector) checkReturnStatements(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		retStmt, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for _, result := range retStmt.Results {
			switch r := result.(type) {
			case *ast.Ident:
				d.markAllStatesWithName(r.Name, func(s *TxState) {
					s.IsReturned = true
				})
			case *ast.UnaryExpr:
				// Handle &tx case (returning pointer)
				if ident, ok := r.X.(*ast.Ident); ok {
					d.markAllStatesWithName(ident.Name, func(s *TxState) {
						s.IsReturned = true
					})
				}
			case *ast.FuncLit:
				// Handle closure return: return func() { tx.Commit() }
				// Check if closure captures any tracked tx variables
				d.checkClosureCaptures(r)
			}
		}
		return true
	})
}

// checkClosureCaptures checks if a closure captures any tracked transaction variables.
func (d *TxLeakDetector) checkClosureCaptures(funcLit *ast.FuncLit) {
	ast.Inspect(funcLit.Body, func(inner ast.Node) bool {
		if ident, ok := inner.(*ast.Ident); ok {
			d.markAllStatesWithName(ident.Name, func(s *TxState) {
				s.IsReturnedInClosure = true
			})
		}
		return true
	})
}

// checkFunctionParameters checks if transaction is passed to another function.
func (d *TxLeakDetector) checkFunctionParameters(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Skip if this is a Commit/Rollback call
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if d.commitMethods[sel.Sel.Name] || d.rollbackMethods[sel.Sel.Name] {
				return true
			}
		}

		// Check each argument
		for _, arg := range call.Args {
			switch a := arg.(type) {
			case *ast.Ident:
				d.markAllStatesWithName(a.Name, func(s *TxState) {
					s.IsPassedToFunc = true
				})
			case *ast.UnaryExpr:
				if ident, ok := a.X.(*ast.Ident); ok {
					d.markAllStatesWithName(ident.Name, func(s *TxState) {
						s.IsPassedToFunc = true
					})
				}
			}
		}
		return true
	})
}

// checkStructFieldStorage checks if transaction is stored in a struct field.
func (d *TxLeakDetector) checkStructFieldStorage(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		assignStmt, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, lhs := range assignStmt.Lhs {
			// Check for struct field assignment: s.tx = tx
			if sel, ok := lhs.(*ast.SelectorExpr); ok {
				_ = sel // We have a selector on the left side
				if i < len(assignStmt.Rhs) {
					if ident, ok := assignStmt.Rhs[i].(*ast.Ident); ok {
						d.markAllStatesWithName(ident.Name, func(s *TxState) {
							s.IsStoredInStruct = true
						})
					}
				}
			}

			// Check for map/slice assignment: txMap[key] = tx or txSlice[i] = tx
			if indexExpr, ok := lhs.(*ast.IndexExpr); ok {
				_ = indexExpr
				if i < len(assignStmt.Rhs) {
					if ident, ok := assignStmt.Rhs[i].(*ast.Ident); ok {
						d.markAllStatesWithName(ident.Name, func(s *TxState) {
							s.IsStoredInCollection = true
						})
					}
				}
			}
		}

		// Check for append: txSlice = append(txSlice, tx)
		for _, rhs := range assignStmt.Rhs {
			if call, ok := rhs.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "append" {
					// Check arguments after the first one
					for j := 1; j < len(call.Args); j++ {
						if argIdent, ok := call.Args[j].(*ast.Ident); ok {
							d.markAllStatesWithName(argIdent.Name, func(s *TxState) {
								s.IsStoredInCollection = true
							})
						}
					}
				}
			}
		}

		return true
	})
}

// checkChannelSend checks if transaction is sent through a channel.
func (d *TxLeakDetector) checkChannelSend(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		sendStmt, ok := n.(*ast.SendStmt)
		if !ok {
			return true
		}

		// Check if value being sent is a tracked transaction: ch <- tx
		if ident, ok := sendStmt.Value.(*ast.Ident); ok {
			d.markAllStatesWithName(ident.Name, func(s *TxState) {
				s.IsSentToChannel = true
			})
		}

		// Also check for pointer: ch <- &tx
		if unary, ok := sendStmt.Value.(*ast.UnaryExpr); ok {
			if ident, ok := unary.X.(*ast.Ident); ok {
				d.markAllStatesWithName(ident.Name, func(s *TxState) {
					s.IsSentToChannel = true
				})
			}
		}

		return true
	})
}

// checkGoroutineCaptures checks if transaction is captured by a goroutine.
func (d *TxLeakDetector) checkGoroutineCaptures(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		goStmt, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}

		// Check what variables the goroutine captures
		ast.Inspect(goStmt.Call, func(inner ast.Node) bool {
			if ident, ok := inner.(*ast.Ident); ok {
				d.markAllStatesWithName(ident.Name, func(s *TxState) {
					s.IsCapturedByGoroutine = true
				})
			}
			return true
		})

		return true
	})
}

// checkPanicPaths checks if function has panic() without deferred rollback.
func (d *TxLeakDetector) checkPanicPaths(funcDecl *ast.FuncDecl) {
	hasPanic := false
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if ident, ok := call.Fun.(*ast.Ident); ok {
			if ident.Name == "panic" {
				hasPanic = true
			}
		}
		return true
	})

	if hasPanic {
		for _, state := range d.txStates {
			if !state.HasDefer {
				state.HasPanicPath = true
			}
		}
	}
}

// checkEarlyReturns checks for early returns before commit without defer.
func (d *TxLeakDetector) checkEarlyReturns(funcDecl *ast.FuncDecl) {
	// Track position of each tx Begin
	txPositions := make(map[string]token.Pos)
	for name, state := range d.txStates {
		txPositions[name] = state.BeginPos
	}

	var returnPositions []token.Pos
	var commitPositions []token.Pos

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ReturnStmt:
			returnPositions = append(returnPositions, node.Pos())
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if d.commitMethods[sel.Sel.Name] {
					commitPositions = append(commitPositions, node.Pos())
				}
			}
		}
		return true
	})

	// Check if there are returns before any commit
	for _, state := range d.txStates {
		if state.HasDefer {
			continue // Defer handles early returns
		}

		for _, retPos := range returnPositions {
			// If return is after Begin but before Commit
			hasCommitBefore := false
			for _, commitPos := range commitPositions {
				if commitPos < retPos {
					hasCommitBefore = true
					break
				}
			}
			if retPos > state.BeginPos && !hasCommitBefore {
				state.HasEarlyReturn = true
				break
			}
		}
	}
}

// checkSwitchCaseCommits checks if Commit is inside switch/case that might not execute.
func (d *TxLeakDetector) checkSwitchCaseCommits(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switchStmt, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}

		// Count cases with commit
		casesWithCommit := 0
		totalCases := 0
		hasDefault := false

		for _, stmt := range switchStmt.Body.List {
			caseClause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			totalCases++

			if caseClause.List == nil {
				hasDefault = true
			}

			hasCommitInCase := false
			ast.Inspect(caseClause, func(inner ast.Node) bool {
				if call, ok := inner.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if d.commitMethods[sel.Sel.Name] {
							hasCommitInCase = true
						}
					}
				}
				return true
			})

			if hasCommitInCase {
				casesWithCommit++
			}
		}

		// If commit is not in all cases (including default), it's problematic
		if casesWithCommit > 0 && (casesWithCommit < totalCases || !hasDefault) {
			for _, state := range d.txStates {
				if state.HasCommit && !state.HasDefer {
					state.CommitInSwitch = true
				}
			}
		}

		return true
	})
}

// checkSelectCaseCommits checks if Commit is inside select/case that might not execute.
func (d *TxLeakDetector) checkSelectCaseCommits(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		selectStmt, ok := n.(*ast.SelectStmt)
		if !ok {
			return true
		}

		// Count cases with commit
		casesWithCommit := 0
		totalCases := 0
		hasDefault := false

		for _, stmt := range selectStmt.Body.List {
			commClause, ok := stmt.(*ast.CommClause)
			if !ok {
				continue
			}
			totalCases++

			if commClause.Comm == nil {
				hasDefault = true
			}

			hasCommitInCase := false
			for _, bodyStmt := range commClause.Body {
				ast.Inspect(bodyStmt, func(inner ast.Node) bool {
					if call, ok := inner.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if d.commitMethods[sel.Sel.Name] {
								hasCommitInCase = true
							}
						}
					}
					return true
				})
			}

			if hasCommitInCase {
				casesWithCommit++
			}
		}

		// If commit is not in all cases (including default), it's problematic
		if casesWithCommit > 0 && (casesWithCommit < totalCases || !hasDefault) {
			for _, state := range d.txStates {
				if state.HasCommit && !state.HasDefer {
					state.CommitInSelect = true
				}
			}
		}

		return true
	})
}

// checkFatalPaths checks if function has os.Exit or log.Fatal without deferred rollback.
func (d *TxLeakDetector) checkFatalPaths(funcDecl *ast.FuncDecl) {
	hasFatal := false
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for os.Exit
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				// os.Exit, log.Fatal, log.Fatalf, log.Fatalln
				if (ident.Name == "os" && sel.Sel.Name == "Exit") ||
					(ident.Name == "log" && strings.HasPrefix(sel.Sel.Name, "Fatal")) {
					hasFatal = true
				}
			}
		}

		return true
	})

	if hasFatal {
		for _, state := range d.txStates {
			if !state.HasDefer {
				state.HasFatalPath = true
			}
		}
	}
}

// checkLoopCommits checks if Commit is inside a loop that might not iterate.
func (d *TxLeakDetector) checkLoopCommits(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		var loopBody *ast.BlockStmt

		switch stmt := n.(type) {
		case *ast.ForStmt:
			loopBody = stmt.Body
		case *ast.RangeStmt:
			loopBody = stmt.Body
		default:
			return true
		}

		if loopBody == nil {
			return true
		}

		// Check if Commit is inside the loop body
		hasCommitInLoop := false
		ast.Inspect(loopBody, func(inner ast.Node) bool {
			if call, ok := inner.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if d.commitMethods[sel.Sel.Name] {
						hasCommitInLoop = true
					}
				}
			}
			return true
		})

		if hasCommitInLoop {
			for _, state := range d.txStates {
				if state.HasCommit && !state.HasDefer {
					state.CommitInLoop = true
				}
			}
		}

		return true
	})
}

// checkVariableReassignment checks if transaction variable is reassigned.
func (d *TxLeakDetector) checkVariableReassignment(funcDecl *ast.FuncDecl) {
	// Track all Begin calls per variable name
	beginCounts := make(map[string]int)

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		assignStmt, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, rhs := range assignStmt.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}

			if d.isBeginCall(call) {
				if i < len(assignStmt.Lhs) {
					if ident, ok := assignStmt.Lhs[i].(*ast.Ident); ok {
						if ident.Name != "err" && ident.Name != "_" {
							beginCounts[ident.Name]++
						}
					}
				}
			}
		}
		return true
	})

	// Mark variables that have multiple Begin calls as reassigned
	for varName, count := range beginCounts {
		if count > 1 {
			d.markAllStatesWithName(varName, func(s *TxState) {
				s.IsReassigned = true
			})
		}
	}
}

// checkCommitErrorIgnored checks if Commit() error is ignored with blank identifier.
func (d *TxLeakDetector) checkCommitErrorIgnored(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		assignStmt, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		// Look for: _ = tx.Commit() or result, _ := tx.Commit()
		for i, rhs := range assignStmt.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}

			if d.commitMethods[sel.Sel.Name] {
				// Check if the error is ignored
				// For single assignment: _ = tx.Commit()
				if len(assignStmt.Lhs) == 1 {
					if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok && ident.Name == "_" {
						if varIdent, ok := sel.X.(*ast.Ident); ok {
							d.markAllStatesWithName(varIdent.Name, func(s *TxState) {
								s.CommitErrorIgnored = true
							})
						}
					}
				}
				// For tuple assignment: result, _ := someFunc() - check if error position is _
				if len(assignStmt.Lhs) > i+1 {
					if ident, ok := assignStmt.Lhs[i+1].(*ast.Ident); ok && ident.Name == "_" {
						if varIdent, ok := sel.X.(*ast.Ident); ok {
							d.markAllStatesWithName(varIdent.Name, func(s *TxState) {
								s.CommitErrorIgnored = true
							})
						}
					}
				}
			}
		}
		return true
	})
}

// checkRollbackErrorIgnored checks if Rollback() error is ignored with blank identifier.
func (d *TxLeakDetector) checkRollbackErrorIgnored(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		assignStmt, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		// Look for: _ = tx.Rollback() or result, _ := tx.Rollback()
		for i, rhs := range assignStmt.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}

			if d.rollbackMethods[sel.Sel.Name] {
				// Check if the error is ignored
				// For single assignment: _ = tx.Rollback()
				if len(assignStmt.Lhs) == 1 {
					if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok && ident.Name == "_" {
						if varIdent, ok := sel.X.(*ast.Ident); ok {
							d.markAllStatesWithName(varIdent.Name, func(s *TxState) {
								s.RollbackErrorIgnored = true
							})
						}
					}
				}
				// For tuple assignment: result, _ := someFunc() - check if error position is _
				if len(assignStmt.Lhs) > i+1 {
					if ident, ok := assignStmt.Lhs[i+1].(*ast.Ident); ok && ident.Name == "_" {
						if varIdent, ok := sel.X.(*ast.Ident); ok {
							d.markAllStatesWithName(varIdent.Name, func(s *TxState) {
								s.RollbackErrorIgnored = true
							})
						}
					}
				}
			}
		}
		return true
	})
}

// checkDeferInLoop checks if defer with transaction is inside a loop (antipattern).
func (d *TxLeakDetector) checkDeferInLoop(funcDecl *ast.FuncDecl) {
	// We need to track loop depth manually since ast.Inspect doesn't give us exit notification
	var processNode func(n ast.Node, loopDepth int)
	processNode = func(n ast.Node, loopDepth int) {
		if n == nil {
			return
		}

		switch node := n.(type) {
		case *ast.ForStmt:
			// Enter for loop - process body with increased depth
			if node.Body != nil {
				for _, stmt := range node.Body.List {
					processNode(stmt, loopDepth+1)
				}
			}
		case *ast.RangeStmt:
			// Enter range loop - process body with increased depth
			if node.Body != nil {
				for _, stmt := range node.Body.List {
					processNode(stmt, loopDepth+1)
				}
			}
		case *ast.DeferStmt:
			if loopDepth > 0 {
				// Defer is inside a loop - check if it involves a tracked transaction
				call := node.Call
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if d.rollbackMethods[sel.Sel.Name] || d.commitMethods[sel.Sel.Name] {
						if ident, ok := sel.X.(*ast.Ident); ok {
							d.markAllStatesWithName(ident.Name, func(s *TxState) {
								s.HasDeferInLoop = true
							})
						}
					}
				}
				// Also check defer func() { tx.Rollback() }()
				if funcLit, ok := call.Fun.(*ast.FuncLit); ok {
					ast.Inspect(funcLit.Body, func(inner ast.Node) bool {
						if innerCall, ok := inner.(*ast.CallExpr); ok {
							if sel, ok := innerCall.Fun.(*ast.SelectorExpr); ok {
								if d.rollbackMethods[sel.Sel.Name] || d.commitMethods[sel.Sel.Name] {
									if ident, ok := sel.X.(*ast.Ident); ok {
										d.markAllStatesWithName(ident.Name, func(s *TxState) {
											s.HasDeferInLoop = true
										})
									}
								}
							}
						}
						return true
					})
				}
			}
		case *ast.BlockStmt:
			for _, stmt := range node.List {
				processNode(stmt, loopDepth)
			}
		case *ast.IfStmt:
			if node.Init != nil {
				processNode(node.Init, loopDepth)
			}
			processNode(node.Body, loopDepth)
			if node.Else != nil {
				processNode(node.Else, loopDepth)
			}
		case *ast.SwitchStmt:
			processNode(node.Body, loopDepth)
		case *ast.TypeSwitchStmt:
			processNode(node.Body, loopDepth)
		case *ast.SelectStmt:
			processNode(node.Body, loopDepth)
		case *ast.CaseClause:
			for _, stmt := range node.Body {
				processNode(stmt, loopDepth)
			}
		case *ast.CommClause:
			for _, stmt := range node.Body {
				processNode(stmt, loopDepth)
			}
		}
	}

	if funcDecl.Body != nil {
		for _, stmt := range funcDecl.Body.List {
			processNode(stmt, 0)
		}
	}
}

// checkConditionalCommits checks if Commit is inside a conditional block.
func (d *TxLeakDetector) checkConditionalCommits(funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}

		// Check if Commit is inside the if body
		hasCommitInIf := false
		ast.Inspect(ifStmt.Body, func(inner ast.Node) bool {
			if call, ok := inner.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if d.commitMethods[sel.Sel.Name] {
						hasCommitInIf = true
					}
				}
			}
			return true
		})

		// Check if Commit is NOT in else (meaning it might not execute)
		hasCommitInElse := false
		if ifStmt.Else != nil {
			ast.Inspect(ifStmt.Else, func(inner ast.Node) bool {
				if call, ok := inner.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if d.commitMethods[sel.Sel.Name] {
							hasCommitInElse = true
						}
					}
				}
				return true
			})
		}

		// If commit is only in one branch, it's conditional
		if hasCommitInIf != hasCommitInElse {
			for _, state := range d.txStates {
				if state.HasCommit && !state.HasDefer {
					state.CommitInConditional = true
				}
			}
		}

		return true
	})
}

// generateViolations creates violations for unclosed transactions.
func (d *TxLeakDetector) generateViolations() []TxLeakViolation {
	var violations []TxLeakViolation

	for _, state := range d.txStates {
		// Skip if transaction is returned to caller
		if state.IsReturned {
			continue
		}

		// Skip if transaction is returned in a closure (lifecycle managed by caller)
		if state.IsReturnedInClosure {
			continue
		}

		// Skip if transaction is sent through a channel (lifecycle managed by receiver)
		if state.IsSentToChannel {
			continue
		}

		// Skip if using callback pattern
		if state.IsCallback {
			continue
		}

		// Skip if passed to another function (can't track inter-procedural)
		// But warn if there's no defer as a safety net
		if state.IsPassedToFunc && state.HasDefer {
			continue
		}

		// Skip if stored in struct (can't track field lifecycle)
		// But warn if there's no defer as a safety net
		if state.IsStoredInStruct && state.HasDefer {
			continue
		}

		// Skip if stored in collection (map/slice) with defer
		// But warn if there's no defer as a safety net
		if state.IsStoredInCollection && state.HasDefer {
			continue
		}

		// Handle shadowing - this is always a problem
		if state.IsShadowed {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " is shadowed by another transaction in inner scope",
				Severity:      TxLeakSeverityHigh,
				ViolationType: "shadowed_transaction",
				TxVarName:     state.VarName,
				Suggestion:    "Use different variable names for nested transactions to avoid shadowing",
			})
			continue
		}

		// Handle goroutine capture - warn about potential issues
		if state.IsCapturedByGoroutine && !state.HasDefer {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " is captured by goroutine without defer",
				Severity:      TxLeakSeverityHigh,
				ViolationType: "goroutine_capture",
				TxVarName:     state.VarName,
				Suggestion:    "Ensure transaction is properly handled in goroutine with defer Rollback()",
			})
			continue
		}

		// Handle panic paths
		if state.HasPanicPath && !state.HasDefer {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " may leak if panic() is called",
				Severity:      TxLeakSeverityMedium,
				ViolationType: "panic_without_defer",
				TxVarName:     state.VarName,
				Suggestion:    "Add defer " + state.VarName + ".Rollback() to handle panic scenarios",
			})
		}

		// Handle conditional commits
		if state.CommitInConditional && !state.HasDefer {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " Commit() is inside conditional - may not execute",
				Severity:      TxLeakSeverityMedium,
				ViolationType: "conditional_commit",
				TxVarName:     state.VarName,
				Suggestion:    "Ensure Commit() is called on all success paths or use defer pattern",
			})
		}

		// Handle switch/case commits
		if state.CommitInSwitch && !state.HasDefer {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " Commit() is inside switch/case - may not execute in all cases",
				Severity:      TxLeakSeverityMedium,
				ViolationType: "commit_in_switch",
				TxVarName:     state.VarName,
				Suggestion:    "Ensure Commit() is called in all switch cases or use defer pattern",
			})
		}

		// Handle select/case commits
		if state.CommitInSelect && !state.HasDefer {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " Commit() is inside select/case - may not execute in all cases",
				Severity:      TxLeakSeverityMedium,
				ViolationType: "commit_in_select",
				TxVarName:     state.VarName,
				Suggestion:    "Ensure Commit() is called in all select cases or use defer pattern",
			})
		}

		// Handle fatal paths (os.Exit, log.Fatal)
		if state.HasFatalPath && !state.HasDefer {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " may leak if os.Exit() or log.Fatal() is called",
				Severity:      TxLeakSeverityHigh,
				ViolationType: "fatal_without_defer",
				TxVarName:     state.VarName,
				Suggestion:    "Add defer " + state.VarName + ".Rollback() to handle fatal exit scenarios (note: defers don't run on os.Exit)",
			})
		}

		// Handle commit in loop
		if state.CommitInLoop && !state.HasDefer {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " Commit() is inside loop - may not execute if loop doesn't iterate",
				Severity:      TxLeakSeverityMedium,
				ViolationType: "commit_in_loop",
				TxVarName:     state.VarName,
				Suggestion:    "Move Commit() outside loop or ensure loop always iterates at least once",
			})
		}

		// Handle variable reassignment
		if state.IsReassigned {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction variable " + state.VarName + " is reassigned - previous transaction may leak",
				Severity:      TxLeakSeverityHigh,
				ViolationType: "variable_reassignment",
				TxVarName:     state.VarName,
				Suggestion:    "Commit or Rollback the first transaction before starting a new one, or use different variable names",
			})
		}

		// Handle ignored commit error
		if state.CommitErrorIgnored {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " Commit() error is ignored with blank identifier",
				Severity:      TxLeakSeverityLow,
				ViolationType: "commit_error_ignored",
				TxVarName:     state.VarName,
				Suggestion:    "Handle the Commit() error - if it fails, the transaction is rolled back automatically",
			})
		}

		// Handle deferred commit - antipattern
		if state.HasDeferredCommit {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " uses defer Commit() - this is an antipattern",
				Severity:      TxLeakSeverityMedium,
				ViolationType: "deferred_commit",
				TxVarName:     state.VarName,
				Suggestion:    "Use defer " + state.VarName + ".Rollback() and explicit Commit() at the end of the function",
			})
		}

		// Handle ignored rollback error
		if state.RollbackErrorIgnored {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " Rollback() error is ignored with blank identifier",
				Severity:      TxLeakSeverityLow,
				ViolationType: "rollback_error_ignored",
				TxVarName:     state.VarName,
				Suggestion:    "Consider logging Rollback() errors for debugging - silent failures can hide issues",
			})
		}

		// Handle defer inside loop - antipattern
		if state.HasDeferInLoop {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " has defer inside loop - defers pile up until function returns",
				Severity:      TxLeakSeverityHigh,
				ViolationType: "defer_in_loop",
				TxVarName:     state.VarName,
				Suggestion:    "Move transaction handling outside loop or use explicit Rollback()/Commit() in each iteration",
			})
		}

		// Case 1: No Commit and no Rollback - Critical
		if !state.HasCommit && !state.HasRollback {
			msg := "unclosed transaction: " + state.VarName + " - missing both Commit() and Rollback()"
			if state.IsPassedToFunc {
				msg += " (passed to function - ensure it handles the transaction)"
			}
			if state.IsStoredInStruct {
				msg += " (stored in struct - ensure lifecycle is managed)"
			}

			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       msg,
				Severity:      TxLeakSeverityCritical,
				ViolationType: "no_commit_rollback",
				TxVarName:     state.VarName,
				Suggestion:    "Add defer " + state.VarName + ".Rollback() immediately after Begin(), then call Commit() on success",
			})
			continue
		}

		// Case 2: Has Commit but no Rollback - High
		if state.HasCommit && !state.HasRollback {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " has Commit() but no Rollback() for error paths",
				Severity:      TxLeakSeverityHigh,
				ViolationType: "no_rollback",
				TxVarName:     state.VarName,
				Suggestion:    "Add defer " + state.VarName + ".Rollback() to handle errors - it's safe to call after Commit()",
			})
			continue
		}

		// Case 3: Has Rollback but no Commit - Medium
		if state.HasRollback && !state.HasCommit {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " has Rollback() but missing Commit()",
				Severity:      TxLeakSeverityMedium,
				ViolationType: "no_commit",
				TxVarName:     state.VarName,
				Suggestion:    "Ensure Commit() is called on success path",
			})
		}

		// Case 4: Early return without defer
		if state.HasEarlyReturn && !state.HasDefer && state.HasCommit {
			violations = append(violations, TxLeakViolation{
				Pos:           state.BeginPos,
				End:           state.BeginEnd,
				Message:       "transaction " + state.VarName + " has early return paths that bypass Commit()",
				Severity:      TxLeakSeverityHigh,
				ViolationType: "early_return",
				TxVarName:     state.VarName,
				Suggestion:    "Add defer " + state.VarName + ".Rollback() to handle early returns safely",
			})
		}
	}

	return violations
}

// AnalyzeTxLeaks is a convenience function to run transaction leak detection on a file.
func AnalyzeTxLeaks(pass *analysis.Pass, file *ast.File) {
	detector := NewTxLeakDetector()
	violations := detector.CheckTxLeaks(pass, file)

	for _, v := range violations {
		message := "[" + string(v.Severity) + "] " + v.Message
		if v.Suggestion != "" {
			message += "\n  Suggestion: " + v.Suggestion
		}

		pass.Report(analysis.Diagnostic{
			Pos:     v.Pos,
			End:     v.End,
			Message: message,
		})
	}
}

// GetTxLeakViolations returns all transaction leak violations for external use.
func GetTxLeakViolations(pass *analysis.Pass, file *ast.File) []TxLeakViolation {
	detector := NewTxLeakDetector()
	return detector.CheckTxLeaks(pass, file)
}

// DetectTxLeaksInAST detects transaction leak problems in an AST file without analysis.Pass.
// This is designed for use in LSP server where we don't have a full analysis pass.
func DetectTxLeaksInAST(fset *token.FileSet, file *ast.File) []TxLeakViolation {
	detector := NewTxLeakDetector()
	var violations []TxLeakViolation

	// Analyze each function declaration
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			continue
		}

		// Skip test functions (TestXxx, BenchmarkXxx, FuzzXxx, ExampleXxx)
		if isTestFunction(funcDecl) {
			continue
		}

		// Skip test helper functions (functions with *testing.T as first param)
		if isTestHelperFunction(funcDecl) {
			continue
		}

		violations = append(violations, detector.analyzeFunction(funcDecl)...)
	}

	return violations
}
