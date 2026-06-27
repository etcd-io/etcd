// Package analyzer provides the SQL static analysis implementation for detecting SELECT * usage.
package analyzer

import (
	"go/ast"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MirrexOne/unqueryvet/pkg/config"
)

// FilterContext holds precompiled patterns for filtering files, functions, and queries.
// It provides efficient filtering by compiling regex patterns once during initialization.
type FilterContext struct {
	ignoredFuncPatterns []*regexp.Regexp
	ignoredFilePatterns []string
	allowedPatterns     []*regexp.Regexp
}

// NewFilterContext creates a new FilterContext from settings.
// It precompiles all regex patterns for efficient filtering.
// Returns an error if any pattern is invalid.
func NewFilterContext(cfg *config.UnqueryvetSettings) (*FilterContext, error) {
	fc := &FilterContext{
		ignoredFilePatterns: cfg.IgnoredFiles,
	}

	// Compile ignored function patterns
	for _, pattern := range cfg.IgnoredFunctions {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		fc.ignoredFuncPatterns = append(fc.ignoredFuncPatterns, re)
	}

	// Compile allowed query patterns
	for _, pattern := range cfg.AllowedPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		fc.allowedPatterns = append(fc.allowedPatterns, re)
	}

	return fc, nil
}

// IsIgnoredFunction checks if a function call should be ignored based on configured patterns.
// It extracts the full function name (package.function or receiver.method) and matches against patterns.
func (fc *FilterContext) IsIgnoredFunction(call *ast.CallExpr) bool {
	if len(fc.ignoredFuncPatterns) == 0 {
		return false
	}

	funcName := extractFunctionName(call)
	if funcName == "" {
		return false
	}

	for _, pattern := range fc.ignoredFuncPatterns {
		if pattern.MatchString(funcName) {
			return true
		}
	}

	return false
}

// IsIgnoredFile checks if a file path matches any of the ignored file patterns.
// Supports glob patterns like "*_test.go", "testdata/**", "mock_*.go".
func (fc *FilterContext) IsIgnoredFile(filePath string) bool {
	if len(fc.ignoredFilePatterns) == 0 {
		return false
	}

	// Normalize path separators for cross-platform support
	normalizedPath := filepath.ToSlash(filePath)
	baseName := filepath.Base(filePath)

	for _, pattern := range fc.ignoredFilePatterns {
		// Try matching against full path
		if matched, _ := filepath.Match(pattern, normalizedPath); matched {
			return true
		}
		// Try matching against base name only
		if matched, _ := filepath.Match(pattern, baseName); matched {
			return true
		}
		// Handle ** patterns (recursive matching)
		if strings.Contains(pattern, "**") {
			if matchDoubleStarPattern(pattern, normalizedPath) {
				return true
			}
		}
	}

	return false
}

// IsAllowedPattern checks if a query matches any of the allowed patterns.
// Returns true if the query should be allowed (not reported as a violation).
func (fc *FilterContext) IsAllowedPattern(query string) bool {
	for _, pattern := range fc.allowedPatterns {
		if pattern.MatchString(query) {
			return true
		}
	}
	return false
}

// extractFunctionName extracts the full function name from a call expression.
// Returns formats like "pkg.Function", "receiver.Method", or just "Function".
func extractFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Method call: obj.Method() or pkg.Function()
		switch x := fun.X.(type) {
		case *ast.Ident:
			// pkg.Function() or receiver.Method()
			return x.Name + "." + fun.Sel.Name
		case *ast.SelectorExpr:
			// Nested: pkg.subpkg.Function()
			if ident, ok := x.X.(*ast.Ident); ok {
				return ident.Name + "." + x.Sel.Name + "." + fun.Sel.Name
			}
		case *ast.CallExpr:
			// Chained call: obj.Method1().Method2()
			return fun.Sel.Name
		}
		return fun.Sel.Name
	case *ast.Ident:
		// Direct function call: Function()
		return fun.Name
	}
	return ""
}

// matchDoubleStarPattern handles glob patterns with ** (recursive matching).
// Example: "testdata/**" matches "testdata/foo/bar.go"
func matchDoubleStarPattern(pattern, path string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false
	}

	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	// Check if path starts with prefix
	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}

	// Check if path ends with suffix (if suffix exists)
	if suffix != "" {
		// Get the part after prefix
		remainingPath := strings.TrimPrefix(path, prefix)
		remainingPath = strings.TrimPrefix(remainingPath, "/")

		// Match suffix at the end or as a pattern
		if matched, _ := filepath.Match(suffix, filepath.Base(path)); matched {
			return true
		}
		if strings.HasSuffix(remainingPath, suffix) {
			return true
		}
	} else {
		// No suffix, just check prefix
		return strings.HasPrefix(path, prefix)
	}

	return false
}
