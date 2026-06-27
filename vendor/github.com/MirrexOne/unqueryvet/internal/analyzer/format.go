// Package analyzer provides the SQL static analysis implementation for detecting SELECT * usage.
package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/MirrexOne/unqueryvet/pkg/config"
)

// formatFunctions maps package.function names to the index of the format string argument.
// Index -1 means the format string is the last argument (for variadic functions).
var formatFunctions = map[string]int{
	// fmt package
	"fmt.Sprintf": 0,
	"fmt.Printf":  0,
	"fmt.Fprintf": 1, // first arg is io.Writer
	"fmt.Errorf":  0,
	"fmt.Fscanf":  1,
	"fmt.Sscanf":  1,
	"Sprintf":     0, // direct call after import
	"Printf":      0,
	"Errorf":      0,

	// log package
	"log.Printf":    0,
	"log.Fatalf":    0,
	"log.Panicf":    0,
	"log.Logf":      1, // first arg is log level
	"Logger.Printf": 0,
	"Logger.Fatalf": 0,
	"Logger.Panicf": 0,

	// testing package
	"testing.T.Logf":   0,
	"testing.T.Errorf": 0,
	"testing.T.Fatalf": 0,
	"testing.T.Skipf":  0,
	"testing.B.Logf":   0,
	"testing.B.Errorf": 0,
	"testing.B.Fatalf": 0,
	"T.Logf":           0,
	"T.Errorf":         0,
	"T.Fatalf":         0,
	"B.Logf":           0,
	"B.Errorf":         0,
	"B.Fatalf":         0,

	// errors package
	"errors.Errorf": 0,
	"errors.Wrapf":  1, // first arg is error

	// pkg/errors
	"errors.WithMessagef": 1,

	// logrus
	"logrus.Infof":  0,
	"logrus.Warnf":  0,
	"logrus.Errorf": 0,
	"logrus.Debugf": 0,
	"logrus.Fatalf": 0,
	"logrus.Panicf": 0,
	"logrus.Tracef": 0,
	"logrus.Printf": 0,
	"Entry.Infof":   0,
	"Entry.Warnf":   0,
	"Entry.Errorf":  0,
	"Entry.Debugf":  0,

	// zap
	"zap.S.Infof":          0,
	"zap.S.Warnf":          0,
	"zap.S.Errorf":         0,
	"zap.S.Debugf":         0,
	"zap.S.Fatalf":         0,
	"zap.S.Panicf":         0,
	"SugaredLogger.Infof":  0,
	"SugaredLogger.Warnf":  0,
	"SugaredLogger.Errorf": 0,
	"SugaredLogger.Debugf": 0,
}

// FormatStringAnalyzer analyzes format functions like fmt.Sprintf for SELECT * patterns.
type FormatStringAnalyzer struct {
	pass *analysis.Pass
	cfg  *config.UnqueryvetSettings
}

// NewFormatStringAnalyzer creates a new FormatStringAnalyzer.
func NewFormatStringAnalyzer(pass *analysis.Pass, cfg *config.UnqueryvetSettings) *FormatStringAnalyzer {
	return &FormatStringAnalyzer{
		pass: pass,
		cfg:  cfg,
	}
}

// AnalyzeFormatCall analyzes a function call for format string patterns with SELECT *.
// Returns true if SELECT * was detected in the format string.
func (fsa *FormatStringAnalyzer) AnalyzeFormatCall(call *ast.CallExpr) bool {
	// Get the function name
	funcName := fsa.getFunctionName(call)
	if funcName == "" {
		return false
	}

	// Check if this is a known format function
	argIndex, ok := formatFunctions[funcName]
	if !ok {
		return false
	}

	// Extract the format string
	formatStr, ok := fsa.extractFormatString(call, argIndex)
	if !ok {
		return false
	}

	// Check if the format string contains SELECT *
	normalized := normalizeSQLQuery("\"" + formatStr + "\"")
	return isSelectStarQuery(normalized, fsa.cfg)
}

// getFunctionName extracts the function name from a call expression.
// Returns formats like "fmt.Sprintf", "log.Printf", or just "Sprintf".
func (fsa *FormatStringAnalyzer) getFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Method call: pkg.Func() or obj.Method()
		switch x := fun.X.(type) {
		case *ast.Ident:
			// pkg.Func() like fmt.Sprintf
			return x.Name + "." + fun.Sel.Name
		case *ast.SelectorExpr:
			// Nested: pkg.subpkg.Func() or receiver.Type.Method()
			if ident, ok := x.X.(*ast.Ident); ok {
				return ident.Name + "." + x.Sel.Name + "." + fun.Sel.Name
			}
			// obj.Field.Method()
			return x.Sel.Name + "." + fun.Sel.Name
		case *ast.CallExpr:
			// Chained: something().Method()
			return fun.Sel.Name
		}
		return fun.Sel.Name
	case *ast.Ident:
		// Direct function call: Sprintf() (after import . "fmt")
		return fun.Name
	}
	return ""
}

// extractFormatString extracts the format string from a function call.
// argIndex specifies which argument contains the format string.
func (fsa *FormatStringAnalyzer) extractFormatString(call *ast.CallExpr, argIndex int) (string, bool) {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return "", false
	}

	arg := call.Args[argIndex]

	// Direct string literal
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, "`\""), true
	}

	// Identifier (constant or variable)
	if ident, ok := arg.(*ast.Ident); ok {
		if ident.Obj != nil && ident.Obj.Kind == ast.Con {
			// It's a constant - try to get its value
			if valueSpec, ok := ident.Obj.Decl.(*ast.ValueSpec); ok {
				for _, value := range valueSpec.Values {
					if lit, ok := value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						return strings.Trim(lit.Value, "`\""), true
					}
				}
			}
		}
	}

	return "", false
}

// CheckFormatFunction is a convenience function to check a call expression for SELECT *.
// It creates an analyzer and performs the check in one call.
func CheckFormatFunction(pass *analysis.Pass, call *ast.CallExpr, cfg *config.UnqueryvetSettings) bool {
	analyzer := NewFormatStringAnalyzer(pass, cfg)
	return analyzer.AnalyzeFormatCall(call)
}

// IsFormatFunction checks if a call expression is a known format function.
func IsFormatFunction(call *ast.CallExpr) bool {
	var funcName string

	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		switch x := fun.X.(type) {
		case *ast.Ident:
			funcName = x.Name + "." + fun.Sel.Name
		case *ast.SelectorExpr:
			if ident, ok := x.X.(*ast.Ident); ok {
				funcName = ident.Name + "." + x.Sel.Name + "." + fun.Sel.Name
			} else {
				funcName = x.Sel.Name + "." + fun.Sel.Name
			}
		default:
			funcName = fun.Sel.Name
		}
	case *ast.Ident:
		funcName = fun.Name
	}

	_, ok := formatFunctions[funcName]
	return ok
}
