// Package analyzer provides the SQL static analysis implementation for detecting SELECT * usage.
package analyzer

import (
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// SuggestedFixGenerator generates auto-fix suggestions for SELECT * violations.
type SuggestedFixGenerator struct {
	fset *token.FileSet
}

// NewSuggestedFixGenerator creates a new SuggestedFixGenerator.
func NewSuggestedFixGenerator(fset *token.FileSet) *SuggestedFixGenerator {
	return &SuggestedFixGenerator{
		fset: fset,
	}
}

// selectStarPattern matches SELECT * patterns for replacement
var selectStarFixPattern = regexp.MustCompile(`(?i)(SELECT\s+)\*(\s+FROM)`)

// aliasedStarPattern matches SELECT alias.* patterns for replacement
var aliasedStarFixPattern = regexp.MustCompile(`(?i)(SELECT\s+)([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*\*`)

// GenerateFix creates a SuggestedFix for a SELECT * violation.
func (sfg *SuggestedFixGenerator) GenerateFix(
	pos token.Pos,
	end token.Pos,
	originalText string,
	violationType string,
) *analysis.SuggestedFix {
	var newText string
	var message string

	switch violationType {
	case "select_star":
		// Replace SELECT * with placeholder columns
		newText = selectStarFixPattern.ReplaceAllString(originalText, "${1}id, /* TODO: specify columns */ ${2}")
		message = "Replace SELECT * with explicit columns"

	case "aliased_wildcard":
		// Replace SELECT t.* with placeholder
		newText = aliasedStarFixPattern.ReplaceAllStringFunc(originalText, func(match string) string {
			// Extract the alias
			parts := aliasedStarFixPattern.FindStringSubmatch(match)
			if len(parts) >= 3 {
				alias := parts[2]
				return parts[1] + alias + ".id, " + alias + "./* TODO: specify columns */"
			}
			return match
		})
		message = "Replace SELECT alias.* with explicit columns"

	case "sql_builder_star":
		// For SQL builder Select("*") -> Select("id", /* TODO */)
		newText = strings.Replace(originalText, `"*"`, `"id", /* TODO: specify columns */`, 1)
		message = "Replace \"*\" with explicit column names"

	case "empty_select":
		// For empty Select() -> Select("id", /* TODO */)
		newText = strings.Replace(originalText, "Select()", `Select("id", /* TODO: specify columns */)`, 1)
		message = "Add column names to Select()"

	default:
		// Generic replacement
		newText = selectStarFixPattern.ReplaceAllString(originalText, "${1}id, /* TODO: specify columns */ ${2}")
		message = "Replace SELECT * with explicit columns"
	}

	// If no change was made, don't suggest a fix
	if newText == originalText {
		return nil
	}

	return &analysis.SuggestedFix{
		Message: message,
		TextEdits: []analysis.TextEdit{
			{
				Pos:     pos,
				End:     end,
				NewText: []byte(newText),
			},
		},
	}
}

// GenerateColumnPlaceholder returns a placeholder string for explicit columns.
func (sfg *SuggestedFixGenerator) GenerateColumnPlaceholder() string {
	return "id, /* TODO: specify columns */"
}

// GenerateAliasedColumnPlaceholder returns a placeholder with table alias.
func (sfg *SuggestedFixGenerator) GenerateAliasedColumnPlaceholder(alias string) string {
	return alias + ".id, " + alias + "./* TODO: specify columns */"
}

// CreateDiagnosticWithFix creates a Diagnostic with an optional SuggestedFix.
func CreateDiagnosticWithFix(
	pos token.Pos,
	end token.Pos,
	message string,
	originalText string,
	violationType string,
	fset *token.FileSet,
) analysis.Diagnostic {
	diagnostic := analysis.Diagnostic{
		Pos:     pos,
		End:     end,
		Message: message,
	}

	// Generate fix if we have enough information
	if originalText != "" && fset != nil {
		generator := NewSuggestedFixGenerator(fset)
		if fix := generator.GenerateFix(pos, end, originalText, violationType); fix != nil {
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{*fix}
		}
	}

	return diagnostic
}
