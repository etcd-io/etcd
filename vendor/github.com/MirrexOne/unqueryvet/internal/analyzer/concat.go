// Package analyzer provides the SQL static analysis implementation for detecting SELECT * usage.
package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/MirrexOne/unqueryvet/pkg/config"
)

// StringConcatAnalyzer analyzes string concatenation expressions for SELECT * patterns.
// It traverses binary expressions to combine string parts and check for SELECT * usage.
type StringConcatAnalyzer struct {
	pass *analysis.Pass
	cfg  *config.UnqueryvetSettings
}

// NewStringConcatAnalyzer creates a new StringConcatAnalyzer.
func NewStringConcatAnalyzer(pass *analysis.Pass, cfg *config.UnqueryvetSettings) *StringConcatAnalyzer {
	return &StringConcatAnalyzer{
		pass: pass,
		cfg:  cfg,
	}
}

// AnalyzeBinaryExpr analyzes a binary expression for string concatenation with SELECT *.
// Returns true if SELECT * was detected in the concatenated string.
func (sca *StringConcatAnalyzer) AnalyzeBinaryExpr(expr *ast.BinaryExpr) bool {
	// Only analyze string concatenation (+ operator)
	if expr.Op != token.ADD {
		return false
	}

	// Extract all string parts from the concatenation chain
	parts := sca.extractStringParts(expr)
	if len(parts) == 0 {
		return false
	}

	// Combine parts and check for SELECT *
	combined := strings.Join(parts, "")
	if combined == "" {
		return false
	}

	// Normalize and check for SELECT *
	normalized := normalizeSQLQuery("\"" + combined + "\"")
	return isSelectStarQuery(normalized, sca.cfg)
}

// extractStringParts recursively extracts all string literal parts from a concatenation chain.
// For expressions like: "SELECT * " + "FROM " + tableName + " WHERE id = 1"
// It extracts: ["SELECT * ", "FROM ", " WHERE id = 1"]
func (sca *StringConcatAnalyzer) extractStringParts(expr ast.Expr) []string {
	var parts []string

	switch e := expr.(type) {
	case *ast.BasicLit:
		// String literal
		if e.Kind == token.STRING {
			// Remove quotes from the string
			value := strings.Trim(e.Value, "`\"")
			parts = append(parts, value)
		}
	case *ast.BinaryExpr:
		// Concatenation - recurse into both sides
		if e.Op == token.ADD {
			parts = append(parts, sca.extractStringParts(e.X)...)
			parts = append(parts, sca.extractStringParts(e.Y)...)
		}
	case *ast.Ident:
		// Variable reference - we can't know the value at compile time
		// but we can check if it's a constant
		if e.Obj != nil && e.Obj.Kind == ast.Con {
			if valueSpec, ok := e.Obj.Decl.(*ast.ValueSpec); ok {
				for _, value := range valueSpec.Values {
					if lit, ok := value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						strValue := strings.Trim(lit.Value, "`\"")
						parts = append(parts, strValue)
					}
				}
			}
		}
	case *ast.ParenExpr:
		// Parenthesized expression - unwrap
		parts = append(parts, sca.extractStringParts(e.X)...)
	}

	return parts
}

// CheckConcatenation is a convenience function to check a binary expression for SELECT *.
// It creates an analyzer and performs the check in one call.
func CheckConcatenation(pass *analysis.Pass, expr *ast.BinaryExpr, cfg *config.UnqueryvetSettings) bool {
	analyzer := NewStringConcatAnalyzer(pass, cfg)
	return analyzer.AnalyzeBinaryExpr(expr)
}
