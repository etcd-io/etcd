package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// N1Detector detects potential N+1 query problems.
// An N+1 query problem occurs when a SQL query is executed inside a loop,
// causing one query per iteration instead of a single batch query.
type N1Detector struct {
	// loopDepth tracks how deeply nested we are in loops
	loopDepth int
	// queryMethods are method names that typically execute SQL queries
	queryMethods map[string]bool
	// ormMethods are ORM-specific methods that indicate queries
	ormMethods map[string]bool
	// transactionMethods are methods that indicate transaction usage
	transactionMethods map[string]bool
	// functionCallsInLoop tracks function calls made inside loops for indirect N+1 detection
	functionCallsInLoop map[string]bool
	// knownQueryFunctions are user-defined functions known to execute queries
	knownQueryFunctions map[string]bool
}

// NewN1Detector creates a new N+1 query detector.
func NewN1Detector() *N1Detector {
	return &N1Detector{
		queryMethods: map[string]bool{
			// Standard database/sql
			"Query":           true,
			"QueryRow":        true,
			"Exec":            true,
			"ExecContext":     true,
			"QueryContext":    true,
			"QueryRowContext": true,
			// SQLx
			"QueryRowx":         true,
			"Queryx":            true,
			"SelectContext":     true,
			"GetContext":        true,
			"NamedQuery":        true,
			"NamedQueryContext": true,
			"NamedExec":         true,
			"NamedExecContext":  true,
			// General
			"Select": true,
			"Get":    true,
			"Find":   true,
			"First":  true,
			"Where":  true,
			"Raw":    true,
		},
		ormMethods: map[string]bool{
			// GORM
			"Preload":     true,
			"Association": true,
			"Related":     true,
			"Model":       true,
			"Table":       true,
			"Joins":       true,
			"InnerJoins":  true,
			"Pluck":       true,
			"Count":       true,
			"Take":        true,
			"Last":        true,
			"Scan":        true,
			"Row":         true,
			"Rows":        true,
			// Bun
			"NewSelect":  true,
			"NewInsert":  true,
			"NewUpdate":  true,
			"NewDelete":  true,
			"Column":     true,
			"ColumnExpr": true,
			"Relation":   true,
			// Ent
			"Only":   true,
			"OnlyX":  true,
			"AllX":   true,
			"FirstX": true,
			// SQLBoiler
			"One":     true,
			"OneP":    true,
			"AllP":    true,
			"Exists":  true,
			"ExistsP": true,
			// PGX
			"SendBatch": true,
		},
		transactionMethods: map[string]bool{
			"Begin":            true,
			"BeginTx":          true,
			"BeginTxx":         true,
			"Transaction":      true,
			"WithContext":      true,
			"RunInTransaction": true,
			"Tx":               true,
			"NewTx":            true,
		},
		functionCallsInLoop: make(map[string]bool),
		knownQueryFunctions: make(map[string]bool),
	}
}

// N1Severity represents the severity level of an N+1 violation.
type N1Severity string

const (
	N1SeverityCritical N1Severity = "critical" // Direct query in loop
	N1SeverityHigh     N1Severity = "high"     // ORM method in loop
	N1SeverityMedium   N1Severity = "medium"   // Indirect query via function call
	N1SeverityLow      N1Severity = "low"      // Potential issue, needs review
)

// N1Violation represents a detected N+1 query problem.
type N1Violation struct {
	Pos          token.Pos
	End          token.Pos
	Message      string
	LoopType     string     // "for", "range", or "while-like"
	QueryType    string     // The method name that was called
	Severity     N1Severity // Severity level
	Suggestion   string     // Suggested fix
	IsIndirect   bool       // True if detected via function call
	FunctionName string     // Name of the function if indirect
}

// CheckN1Queries analyzes the file for N+1 query patterns.
func (d *N1Detector) CheckN1Queries(pass *analysis.Pass, file *ast.File) []N1Violation {
	var violations []N1Violation

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			// Entering a for loop
			d.loopDepth++
			defer func() { d.loopDepth-- }()

			// Check the body for queries
			if node.Body != nil {
				violations = append(violations, d.checkBlockForQueries(pass, node.Body, "for")...)
			}
			return true

		case *ast.RangeStmt:
			// Entering a range loop
			d.loopDepth++
			defer func() { d.loopDepth-- }()

			// Check the body for queries
			if node.Body != nil {
				violations = append(violations, d.checkBlockForQueries(pass, node.Body, "range")...)
			}
			return true
		}
		return true
	})

	return violations
}

// checkBlockForQueries checks a block statement for SQL query calls.
func (d *N1Detector) checkBlockForQueries(pass *analysis.Pass, block *ast.BlockStmt, loopType string) []N1Violation {
	var violations []N1Violation

	ast.Inspect(block, func(n ast.Node) bool {
		// Skip nested loops - they'll be handled separately
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		methodName := d.getMethodName(call)
		if methodName == "" {
			return true
		}

		// Check 1: Direct query method call
		if d.queryMethods[methodName] {
			if v := d.checkDirectQueryCall(call, methodName, loopType); v != nil {
				violations = append(violations, *v)
			}
			return true
		}

		// Check 2: ORM-specific method call
		if d.ormMethods[methodName] {
			if v := d.checkORMMethodCall(call, methodName, loopType); v != nil {
				violations = append(violations, *v)
			}
			return true
		}

		// Check 3: Transaction method in loop (potential issue)
		if d.transactionMethods[methodName] {
			violations = append(violations, N1Violation{
				Pos:        call.Pos(),
				End:        call.End(),
				Message:    "transaction started inside loop - consider batching operations",
				LoopType:   loopType,
				QueryType:  methodName,
				Severity:   N1SeverityMedium,
				Suggestion: "Move transaction outside the loop and batch operations, or use a single transaction for all iterations",
			})
			return true
		}

		// Check 4: Indirect query via function call
		if v := d.checkIndirectQueryCall(call, methodName, loopType); v != nil {
			violations = append(violations, *v)
		}

		return true
	})

	return violations
}

// checkDirectQueryCall checks for direct database query calls.
func (d *N1Detector) checkDirectQueryCall(call *ast.CallExpr, methodName, loopType string) *N1Violation {
	// Check if any argument contains SELECT or other SQL keywords
	hasSQLKeyword := false
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok {
			if lit.Kind == token.STRING {
				value := strings.ToUpper(lit.Value)
				if strings.Contains(value, "SELECT") || strings.Contains(value, "INSERT") ||
					strings.Contains(value, "UPDATE") || strings.Contains(value, "DELETE") {
					hasSQLKeyword = true
					break
				}
			}
		}
	}

	// Also check if it's a method on a database variable
	if !hasSQLKeyword {
		hasSQLKeyword = d.mightBeQueryCall(call)
	}

	if hasSQLKeyword {
		return &N1Violation{
			Pos:        call.Pos(),
			End:        call.End(),
			Message:    n1Message(methodName, loopType),
			LoopType:   loopType,
			QueryType:  methodName,
			Severity:   N1SeverityCritical,
			Suggestion: n1Suggestion(loopType),
		}
	}

	return nil
}

// checkORMMethodCall checks for ORM-specific method calls that might cause N+1.
func (d *N1Detector) checkORMMethodCall(call *ast.CallExpr, methodName, loopType string) *N1Violation {
	// ORM methods like Preload, Association, Related are often N+1 sources
	suspiciousMethods := map[string]bool{
		"Preload":     true,
		"Association": true,
		"Related":     true,
		"Relation":    true,
	}

	if suspiciousMethods[methodName] {
		return &N1Violation{
			Pos:        call.Pos(),
			End:        call.End(),
			Message:    "ORM relation loading inside loop: " + methodName + "() - this causes N+1 queries",
			LoopType:   loopType,
			QueryType:  methodName,
			Severity:   N1SeverityHigh,
			Suggestion: "Use eager loading (Preload) before the loop, or fetch all related data in a single query with JOIN",
		}
	}

	// Check for query execution methods
	executionMethods := map[string]bool{
		"Find": true, "First": true, "Take": true, "Last": true,
		"One": true, "All": true, "Only": true, "Scan": true,
	}

	if executionMethods[methodName] && d.mightBeQueryCall(call) {
		return &N1Violation{
			Pos:        call.Pos(),
			End:        call.End(),
			Message:    "ORM query execution inside loop: " + methodName + "() - potential N+1 problem",
			LoopType:   loopType,
			QueryType:  methodName,
			Severity:   N1SeverityHigh,
			Suggestion: "Collect IDs first, then use WHERE IN clause or batch loading",
		}
	}

	return nil
}

// checkIndirectQueryCall checks for function calls that might execute queries.
func (d *N1Detector) checkIndirectQueryCall(call *ast.CallExpr, methodName, loopType string) *N1Violation {
	// Check for suspicious function names that might contain queries
	lowerName := strings.ToLower(methodName)
	suspiciousPatterns := []string{
		"get", "fetch", "find", "load", "query", "select",
		"retrieve", "lookup", "read", "search",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerName, pattern) {
			// Check if it's called on a repository/store/dao object
			if d.isRepositoryCall(call) {
				return &N1Violation{
					Pos:          call.Pos(),
					End:          call.End(),
					Message:      "potential N+1: function " + methodName + "() called inside loop may execute database query",
					LoopType:     loopType,
					QueryType:    methodName,
					Severity:     N1SeverityMedium,
					Suggestion:   "Review this function - if it executes a query, consider batch loading or caching",
					IsIndirect:   true,
					FunctionName: methodName,
				}
			}
		}
	}

	return nil
}

// isRepositoryCall checks if the call is on a repository-like object.
func (d *N1Detector) isRepositoryCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	if ident, ok := sel.X.(*ast.Ident); ok {
		name := strings.ToLower(ident.Name)
		repositoryPatterns := []string{
			"repo", "repository", "store", "dao", "service",
			"handler", "manager", "provider", "client",
		}
		for _, pattern := range repositoryPatterns {
			if strings.Contains(name, pattern) {
				return true
			}
		}
	}

	return false
}

// n1Suggestion generates a suggestion based on loop type.
func n1Suggestion(loopType string) string {
	switch loopType {
	case "range":
		return "Collect all IDs before the loop, then use a single query with IN clause:\n" +
			"  ids := make([]int, 0, len(items))\n" +
			"  for _, item := range items { ids = append(ids, item.ID) }\n" +
			"  db.Query(\"SELECT * FROM table WHERE id IN (?)\", ids)"
	case "for":
		return "Consider using batch query with IN clause or JOIN instead of querying in each iteration"
	default:
		return "Refactor to execute a single batch query instead of multiple queries in loop"
	}
}

// getMethodName extracts the method name from a call expression.
func (d *N1Detector) getMethodName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

// mightBeQueryCall checks if a call might be a database query.
func (d *N1Detector) mightBeQueryCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check for common database variable names
	if ident, ok := sel.X.(*ast.Ident); ok {
		name := strings.ToLower(ident.Name)
		return name == "db" || name == "conn" || name == "tx" ||
			strings.Contains(name, "database") ||
			strings.Contains(name, "repo") ||
			strings.Contains(name, "store")
	}

	return false
}

// n1Message generates a helpful message for N+1 detection.
func n1Message(methodName, loopType string) string {
	suggestion := ""
	switch loopType {
	case "range":
		suggestion = "Consider using a batch query with IN clause or JOIN"
	case "for":
		suggestion = "Consider collecting IDs first, then executing a single query with IN clause"
	}

	return "potential N+1 query: " + methodName + "() called inside loop - " + suggestion
}

// AnalyzeN1 is a convenience function to run N+1 detection on a file.
func AnalyzeN1(pass *analysis.Pass, file *ast.File) {
	detector := NewN1Detector()
	violations := detector.CheckN1Queries(pass, file)

	for _, v := range violations {
		message := v.Message
		if v.Suggestion != "" {
			message += "\n  Suggestion: " + v.Suggestion
		}
		if v.Severity != "" {
			message = "[" + string(v.Severity) + "] " + message
		}

		pass.Report(analysis.Diagnostic{
			Pos:     v.Pos,
			End:     v.End,
			Message: message,
		})
	}
}

// GetN1Violations returns all N+1 violations for external use.
func GetN1Violations(pass *analysis.Pass, file *ast.File) []N1Violation {
	detector := NewN1Detector()
	return detector.CheckN1Queries(pass, file)
}

// CheckN1QueriesNoPass is a method to check for N+1 queries without analysis.Pass.
// This is useful for testing.
func (d *N1Detector) CheckN1QueriesNoPass(file *ast.File) []N1Violation {
	return DetectN1InAST(nil, file)
}

// DetectN1InAST detects N+1 query problems in an AST file without analysis.Pass.
// This is designed for use in LSP server where we don't have a full analysis pass.
func DetectN1InAST(fset *token.FileSet, file *ast.File) []N1Violation {
	detector := NewN1Detector()
	var violations []N1Violation

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			detector.loopDepth++
			defer func() { detector.loopDepth-- }()

			if node.Body != nil {
				violations = append(violations, detector.checkBlockForQueriesNoPass(node.Body, "for")...)
			}
			return true

		case *ast.RangeStmt:
			detector.loopDepth++
			defer func() { detector.loopDepth-- }()

			if node.Body != nil {
				violations = append(violations, detector.checkBlockForQueriesNoPass(node.Body, "range")...)
			}
			return true
		}
		return true
	})

	return violations
}

// checkBlockForQueriesNoPass checks a block statement for SQL query calls without analysis.Pass.
func (d *N1Detector) checkBlockForQueriesNoPass(block *ast.BlockStmt, loopType string) []N1Violation {
	var violations []N1Violation

	ast.Inspect(block, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		methodName := d.getMethodName(call)
		if methodName == "" {
			return true
		}

		// Check 1: Direct query method call
		if d.queryMethods[methodName] {
			if v := d.checkDirectQueryCall(call, methodName, loopType); v != nil {
				violations = append(violations, *v)
			}
			return true
		}

		// Check 2: ORM-specific method call
		if d.ormMethods[methodName] {
			if v := d.checkORMMethodCall(call, methodName, loopType); v != nil {
				violations = append(violations, *v)
			}
			return true
		}

		// Check 3: Transaction method in loop
		if d.transactionMethods[methodName] {
			violations = append(violations, N1Violation{
				Pos:        call.Pos(),
				End:        call.End(),
				Message:    "transaction started inside loop - consider batching operations",
				LoopType:   loopType,
				QueryType:  methodName,
				Severity:   N1SeverityMedium,
				Suggestion: "Move transaction outside the loop and batch operations, or use a single transaction for all iterations",
			})
			return true
		}

		// Check 4: Indirect query via function call
		if v := d.checkIndirectQueryCall(call, methodName, loopType); v != nil {
			violations = append(violations, *v)
		}

		return true
	})

	return violations
}
