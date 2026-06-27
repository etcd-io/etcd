package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// SQLISeverity represents the severity level of SQL injection vulnerability.
type SQLISeverity string

const (
	SQLISeverityCritical SQLISeverity = "critical" // Direct user input in query
	SQLISeverityHigh     SQLISeverity = "high"     // Format string with variables
	SQLISeverityMedium   SQLISeverity = "medium"   // String concatenation
	SQLISeverityLow      SQLISeverity = "low"      // Potential issue, needs review
)

// SQLInjectionScanner detects potential SQL injection vulnerabilities.
type SQLInjectionScanner struct {
	// dangerousFuncs are functions that format strings (potential SQL injection vectors)
	dangerousFuncs map[string]map[string]bool // package -> function -> bool
	// queryFuncs are database query functions
	queryFuncs map[string]bool
	// ormQueryMethods are ORM-specific query methods
	ormQueryMethods map[string]bool
	// taintedVariables tracks variables that might contain user input
	taintedVariables map[string]bool
	// userInputPatterns are variable name patterns that suggest user input
	userInputPatterns []string
	// httpInputFuncs are functions that read HTTP input
	httpInputFuncs map[string]map[string]bool
	// pass is the current analysis pass
	pass *analysis.Pass
}

// SQLInjectionViolation represents a detected SQL injection vulnerability.
type SQLInjectionViolation struct {
	Pos        token.Pos
	End        token.Pos
	Message    string
	Severity   SQLISeverity
	VulnType   string // "concat", "sprintf", "exec", "tainted", "orm_raw"
	Suggestion string
	CodeFix    string // Suggested code fix
}

// NewSQLInjectionScanner creates a new SQL injection scanner.
func NewSQLInjectionScanner() *SQLInjectionScanner {
	return &SQLInjectionScanner{
		dangerousFuncs: map[string]map[string]bool{
			"fmt": {
				"Sprintf": true,
				"Fprintf": true,
				"Printf":  true,
				"Errorf":  true,
				"Sscanf":  true,
			},
			"strings": {
				"Join":       true,
				"Replace":    true,
				"ReplaceAll": true,
				"Builder":    true,
			},
			"strconv": {
				"Itoa":      true,
				"FormatInt": true,
			},
		},
		queryFuncs: map[string]bool{
			// Standard database/sql
			"Query":           true,
			"QueryRow":        true,
			"Exec":            true,
			"ExecContext":     true,
			"QueryContext":    true,
			"QueryRowContext": true,
			"Prepare":         true,
			"PrepareContext":  true,
			// SQLx
			"QueryRowx":         true,
			"Queryx":            true,
			"MustExec":          true,
			"NamedExec":         true,
			"NamedQuery":        true,
			"NamedExecContext":  true,
			"NamedQueryContext": true,
			"Get":               true,
			"Select":            true,
			"GetContext":        true,
			"SelectContext":     true,
			// GORM
			"Raw":    true,
			"Where":  true,
			"Having": true,
			"Order":  true,
			"Group":  true,
			"Joins":  true,
			// Bun
			"NewRaw":     true,
			"ColumnExpr": true,
			"WhereOr":    true,
			// PGX
			"SendBatch": true,
		},
		ormQueryMethods: map[string]bool{
			// GORM dangerous methods when used with string concat
			"Raw":    true,
			"Exec":   true,
			"Where":  true,
			"Or":     true,
			"Not":    true,
			"Having": true,
			"Order":  true,
			"Group":  true,
			"Joins":  true,
			// Bun
			"NewRaw":     true,
			"WhereOr":    true,
			"ColumnExpr": true,
			"TableExpr":  true,
		},
		taintedVariables: make(map[string]bool),
		userInputPatterns: []string{
			"user", "input", "param", "query", "search", "filter",
			"id", "name", "email", "password", "username", "request",
			"body", "form", "data", "value", "arg", "args",
			"term", "keyword", "text", "content",
		},
		httpInputFuncs: map[string]map[string]bool{
			"http": {
				"Request": true,
			},
			"gin": {
				"Param":        true,
				"Query":        true,
				"PostForm":     true,
				"DefaultQuery": true,
				"GetQuery":     true,
				"BindJSON":     true,
				"ShouldBind":   true,
			},
			"echo": {
				"Param":      true,
				"QueryParam": true,
				"FormValue":  true,
				"Bind":       true,
			},
			"fiber": {
				"Params":     true,
				"Query":      true,
				"FormValue":  true,
				"BodyParser": true,
			},
			"chi": {
				"URLParam": true,
			},
			"mux": {
				"Vars": true,
			},
		},
	}
}

// SQL pattern for identifying SQL-like strings
var sqlPattern = regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE|DROP|CREATE|ALTER|TRUNCATE)\s+`)

// Placeholder patterns for parameterized queries
var placeholderPattern = regexp.MustCompile(`(\?|\$\d+|:\w+|@\w+)`)

// ScanFile scans a file for SQL injection vulnerabilities.
func (s *SQLInjectionScanner) ScanFile(pass *analysis.Pass, file *ast.File) []SQLInjectionViolation {
	s.pass = pass
	var violations []SQLInjectionViolation

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// Check for dangerous patterns
			if v := s.checkCallExpr(node); v != nil {
				violations = append(violations, *v)
			}
		case *ast.BinaryExpr:
			// Check for string concatenation in SQL context
			if v := s.checkBinaryExpr(node); v != nil {
				violations = append(violations, *v)
			}
		}
		return true
	})

	return violations
}

// checkCallExpr checks a function call for SQL injection patterns.
func (s *SQLInjectionScanner) checkCallExpr(call *ast.CallExpr) *SQLInjectionViolation {
	// Check if this is a query function call
	methodName := s.getMethodName(call)
	if !s.queryFuncs[methodName] && !s.ormQueryMethods[methodName] {
		return nil
	}

	// Ignore *sql.Stmt calls since they don't take queries
	if s.isStmtMethod(call) {
		return nil
	}

	// If this is a parameterized query (first arg is string literal with placeholders,
	// subsequent args are parameters), it's safe
	if s.isParameterizedQuery(call) {
		return nil
	}

	// Determine which argument is the query string
	queryIdx := 0
	if strings.HasSuffix(methodName, "Context") {
		queryIdx = 1
	}

	if len(call.Args) <= queryIdx {
		return nil
	}

	// Only check the query argument for dangerous patterns
	arg := call.Args[queryIdx]

	// Pattern 1: fmt.Sprintf result used as query
	if innerCall, ok := arg.(*ast.CallExpr); ok {
		if s.isDangerousFormatCall(innerCall) {
			if s.containsUserInput(innerCall) {
				return &SQLInjectionViolation{
					Pos:        call.Pos(),
					End:        call.End(),
					Message:    "SQL INJECTION: fmt.Sprintf with user input passed to " + methodName + "()",
					Severity:   SQLISeverityCritical,
					VulnType:   "sprintf",
					Suggestion: "Use parameterized queries with placeholders (?, $1, :name)",
					CodeFix:    s.generateParameterizedFix(methodName, arg),
				}
			}
			// Even without detected user input, format strings are suspicious
			return &SQLInjectionViolation{
				Pos:        call.Pos(),
				End:        call.End(),
				Message:    "potential SQL injection: fmt.Sprintf result passed to " + methodName + "() - use parameterized queries",
				Severity:   SQLISeverityHigh,
				VulnType:   "sprintf",
				Suggestion: "Replace fmt.Sprintf with parameterized query using placeholders",
				CodeFix:    s.generateParameterizedFix(methodName, arg),
			}
		}
		// Check for HTTP input functions
		if s.isHTTPInputCall(innerCall) {
			return &SQLInjectionViolation{
				Pos:        call.Pos(),
				End:        call.End(),
				Message:    "SQL INJECTION: HTTP input directly used in " + methodName + "()",
				Severity:   SQLISeverityCritical,
				VulnType:   "tainted",
				Suggestion: "Never use HTTP input directly in SQL - always use parameterized queries",
			}
		}
	}

	// Pattern 2: String concatenation used as query
	if binExpr, ok := arg.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.ADD {
			if s.containsTaintedVariable(binExpr) {
				return &SQLInjectionViolation{
					Pos:        call.Pos(),
					End:        call.End(),
					Message:    "SQL INJECTION: string concatenation with user input in " + methodName + "()",
					Severity:   SQLISeverityCritical,
					VulnType:   "concat",
					Suggestion: "Use parameterized queries instead of string concatenation",
					CodeFix:    "Replace: db." + methodName + "(\"SELECT * FROM users WHERE id = \" + id)\nWith: db." + methodName + "(\"SELECT * FROM users WHERE id = ?\", id)",
				}
			}
			if s.containsStringVariable(binExpr) {
				return &SQLInjectionViolation{
					Pos:        call.Pos(),
					End:        call.End(),
					Message:    "potential SQL injection: string concatenation in " + methodName + "()",
					Severity:   SQLISeverityHigh,
					VulnType:   "concat",
					Suggestion: "Use parameterized queries instead of string concatenation",
				}
			}
		}
	}

	// Pattern 3: Tainted variable used directly
	if ident := getIdent(arg); ident != nil {
		if s.isTaintedVariable(ident.Name) && !s.isConstant(arg) {
			return &SQLInjectionViolation{
				Pos:        call.Pos(),
				End:        call.End(),
				Message:    fmt.Sprintf("SQL INJECTION: potentially tainted variable '%s' used in %s ()", ident.Name, methodName),
				Severity:   SQLISeverityHigh,
				VulnType:   "tainted",
				Suggestion: fmt.Sprintf("Validate and sanitize '%s' or use parameterized queries", ident.Name),
			}
		}
		if s.mightBeDynamicQuery(ident) && !s.isConstant(arg) {
			return &SQLInjectionViolation{
				Pos:        call.Pos(),
				End:        call.End(),
				Message:    "review SQL query in " + methodName + "(): ensure '" + ident.Name + "' is not built with user input",
				Severity:   SQLISeverityMedium,
				VulnType:   "variable",
				Suggestion: "Audit the construction of '" + ident.Name + "' to ensure it doesn't contain user input",
			}
		}
	}

	// Pattern 4: ORM Raw methods with string variables
	if s.ormQueryMethods[methodName] {
		if v := s.checkORMRawMethod(call, methodName); v != nil {
			return v
		}
	}

	return nil
}

// checkORMRawMethod checks ORM-specific raw SQL methods.
func (s *SQLInjectionScanner) checkORMRawMethod(call *ast.CallExpr, methodName string) *SQLInjectionViolation {
	// Methods like db.Raw(), db.Where() with string concatenation
	if len(call.Args) == 0 {
		return nil
	}

	firstArg := call.Args[0]

	// Check if first argument is a string literal (safe) or variable (needs review)
	if _, ok := firstArg.(*ast.BasicLit); !ok {
		// Not a string literal - might be dangerous
		if ident := getIdent(firstArg); ident != nil {
			if s.isTaintedVariable(ident.Name) && !s.isConstant(firstArg) {
				return &SQLInjectionViolation{
					Pos:        call.Pos(),
					End:        call.End(),
					Message:    "SQL INJECTION risk: " + methodName + "() with potentially tainted variable",
					Severity:   SQLISeverityHigh,
					VulnType:   "orm_raw",
					Suggestion: "Use parameterized syntax: db." + methodName + "(\"field = ?\", value)",
				}
			}
		}
	}

	return nil
}

// generateParameterizedFix generates a suggested fix for sprintf patterns.
func (s *SQLInjectionScanner) generateParameterizedFix(methodName string, arg ast.Expr) string {
	return "Replace fmt.Sprintf with parameterized query:\n" +
		"  Before: db." + methodName + "(fmt.Sprintf(\"SELECT * FROM users WHERE id = %d\", id))\n" +
		"  After:  db." + methodName + "(\"SELECT * FROM users WHERE id = ?\", id)"
}

// isHTTPInputCall checks if a call is reading HTTP input.
func (s *SQLInjectionScanner) isHTTPInputCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	methodName := sel.Sel.Name

	// Check for common HTTP input methods
	httpMethods := map[string]bool{
		"Param": true, "Query": true, "PostForm": true,
		"FormValue": true, "QueryParam": true, "Params": true,
		"GetQuery": true, "DefaultQuery": true, "BodyParser": true,
		"Bind": true, "BindJSON": true, "ShouldBind": true,
		"URLParam": true, "Vars": true,
	}

	return httpMethods[methodName]
}

// isConstant checks if an expression refers to a constant.
func (s *SQLInjectionScanner) isConstant(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if s.pass != nil && s.pass.TypesInfo != nil {
		if ident := getIdent(expr); ident != nil {
			if obj := s.pass.TypesInfo.ObjectOf(ident); obj != nil {
				_, ok := obj.(*types.Const)
				return ok
			}
		}
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Obj != nil && ident.Obj.Kind == ast.Con {
			return true
		}
	}
	return false
}

// getIdent extracts an identifier from an expression (Ident or SelectorExpr).
func getIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	default:
		return nil
	}
}

// getConstantValue attempts to get the string value of a constant expression.
func (s *SQLInjectionScanner) getConstantValue(expr ast.Expr) (string, bool) {
	if expr == nil {
		return "", false
	}
	if s.pass != nil && s.pass.TypesInfo != nil {
		if ident := getIdent(expr); ident != nil {
			if obj := s.pass.TypesInfo.ObjectOf(ident); obj != nil {
				if c, ok := obj.(*types.Const); ok {
					val := c.Val().ExactString()
					// Remove quotes if it's a string constant
					if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
						return val[1 : len(val)-1], true
					}
					return val, true
				}
			}
		}
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Obj != nil && ident.Obj.Kind == ast.Con {
			if vs, ok := ident.Obj.Decl.(*ast.ValueSpec); ok {
				for i, name := range vs.Names {
					if name.Name == ident.Name && i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							val := lit.Value
							if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
								return val[1 : len(val)-1], true
							}
							return val, true
						}
					}
				}
			}
		}
	}
	return "", false
}

// isTaintedVariable checks if a variable name suggests user input.
func (s *SQLInjectionScanner) isTaintedVariable(name string) bool {
	lowerName := strings.ToLower(name)
	for _, pattern := range s.userInputPatterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}
	return s.taintedVariables[name]
}

// containsTaintedVariable checks if an expression contains tainted variables.
func (s *SQLInjectionScanner) containsTaintedVariable(expr ast.Expr) bool {
	hasTainted := false

	ast.Inspect(expr, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if s.isTaintedVariable(ident.Name) && !s.isConstant(ident) {
				hasTainted = true
				return false
			}
		}
		return true
	})

	return hasTainted
}

// MarkVariableAsTainted marks a variable as containing user input.
func (s *SQLInjectionScanner) MarkVariableAsTainted(name string) {
	s.taintedVariables[name] = true
}

// isStmtMethod checks if a call is a method on a prepared statement (e.g., *sql.Stmt).
func (s *SQLInjectionScanner) isStmtMethod(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if s.pass != nil && s.pass.TypesInfo != nil {
			if selObj := s.pass.TypesInfo.Uses[sel.Sel]; selObj != nil {
				if sig, ok := selObj.Type().(*types.Signature); ok {
					if recv := sig.Recv(); recv != nil {
						recvType := recv.Type().String()
						if strings.Contains(recvType, "sql.Stmt") || strings.Contains(recvType, "sqlx.Stmt") || strings.Contains(recvType, "NamedStmt") {
							return true
						}
					}
				}
			}
		} else {
			// Fallback heuristic: if the receiver is named "stmt", assume it's a statement
			if ident, ok := sel.X.(*ast.Ident); ok {
				name := strings.ToLower(ident.Name)
				if strings.Contains(name, "stmt") {
					return true
				}
			}
		}
	}
	return false
}

// isParameterizedQuery checks if a call uses parameterized query syntax.
// A parameterized query has a string literal or constant with placeholders (?, $1, :name, @param)
// as the query argument, with subsequent arguments providing the values.
func (s *SQLInjectionScanner) isParameterizedQuery(call *ast.CallExpr) bool {
	methodName := s.getMethodName(call)
	queryIdx := 0
	// Context-aware methods usually have the query as the second argument
	if strings.HasSuffix(methodName, "Context") {
		queryIdx = 1
	}

	if len(call.Args) <= queryIdx+1 {
		return false
	}

	// Query argument should be a string literal or constant
	queryArg := call.Args[queryIdx]
	var queryStr string
	if lit, ok := queryArg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		queryStr = lit.Value
	} else if val, ok := s.getConstantValue(queryArg); ok {
		queryStr = val
	}

	if queryStr == "" {
		return false
	}

	// Check if the string contains placeholder patterns
	return placeholderPattern.MatchString(queryStr)
}

// checkBinaryExpr checks string concatenation for SQL injection.
func (s *SQLInjectionScanner) checkBinaryExpr(expr *ast.BinaryExpr) *SQLInjectionViolation {
	if expr.Op != token.ADD {
		return nil
	}

	// Check if this looks like SQL concatenation
	if s.isSQLStringConcat(expr) && s.containsStringVariable(expr) {
		return &SQLInjectionViolation{
			Pos:      expr.Pos(),
			End:      expr.End(),
			Message:  "potential SQL injection: string concatenation with SQL keywords - use parameterized queries",
			Severity: SQLISeverityMedium,
			VulnType: "concat",
		}
	}

	return nil
}

// getMethodName extracts the method name from a call expression.
func (s *SQLInjectionScanner) getMethodName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

// isDangerousFormatCall checks if a call is to a dangerous format function.
func (s *SQLInjectionScanner) isDangerousFormatCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	if funcs, ok := s.dangerousFuncs[pkgIdent.Name]; ok {
		return funcs[sel.Sel.Name]
	}

	return false
}

// containsUserInput checks if a format call might contain user input.
func (s *SQLInjectionScanner) containsUserInput(call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false
	}

	// Check if any argument is a variable (not a literal)
	for i, arg := range call.Args {
		if i == 0 {
			// Skip format string
			continue
		}
		// If argument is not a literal, it might be user input
		if _, ok := arg.(*ast.BasicLit); !ok {
			// Check if it's a constant (including exported constants from other packages)
			if s.isConstant(arg) {
				continue
			}
			return true
		}
	}

	return false
}

// containsStringVariable checks if an expression contains string variables.
func (s *SQLInjectionScanner) containsStringVariable(expr ast.Expr) bool {
	hasVariable := false

	ast.Inspect(expr, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Ident:
			// Check if this is a variable (not a constant)
			if s.isConstant(node) {
				return true
			}
			if node.Obj != nil {
				hasVariable = true
				return false
			}
		case *ast.CallExpr:
			// Function call results are considered dynamic
			hasVariable = true
			return false
		}
		return true
	})

	return hasVariable
}

// isSQLStringConcat checks if a binary expression looks like SQL concatenation.
func (s *SQLInjectionScanner) isSQLStringConcat(expr *ast.BinaryExpr) bool {
	// Check left operand
	if s.isQueryString(expr.X) {
		return true
	}
	// Check right operand
	if s.isQueryString(expr.Y) {
		return true
	}
	return false
}

// isQueryString checks if an expression looks like a SQL query string.
func (s *SQLInjectionScanner) isQueryString(expr ast.Expr) bool {
	var val string
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		val = lit.Value
	} else if v, ok := s.getConstantValue(expr); ok {
		val = v
	}

	if val == "" {
		return false
	}

	value := strings.ToUpper(val)
	return sqlPattern.MatchString(value)
}

// mightBeDynamicQuery checks if an identifier might be a dynamically built query.
func (s *SQLInjectionScanner) mightBeDynamicQuery(ident *ast.Ident) bool {
	name := strings.ToLower(ident.Name)
	return strings.Contains(name, "query") ||
		strings.Contains(name, "sql") ||
		strings.Contains(name, "stmt")
}

// AnalyzeSQLInjection is a convenience function to run SQL injection scanning.
func AnalyzeSQLInjection(pass *analysis.Pass, file *ast.File) {
	scanner := NewSQLInjectionScanner()
	violations := scanner.ScanFile(pass, file)

	for _, v := range violations {
		message := v.Message
		if v.Suggestion != "" {
			message += "\n  Suggestion: " + v.Suggestion
		}
		if v.CodeFix != "" {
			message += "\n  Fix: " + v.CodeFix
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

// GetSQLInjectionViolations returns all SQL injection violations for external use.
func GetSQLInjectionViolations(pass *analysis.Pass, file *ast.File) []SQLInjectionViolation {
	scanner := NewSQLInjectionScanner()
	return scanner.ScanFile(pass, file)
}

// ScanFileAST scans a file for SQL injection vulnerabilities without analysis.Pass.
// This is designed for use in LSP server where we don't have a full analysis pass.
func ScanFileAST(fset *token.FileSet, file *ast.File) []SQLInjectionViolation {
	scanner := NewSQLInjectionScanner()
	return scanner.ScanFileNoPass(fset, file)
}

// ScanFileNoPass scans a file for SQL injection vulnerabilities without analysis.Pass.
// This is a method version for testing purposes.
func (s *SQLInjectionScanner) ScanFileNoPass(fset *token.FileSet, file *ast.File) []SQLInjectionViolation {
	s.pass = nil
	var violations []SQLInjectionViolation

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if v := s.checkCallExpr(node); v != nil {
				violations = append(violations, *v)
			}
		case *ast.BinaryExpr:
			if v := s.checkBinaryExpr(node); v != nil {
				violations = append(violations, *v)
			}
		}
		return true
	})

	return violations
}
