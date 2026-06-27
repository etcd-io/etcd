// Package analyzer provides the SQL static analysis implementation for detecting SELECT * usage.
package analyzer

import (
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/MirrexOne/unqueryvet/internal/analyzer/sqlbuilders"
	"github.com/MirrexOne/unqueryvet/pkg/config"
)

const (
	// defaultWarningMessage is the standard warning for SELECT * usage
	defaultWarningMessage = "avoid SELECT * - explicitly specify needed columns for better performance, maintainability and stability"
)

// Precompiled regex patterns for performance
var (
	// aliasedWildcardPattern matches SELECT alias.* patterns like "SELECT t.*", "SELECT u.*, o.*"
	aliasedWildcardPattern = regexp.MustCompile(`(?i)SELECT\s+(?:[A-Za-z_][A-Za-z0-9_]*\s*\.\s*\*\s*,?\s*)+`)

	// subquerySelectStarPattern matches SELECT * in subqueries like "(SELECT * FROM ...)"
	subquerySelectStarPattern = regexp.MustCompile(`(?i)\(\s*SELECT\s+\*`)
)

// NewAnalyzer creates the Unqueryvet analyzer with enhanced logic for production use
func NewAnalyzer() *analysis.Analyzer {
	return NewAnalyzerWithSettings(config.DefaultSettings())
}

// NewAnalyzerWithSettings creates analyzer with provided settings for golangci-lint integration
func NewAnalyzerWithSettings(s config.UnqueryvetSettings) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "unqueryvet",
		Doc:  "detects SELECT * in SQL queries and SQL builders, preventing performance issues and encouraging explicit column selection",
		Run: func(pass *analysis.Pass) (any, error) {
			return RunWithConfig(pass, &s)
		},
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

// analysisContext holds the context for AST analysis
type analysisContext struct {
	pass            *analysis.Pass
	cfg             *config.UnqueryvetSettings
	filter          *FilterContext
	builderRegistry *sqlbuilders.Registry
}

// RunWithConfig performs analysis with provided configuration
// This is the main entry point for configured analysis
func RunWithConfig(pass *analysis.Pass, cfg *config.UnqueryvetSettings) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Use provided configuration or default if nil
	if cfg == nil {
		defaultSettings := config.DefaultSettings()
		cfg = &defaultSettings
	}

	// Create filter context for efficient filtering
	filter, err := NewFilterContext(cfg)
	if err != nil {
		// If filter creation fails, continue without filtering
		filter = nil
	}

	// Check if current file should be ignored
	if filter != nil && len(pass.Files) > 0 {
		fileName := pass.Fset.File(pass.Files[0].Pos()).Name()
		if filter.IsIgnoredFile(fileName) {
			return nil, nil
		}
	}

	// Create SQL builder registry for checking SQL builder patterns
	var builderRegistry *sqlbuilders.Registry
	if cfg.CheckSQLBuilders {
		builderRegistry = sqlbuilders.NewRegistry(&cfg.SQLBuilders)
	}

	ctx := &analysisContext{
		pass:            pass,
		cfg:             cfg,
		filter:          filter,
		builderRegistry: builderRegistry,
	}

	// Define AST node types we're interested in
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),   // Function/method calls
		(*ast.File)(nil),       // Files (for SQL builder analysis)
		(*ast.AssignStmt)(nil), // Assignment statements for standalone literals
		(*ast.GenDecl)(nil),    // General declarations (const, var, type)
		(*ast.BinaryExpr)(nil), // Binary expressions for string concatenation
	}

	// Walk through all AST nodes and analyze them
	insp.Preorder(nodeFilter, ctx.handleNode)

	return nil, nil
}

// handleNode dispatches AST node to appropriate handler
func (ctx *analysisContext) handleNode(n ast.Node) {
	switch node := n.(type) {
	case *ast.File:
		ctx.handleFileNode(node)
	case *ast.AssignStmt:
		// Check assignment statements for standalone SQL literals
		checkAssignStmt(ctx.pass, node, ctx.cfg)
	case *ast.GenDecl:
		// Check constant and variable declarations
		checkGenDecl(ctx.pass, node, ctx.cfg)
	case *ast.CallExpr:
		ctx.handleCallExpr(node)
		// Analyze function calls for SQL with SELECT * usage
	case *ast.BinaryExpr:
		ctx.handleBinaryExpr(node)
	}
}

// handleFileNode processes file-level analysis
func (ctx *analysisContext) handleFileNode(node *ast.File) {
	// Note: SQL builder analysis with type checking is done by Registry.Check() in handleCallExpr.
	// The old analyzeSQLBuilders() function was removed because it didn't use type checking
	// and caused false positives (issue #5).

	if ctx.cfg.N1DetectionEnabled {
		AnalyzeN1(ctx.pass, node)
	}
	if ctx.cfg.SQLInjectionDetectionEnabled {
		AnalyzeSQLInjection(ctx.pass, node)
	}
	if ctx.cfg.TxLeakDetectionEnabled {
		AnalyzeTxLeaks(ctx.pass, node)
	}
}

// handleCallExpr processes function/method call expressions
func (ctx *analysisContext) handleCallExpr(node *ast.CallExpr) {
	if ctx.filter != nil && ctx.filter.IsIgnoredFunction(node) {
		return
	}

	if ctx.cfg.CheckFormatStrings && CheckFormatFunction(ctx.pass, node, ctx.cfg) {
		ctx.pass.Report(analysis.Diagnostic{
			Pos:     node.Pos(),
			Message: getDetailedWarningMessage("format_string"),
		})
		return
	}

	if ctx.builderRegistry != nil && ctx.builderRegistry.HasCheckers() {
		violations := ctx.builderRegistry.Check(ctx.pass.TypesInfo, node)
		for _, v := range violations {
			ctx.pass.Report(analysis.Diagnostic{
				Pos:     v.Pos,
				End:     v.End,
				Message: v.Message,
			})
		}
		if len(violations) > 0 {
			return
		}
	}

	checkCallExpr(ctx.pass, node, ctx.cfg)
}

// handleBinaryExpr processes binary expressions (string concatenation)
func (ctx *analysisContext) handleBinaryExpr(node *ast.BinaryExpr) {
	if ctx.cfg.CheckStringConcat && CheckConcatenation(ctx.pass, node, ctx.cfg) {
		ctx.pass.Report(analysis.Diagnostic{
			Pos:     node.Pos(),
			Message: getDetailedWarningMessage("concat"),
		})
	}
}

// checkAssignStmt checks assignment statements for standalone SQL literals
func checkAssignStmt(pass *analysis.Pass, stmt *ast.AssignStmt, cfg *config.UnqueryvetSettings) {
	// Check right-hand side expressions for string literals with SELECT *
	for _, expr := range stmt.Rhs {
		// Only check direct string literals, not function calls
		if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			content := normalizeSQLQuery(lit.Value)
			if isSelectStarQuery(content, cfg) {
				pass.Report(analysis.Diagnostic{
					Pos:     lit.Pos(),
					Message: getWarningMessage(),
				})
			}
		}
	}
}

// checkGenDecl checks general declarations (const, var) for SELECT * in SQL queries
func checkGenDecl(pass *analysis.Pass, decl *ast.GenDecl, cfg *config.UnqueryvetSettings) {
	// Only check const and var declarations
	if decl.Tok != token.CONST && decl.Tok != token.VAR {
		return
	}

	// Iterate through all specifications in the declaration
	for _, spec := range decl.Specs {
		// Type assert to ValueSpec (const/var specifications)
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		// Check all values in the specification
		for _, value := range valueSpec.Values {
			// Only check direct string literals
			if lit, ok := value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				content := normalizeSQLQuery(lit.Value)
				if isSelectStarQuery(content, cfg) {
					pass.Report(analysis.Diagnostic{
						Pos:     lit.Pos(),
						Message: getWarningMessage(),
					})
				}
			}
		}
	}
}

// checkCallExpr analyzes function calls for SQL with SELECT * usage
// Note: SQL builder checking with type verification is done by Registry.Check() in handleCallExpr.
// This function only checks raw SQL strings in function arguments.
func checkCallExpr(pass *analysis.Pass, call *ast.CallExpr, cfg *config.UnqueryvetSettings) {
	// Check function call arguments for strings with SELECT *
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			content := normalizeSQLQuery(lit.Value)
			if isSelectStarQuery(content, cfg) {
				pass.Report(analysis.Diagnostic{
					Pos:     lit.Pos(),
					Message: getWarningMessage(),
				})
			}
		}
	}
}

// NormalizeSQLQuery normalizes SQL query for analysis with advanced escape sequence handling.
// Exported for testing purposes.
func NormalizeSQLQuery(query string) string {
	return normalizeSQLQuery(query)
}

func normalizeSQLQuery(query string) string {
	if len(query) < 2 {
		return query
	}

	first, last := query[0], query[len(query)-1]

	// 1. Handle different quote types with escape sequence processing
	if first == '"' && last == '"' {
		// For regular strings check for escape sequences
		if !strings.Contains(query, "\\") {
			query = trimQuotes(query)
		} else if unquoted, err := strconv.Unquote(query); err == nil {
			// Use standard Go unquoting for proper escape sequence handling
			query = unquoted
		} else {
			// Fallback: simple quote removal
			query = trimQuotes(query)
		}
	} else if first == '`' && last == '`' {
		// Raw strings - simply remove backticks
		query = trimQuotes(query)
	}

	// 2. Process comments line by line before normalization
	lines := strings.Split(query, "\n")
	var processedParts []string

	for _, line := range lines {
		// Remove comments from current line
		if idx := strings.Index(line, "--"); idx != -1 {
			line = line[:idx]
		}

		// Add non-empty lines
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			processedParts = append(processedParts, trimmed)
		}
	}

	// 3. Reassemble query and normalize
	query = strings.Join(processedParts, " ")
	query = strings.ToUpper(query)
	query = strings.ReplaceAll(query, "\t", " ")
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	return strings.TrimSpace(query)
}

// trimQuotes removes first and last character (quotes)
func trimQuotes(query string) string {
	return query[1 : len(query)-1]
}

// IsSelectStarQuery determines if query contains SELECT * with enhanced allowed patterns support.
// Exported for testing purposes.
func IsSelectStarQuery(query string, cfg *config.UnqueryvetSettings) bool {
	return isSelectStarQuery(query, cfg)
}

func isSelectStarQuery(query string, cfg *config.UnqueryvetSettings) bool {
	// Check allowed patterns first - if query matches an allowed pattern, ignore it
	for _, pattern := range cfg.AllowedPatterns {
		if matched, _ := regexp.MatchString(pattern, query); matched {
			return false
		}
	}

	upperQuery := strings.ToUpper(query)

	// Check for SELECT * in query (case-insensitive)
	if strings.Contains(upperQuery, "SELECT *") { //nolint:unqueryvet
		// Ensure this is actually an SQL query by checking for SQL keywords
		sqlKeywords := []string{"FROM", "WHERE", "JOIN", "GROUP", "ORDER", "HAVING", "UNION", "LIMIT"}
		for _, keyword := range sqlKeywords {
			if strings.Contains(upperQuery, keyword) {
				return true
			}
		}

		// Also check if it's just "SELECT *" without other keywords (still problematic)
		trimmed := strings.TrimSpace(upperQuery)
		if trimmed == "SELECT *" {
			return true
		}
	}

	// Check for SELECT alias.* patterns (e.g., SELECT t.*, SELECT u.*, o.*)
	if cfg.CheckAliasedWildcard && isSelectAliasStarQuery(query) {
		return true
	}

	// Check for SELECT * in subqueries (e.g., (SELECT * FROM ...))
	if cfg.CheckSubqueries && isSelectStarInSubquery(query) {
		return true
	}

	return false
}

// isSelectAliasStarQuery detects SELECT alias.* patterns like "SELECT t.*", "SELECT u.*, o.*"
func isSelectAliasStarQuery(query string) bool {
	return aliasedWildcardPattern.MatchString(query)
}

// isSelectStarInSubquery detects SELECT * in subqueries like "(SELECT * FROM ...)"
func isSelectStarInSubquery(query string) bool {
	return subquerySelectStarPattern.MatchString(query)
}

// getWarningMessage returns informative warning message
func getWarningMessage() string {
	return defaultWarningMessage
}

// getDetailedWarningMessage returns context-specific warning message
func getDetailedWarningMessage(context string) string {
	switch context {
	case "sql_builder":
		return "avoid SELECT * in SQL builder - explicitly specify columns to prevent unnecessary data transfer and schema change issues"
	case "nested":
		return "avoid SELECT * in subquery - can cause performance issues and unexpected results when schema changes"
	case "empty_select":
		return "SQL builder Select() without columns defaults to SELECT * - add specific columns with .Columns() method"
	case "aliased_wildcard":
		return "avoid SELECT alias.* - explicitly specify columns like alias.id, alias.name for better maintainability"
	case "subquery":
		return "avoid SELECT * in subquery - specify columns explicitly to prevent issues when schema changes"
	case "concat":
		return "avoid SELECT * in concatenated query - explicitly specify needed columns"
	case "format_string":
		return "avoid SELECT * in format string - explicitly specify needed columns"
	default:
		return defaultWarningMessage
	}
}

// IsRuleEnabledExported checks if a rule is enabled in the configuration.
// A rule is enabled if it exists in the Rules map and its severity is not "ignore".
func IsRuleEnabledExported(rules config.RuleSeverity, ruleID string) bool {
	if rules == nil {
		return false
	}
	severity, exists := rules[ruleID]
	if !exists {
		return false
	}
	return severity != "ignore"
}

// isRuleEnabled is an internal helper for checking rule enablement.
func isRuleEnabled(rules config.RuleSeverity, ruleID string) bool {
	return IsRuleEnabledExported(rules, ruleID)
}
