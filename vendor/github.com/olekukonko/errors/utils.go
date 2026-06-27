// Package errors provides utility functions for error handling, including stack
// trace capture and function name extraction.
package errors

import (
	"database/sql"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// captureStack captures a stack trace with the configured depth.
// Skip=0 captures the current call site; skips captureStack and its caller (+2 frames); thread-safe via stackPool.
func captureStack(skip int) []uintptr {
	buf := stackPool.Get().([]uintptr)
	buf = buf[:cap(buf)]

	// +2 to skip captureStack and the immediate caller
	n := runtime.Callers(skip+2, buf)
	if n == 0 {
		stackPool.Put(buf)
		return nil
	}

	// Create a new slice to return, avoiding direct use of pooled memory
	stack := make([]uintptr, n)
	copy(stack, buf[:n])
	stackPool.Put(buf)

	return stack
}

// min returns the smaller of two integers.
// Simple helper for limiting stack trace size or other comparisons.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// clearMap removes all entries from a map.
// Helper function to reset map contents without reallocating.
func clearMap(m map[string]interface{}) {
	for k := range m {
		delete(m, k)
	}
}

// sqlNull detects if a value represents a SQL NULL type.
// Returns true for nil or invalid sql.Null* types (e.g., NullString, NullInt64); false otherwise.
func sqlNull(v interface{}) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case sql.NullString:
		return !val.Valid
	case sql.NullTime:
		return !val.Valid
	case sql.NullInt64:
		return !val.Valid
	case sql.NullBool:
		return !val.Valid
	case sql.NullFloat64:
		return !val.Valid
	default:
		return false
	}
}

// getFuncName extracts the function name from an interface, typically a function or method.
// Returns "unknown" if the input is nil or invalid; trims leading dots from runtime name.
func getFuncName(fn interface{}) string {
	if fn == nil {
		return "unknown"
	}
	fullName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	return strings.TrimPrefix(fullName, ".")
}

// isInternalFrame determines if a stack frame is considered "internal".
// Returns true for frames from runtime, reflect, or this package’s subdirectories if FilterInternal is true.
func isInternalFrame(frame runtime.Frame) bool {
	if strings.HasPrefix(frame.Function, "runtime.") || strings.HasPrefix(frame.Function, "reflect.") {
		return true
	}

	suffixes := []string{
		"errors",
		"utils",
		"helper",
		"retry",
		"multi",
	}

	file := frame.File
	for _, v := range suffixes {
		if strings.Contains(file, fmt.Sprintf("github.com/olekukonko/errors/%s", v)) {
			return true
		}
	}
	return false
}

// FormatError returns a formatted string representation of an error.
// Includes message, name, context, stack trace, and cause for *Error types; just message for others; "<nil>" if nil.
func FormatError(err error) string {
	if err == nil {
		return "<nil>"
	}
	var sb strings.Builder
	if e, ok := err.(*Error); ok {
		sb.WriteString(fmt.Sprintf("Error: %s\n", e.Error()))
		if e.name != "" {
			sb.WriteString(fmt.Sprintf("Name: %s\n", e.name))
		}
		if ctx := e.Context(); len(ctx) > 0 {
			sb.WriteString("Context:\n")
			for k, v := range ctx {
				sb.WriteString(fmt.Sprintf("\t%s: %v\n", k, v))
			}
		}
		if stack := e.Stack(); len(stack) > 0 {
			sb.WriteString("Stack Trace:\n")
			for _, frame := range stack {
				sb.WriteString(fmt.Sprintf("\t%s\n", frame))
			}
		}
		if e.cause != nil {
			sb.WriteString(fmt.Sprintf("Caused by: %s\n", FormatError(e.cause)))
		}
	} else {
		sb.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
	}
	return sb.String()
}

// Caller returns the file, line, and function name of the caller at the specified skip level.
// Skip=0 returns the caller of this function, 1 returns its caller, etc.; returns "unknown" if no caller found.
func Caller(skip int) (file string, line int, function string) {
	configMu.RLock()
	defer configMu.RUnlock()
	var pcs [1]uintptr
	n := runtime.Callers(skip+2, pcs[:]) // +2 skips Caller and its immediate caller
	if n == 0 {
		return "", 0, "unknown"
	}
	frame, _ := runtime.CallersFrames(pcs[:n]).Next()
	return frame.File, frame.Line, frame.Function
}
