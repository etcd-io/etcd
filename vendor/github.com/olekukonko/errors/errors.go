// Package errors provides a robust error handling library with support for
// error wrapping, stack traces, context storage, and retry mechanisms. It extends
// the standard library's error interface with features like HTTP-like status codes,
// error categorization, and JSON serialization, while maintaining compatibility
// with `errors.Is`, `errors.As`, and `errors.Unwrap`. The package is thread-safe
// and optimized with object pooling for performance.
package errors

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// Constants defining default configuration and context keys.
const (
	ctxTimeout = "[error] timeout" // Context key marking timeout errors.
	ctxRetry   = "[error] retry"   // Context key marking retryable errors.

	contextSize = 4   // Initial size of fixed-size context array for small contexts.
	bufferSize  = 256 // Initial buffer size for JSON marshaling.
	warmUpSize  = 100 // Number of errors to pre-warm the pool for efficiency.
	stackDepth  = 32  // Maximum stack trace depth to prevent excessive memory use.

	DefaultCode = 500 // Default HTTP status code for errors if not specified.
)

// spaceRe is a precompiled regex for normalizing whitespace in error messages.
var spaceRe = regexp.MustCompile(`\s+`)

// ErrorCategory is a string type for categorizing errors (e.g., "network", "validation").
type ErrorCategory string

// ErrorOpts provides options for customizing error creation.
type ErrorOpts struct {
	SkipStack int // Number of stack frames to skip when capturing the stack trace.
}

// Config defines the global configuration for the errors package, controlling
// stack depth, context size, pooling, and frame filtering.
type Config struct {
	StackDepth     int  // Maximum stack trace depth; 0 uses default (32).
	ContextSize    int  // Initial context map size; 0 uses default (4).
	DisablePooling bool // If true, disables object pooling for errors.
	FilterInternal bool // If true, filters internal package frames from stack traces.
	AutoFree       bool // If true, automatically frees errors to pool after use.
}

// cachedConfig holds the current configuration, updated only by Configure().
// Protected by configMu for thread-safety.
type cachedConfig struct {
	stackDepth     int
	contextSize    int
	disablePooling bool
	filterInternal bool
	autoFree       bool
}

var (
	// currentConfig stores the active configuration, read frequently and updated rarely.
	currentConfig cachedConfig
	// configMu protects updates to currentConfig for thread-safety.
	configMu sync.RWMutex
	// errorPool manages reusable Error instances to reduce allocations.
	errorPool = NewErrorPool()
	// stackPool manages reusable stack trace slices for efficiency.
	stackPool = sync.Pool{
		New: func() interface{} {
			return make([]uintptr, currentConfig.stackDepth)
		},
	}
	// emptyError is a pre-allocated empty error for lightweight reuse.
	emptyError = &Error{
		smallContext: [contextSize]contextItem{},
		msg:          "",
		name:         "",
		template:     "",
		cause:        nil,
	}
)

// contextItem holds a single key-value pair in the smallContext array.
type contextItem struct {
	key   string
	value interface{}
}

// Error is a custom error type with enhanced features: message, name, stack trace,
// context, cause, and metadata like code and category. It is thread-safe and
// supports pooling for performance.
type Error struct {
	// Fields used in atomic operations. Place them at the beginning of the
	// struct to ensure proper alignment across all architectures.
	count uint64 // Occurrence count for tracking frequency.

	// Primary fields (frequently accessed).
	msg   string    // The error message displayed by Error().
	name  string    // The error name or type (e.g., "AuthError").
	stack []uintptr // Stack trace as program counters.

	// Secondary metadata.
	template   string // Fallback message template if msg is empty.
	category   string // Error category (e.g., "network").
	code       int32  // HTTP-like status code (e.g., 400, 500).
	smallCount int32  // Number of items in smallContext.

	// Context and chaining.
	context      map[string]interface{}   // Key-value pairs for additional context.
	cause        error                    // Wrapped underlying error for chaining.
	callback     func()                   // Optional callback invoked by Error().
	smallContext [contextSize]contextItem // Fixed-size array for small contexts.

	// Synchronization.
	mu sync.RWMutex // Protects mutable fields (context, smallContext).

	// Internal flags.
	formatWrapped bool // True if created by Newf with %w verb.
}

// init sets up the package with default configuration and pre-warms the error pool.
func init() {
	currentConfig = cachedConfig{
		stackDepth:     stackDepth,
		contextSize:    contextSize,
		disablePooling: false,
		filterInternal: true,
		autoFree:       true,
	}
	WarmPool(warmUpSize) // Pre-allocate errors for performance.
}

// Configure updates the global configuration for the errors package.
// It is thread-safe and should be called early to avoid race conditions.
// Changes apply to all subsequent error operations.
// Example:
//
//	errors.Configure(errors.Config{StackDepth: 16, DisablePooling: true})
func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()

	if cfg.StackDepth != 0 {
		currentConfig.stackDepth = cfg.StackDepth
	}
	if cfg.ContextSize != 0 {
		currentConfig.contextSize = cfg.ContextSize
	}
	currentConfig.disablePooling = cfg.DisablePooling
	currentConfig.filterInternal = cfg.FilterInternal
	currentConfig.autoFree = cfg.AutoFree
}

// newError creates a new Error instance, reusing from the pool if enabled.
// Initializes smallContext and sets stack to nil.
// Internal use; prefer New, Named, or Trace for public API.
func newError() *Error {
	if currentConfig.disablePooling {
		return &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
	}
	return errorPool.Get()
}

// Empty returns a new empty error with no message, name, or stack trace.
// Useful for incrementally building errors or as a neutral base.
// Example:
//
//	err := errors.Empty().With("key", "value").WithCode(400)
func Empty() *Error {
	return newError()
}

// Named creates an error with the specified name and captures a stack trace.
// The name doubles as the error message if no message is set.
// Use for errors where type identification and stack context are important.
// Example:
//
//	err := errors.Named("AuthError").WithCode(401)
func Named(name string) *Error {
	e := newError()
	e.name = name
	return e.WithStack()
}

// New creates a lightweight error with the given message and no stack trace.
// Optimized for performance; use Trace() for stack traces.
// Returns a shared empty error for empty messages to reduce allocations.
// Example:
//
//	err := errors.New("invalid input")
func New(text string) *Error {
	if text == "" {
		return emptyError.Copy() // Avoid modifying shared instance.
	}
	err := newError()
	err.msg = text
	return err
}

// Newf creates a formatted error, supporting the %w verb for wrapping errors.
// If the format contains exactly one %w verb with a non-nil error argument,
// the error is wrapped as the cause. The final error message string generated
// by Error() will be compatible with the output of fmt.Errorf for the same inputs.
// Does not capture a stack trace by default.
// Example:
//
//	cause := errors.New("db error")
//	err := errors.Newf("query failed: %w", cause)
//	// err.Error() will match fmt.Errorf("query failed: %w", cause).Error()
//	// errors.Unwrap(err) == cause
func Newf(f any, args ...interface{}) *Error {
	var format string
	switch v := f.(type) {
	case string:
		format = v
	case fmt.Stringer:
		format = v.String()
	default:
		panic("Newf: format must be a string or fmt.Stringer")
	}
	err := newError()

	var wCount int
	var wArgPos = -1
	var wArg error
	var validationErrorMsg string
	argPos := 0
	runes := []rune(format)
	i := 0
	parsingOk := true
	var fmtVerbs []struct {
		isW    bool
		spec   string // The full verb specifier or literal segment
		argIdx int    // Index in the original 'args' slice, -1 for literals/%%
	}

	// Parse format string to identify verbs and literals.
	for i < len(runes) && parsingOk {
		segmentStart := i
		if runes[i] == '%' {
			if i+1 >= len(runes) {
				parsingOk = false
				validationErrorMsg = "ends with %"
				break
			}
			if runes[i+1] == '%' {
				fmtVerbs = append(fmtVerbs, struct {
					isW    bool
					spec   string
					argIdx int
				}{isW: false, spec: "%%", argIdx: -1})
				i += 2
				continue
			}
			i++ // Move past '%'
			// Parse flags, width, precision (simplified loop)
			for i < len(runes) && strings.ContainsRune("+- #0", runes[i]) {
				i++
			}
			for i < len(runes) && ((runes[i] >= '0' && runes[i] <= '9') || runes[i] == '.') {
				i++
			}
			if i >= len(runes) {
				parsingOk = false
				validationErrorMsg = "ends mid-specifier"
				break
			}
			verb := runes[i]
			specifierEndIndex := i + 1
			fullSpec := string(runes[segmentStart:specifierEndIndex])
			// Check if the verb consumes an argument
			currentVerbConsumesArg := strings.ContainsRune("vTtbcdoqxXUeEfFgGspw", verb)
			currentArgIdx := -1
			isWVerb := false

			if verb == 'w' {
				isWVerb = true
				wCount++
				if wCount == 1 {
					wArgPos = argPos // Record position of the error argument
				} else {
					parsingOk = false
					validationErrorMsg = "multiple %w"
					break
				}
			}

			if currentVerbConsumesArg {
				if argPos >= len(args) {
					parsingOk = false
					if isWVerb { // More specific message for missing %w arg
						validationErrorMsg = "missing %w argument"
					} else {
						validationErrorMsg = fmt.Sprintf("missing argument for %s", string(verb))
					}
					break
				}
				currentArgIdx = argPos
				if isWVerb {
					cause, ok := args[argPos].(error)
					if !ok || cause == nil {
						parsingOk = false
						validationErrorMsg = "bad %w argument type"
						break
					}
					wArg = cause // Store the actual error argument
				}
				argPos++ // Consume the argument position
			}
			fmtVerbs = append(fmtVerbs, struct {
				isW    bool
				spec   string
				argIdx int
			}{isW: isWVerb, spec: fullSpec, argIdx: currentArgIdx})
			i = specifierEndIndex // Move past the verb character
		} else {
			// Handle literal segment
			literalStart := i
			for i < len(runes) && runes[i] != '%' {
				i++
			}
			fmtVerbs = append(fmtVerbs, struct {
				isW    bool
				spec   string
				argIdx int
			}{isW: false, spec: string(runes[literalStart:i]), argIdx: -1})
		}
	}

	// Check for too many arguments after parsing
	if parsingOk && argPos < len(args) {
		parsingOk = false
		validationErrorMsg = fmt.Sprintf("too many arguments for format %q", format)
	}

	// Handle format validation errors.
	if !parsingOk {
		switch validationErrorMsg {
		case "multiple %w":
			err.msg = fmt.Sprintf("errors.Newf: format %q has multiple %%w verbs", format)
		case "missing %w argument":
			err.msg = fmt.Sprintf("errors.Newf: format %q has %%w but not enough arguments", format)
		case "bad %w argument type":
			argValStr := "(<nil>)"
			if wArgPos >= 0 && wArgPos < len(args) && args[wArgPos] != nil {
				argValStr = fmt.Sprintf("(%T)", args[wArgPos])
			} else if wArgPos >= len(args) {
				argValStr = "(missing)" // Should be caught by "missing %w argument" case
			}
			err.msg = fmt.Sprintf("errors.Newf: argument %d for %%w is not a non-nil error %s", wArgPos, argValStr)
		case "ends with %":
			err.msg = fmt.Sprintf("errors.Newf: format %q ends with %%", format)
		case "ends mid-specifier":
			err.msg = fmt.Sprintf("errors.Newf: format %q ends during verb specifier", format)
		default: // Includes "too many arguments" and other potential fmt issues
			err.msg = fmt.Sprintf("errors.Newf: error in format %q: %s", format, validationErrorMsg)
		}
		err.cause = nil // Ensure no cause is set on format error
		err.formatWrapped = false
		return err
	}

	//  Start: Processing Valid Format String
	if wCount == 1 && wArg != nil {
		//  Handle %w: Simulate for Sprintf and pre-format
		err.cause = wArg         // Set the cause for unwrapping
		err.formatWrapped = true // Signal that msg is the final formatted string

		var finalFormat strings.Builder
		var finalArgs []interface{}
		causeStr := wArg.Error() // Get the string representation of the cause

		// Rebuild format string and argument list for Sprintf
		for _, verb := range fmtVerbs {
			if verb.isW {
				// Replace the %w verb specifier (e.g., "%w", "%+w") with "%s"
				finalFormat.WriteString("%s")
				// Add the cause's *string* to the arguments list for the new %s
				finalArgs = append(finalArgs, causeStr)
			} else {
				// Keep the original literal segment or non-%w verb specifier
				finalFormat.WriteString(verb.spec)
				if verb.argIdx != -1 {
					// Add the original argument for this non-%w verb/literal
					finalArgs = append(finalArgs, args[verb.argIdx])
				}
			}
		}

		// Format using the *modified* format string and arguments list
		result, fmtErr := FmtErrorCheck(finalFormat.String(), finalArgs...)
		if fmtErr != nil {
			// Handle potential errors during the final formatting step
			// This is unlikely if parsing passed, but possible with complex verbs/args
			err.msg = fmt.Sprintf("errors.Newf: formatting error during %%w simulation for format %q: %v", format, fmtErr)
			err.cause = nil // Don't keep the cause if final formatting failed
			err.formatWrapped = false
		} else {
			// Store the final, fully formatted string, matching fmt.Errorf output
			err.msg = result
		}
		//  End %w Simulation

	} else {
		//  No %w or wArg is nil: Format directly (original logic)
		result, fmtErr := FmtErrorCheck(format, args...)
		if fmtErr != nil {
			err.msg = fmt.Sprintf("errors.Newf: formatting error for format %q: %v", format, fmtErr)
			err.cause = nil
			err.formatWrapped = false
		} else {
			err.msg = result
			err.formatWrapped = false // Ensure false if no %w was involved
		}
	}
	//  End: Processing Valid Format String

	return err
}

// Errorf is an alias for Newf, providing a familiar interface compatible with
// fmt.Errorf. It creates a formatted error without capturing a stack trace.
// See Newf for full details on formatting, including %w support for error wrapping.
//
// Example:
//
//	err := errors.Errorf("failed: %w", errors.New("cause"))
//	// err.Error() == "failed: cause"
func Errorf(format string, args ...interface{}) *Error {
	return Newf(format, args...)
}

// FmtErrorCheck safely formats a string using fmt.Sprintf, catching panics.
// Returns the formatted string and any error encountered.
// Internal use by Newf to validate format strings.
// Example:
//
//	result, err := FmtErrorCheck("value: %s", "test")
func FmtErrorCheck(format string, args ...interface{}) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic during formatting: %v", r)
			}
		}
	}()
	result = fmt.Sprintf(format, args...)
	return result, nil
}

// Std creates a standard error using errors.New for compatibility.
// Does not capture stack traces or add context.
// Example:
//
//	err := errors.Std("simple error")
func Std(text string) error {
	return errors.New(text)
}

// Stdf creates a formatted standard error using fmt.Errorf for compatibility.
// Supports %w for wrapping; does not capture stack traces.
// Example:
//
//	err := errors.Stdf("failed: %w", cause)
func Stdf(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}

// Trace creates an error with the given message and captures a stack trace.
// Use when debugging context is needed; for performance, prefer New().
// Example:
//
//	err := errors.Trace("operation failed")
func Trace(text string) *Error {
	e := New(text)
	return e.WithStack()
}

// Tracef creates a formatted error with a stack trace.
// Supports %w for wrapping errors.
// Example:
//
//	err := errors.Tracef("query %s failed: %w", query, cause)
func Tracef(format string, args ...interface{}) *Error {
	e := Newf(format, args...)
	return e.WithStack()
}

// As attempts to assign the error or one in its chain to the target interface.
// Supports *Error and standard error types, traversing the cause chain.
// Returns true if successful.
// Example:
//
//	var target *Error
//	if errors.As(err, &target) {
//	  fmt.Println(target.Name())
//	}
func (e *Error) As(target interface{}) bool {
	if e == nil {
		return false
	}
	// Handle *Error target.
	if targetPtr, ok := target.(*Error); ok {
		current := e
		for current != nil {
			if current.name != "" {
				*targetPtr = *current
				return true
			}
			if next, ok := current.cause.(*Error); ok {
				current = next
			} else if current.cause != nil {
				return errors.As(current.cause, target)
			} else {
				return false
			}
		}
		return false
	}
	// Handle *error target.
	if targetErr, ok := target.(*error); ok {
		innermost := error(e)
		current := error(e)
		for current != nil {
			if err, ok := current.(*Error); ok && err.cause != nil {
				current = err.cause
				innermost = current
			} else {
				break
			}
		}
		*targetErr = innermost
		return true
	}
	// Delegate to cause for other types.
	if e.cause != nil {
		return errors.As(e.cause, target)
	}
	return false
}

// Callback sets a function to be called when Error() is invoked.
// Useful for logging or side effects on error access.
// Example:
//
//	err := errors.New("test").Callback(func() { log.Println("error accessed") })
func (e *Error) Callback(fn func()) *Error {
	e.callback = fn
	return e
}

// Category returns the error’s category, if set.
// Example:
//
//	if err.Category() == "network" {
//	  handleNetworkError(err)
//	}
func (e *Error) Category() string {
	return e.category
}

// Code returns the error’s HTTP-like status code, if set.
// Returns 0 if no code is set.
// Example:
//
//	if err.Code() == 404 {
//	  renderNotFound()
//	}
func (e *Error) Code() int {
	return int(e.code)
}

// Context returns the error’s context as a map, merging smallContext and map-based context.
// Thread-safe; lazily initializes the map if needed.
// Example:
//
//	ctx := err.Context()
//	if userID, ok := ctx["user_id"]; ok {
//	  fmt.Println(userID)
//	}
func (e *Error) Context() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.smallCount > 0 && e.context == nil {
		e.context = make(map[string]interface{}, e.smallCount)
		for i := int32(0); i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	return e.context
}

// Copy creates a deep copy of the error, preserving all fields except stack freshness.
// The new error can be modified independently.
// Example:
//
//	newErr := err.Copy().With("new_key", "value")
func (e *Error) Copy() *Error {
	if e == emptyError {
		return &Error{
			smallContext: [contextSize]contextItem{},
		}
	}

	newErr := newError()

	newErr.msg = e.msg
	newErr.name = e.name
	newErr.template = e.template
	newErr.cause = e.cause
	newErr.code = e.code
	newErr.category = e.category
	newErr.count = e.count

	if e.smallCount > 0 {
		newErr.smallCount = e.smallCount
		for i := int32(0); i < e.smallCount; i++ {
			newErr.smallContext[i] = e.smallContext[i]
		}
	} else if e.context != nil {
		newErr.context = make(map[string]interface{}, len(e.context))
		for k, v := range e.context {
			newErr.context[k] = v
		}
	}

	if e.stack != nil && len(e.stack) > 0 {
		if newErr.stack == nil {
			newErr.stack = stackPool.Get().([]uintptr)
		}
		newErr.stack = append(newErr.stack[:0], e.stack...)
	}

	return newErr
}

// Count returns the number of times the error has been incremented.
// Useful for tracking error frequency.
// Example:
//
//	fmt.Printf("Error occurred %d times", err.Count())
func (e *Error) Count() uint64 {
	return e.count
}

// Err returns the error as an error interface.
// Useful for type assertions or interface compatibility.
// Example:
//
//	var stdErr error = err.Err()
func (e *Error) Err() error {
	return e
}

// Error returns the string representation of the error.
// If the error was created using Newf/Errorf with the %w verb, it returns the
// pre-formatted string compatible with fmt.Errorf.
// Otherwise, it combines the message, template, or name with the cause's error
// string, separated by ": ". Invokes any set callback.
func (e *Error) Error() string {
	if e.callback != nil {
		e.callback()
	}

	// If created by Newf/Errorf with %w, msg already contains the final string.
	if e.formatWrapped {
		return e.msg // Return the pre-formatted fmt.Errorf-compatible string
	}

	//  Original logic for errors not created via Newf("%w", ...)
	//  or errors created via New/Named and then Wrap() called.
	var buf strings.Builder

	// Append primary message part (msg, template, or name)
	if e.msg != "" {
		buf.WriteString(e.msg)
	} else if e.template != "" {
		buf.WriteString(e.template)
	} else if e.name != "" {
		buf.WriteString(e.name)
	}

	// Append cause if it exists (only relevant if not formatWrapped)
	if e.cause != nil {
		if buf.Len() > 0 {
			// Add separator only if there was a prefix message/name/template
			buf.WriteString(": ")
		}
		buf.WriteString(e.cause.Error())
	} else if buf.Len() == 0 {
		// Handle case where msg/template/name are empty AND cause is nil
		// Could return a specific string like "[empty error]" or just ""
		return "" // Return empty string for a truly empty error
	}

	return buf.String()
}

// FastStack returns a lightweight stack trace with file and line numbers only.
// Omits function names for performance; skips internal frames if configured.
// Returns nil if no stack trace exists.
// Example:
//
//	for _, frame := range err.FastStack() {
//	  fmt.Println(frame) // e.g., "main.go:42"
//	}
func (e *Error) FastStack() []string {
	if e.stack == nil {
		return nil
	}
	configMu.RLock()
	filter := currentConfig.filterInternal
	configMu.RUnlock()

	pcs := e.stack
	frames := make([]string, 0, len(pcs))
	for _, pc := range pcs {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			frames = append(frames, "unknown")
			continue
		}
		file, line := fn.FileLine(pc)
		if filter && isInternalFrame(runtime.Frame{File: file, Function: fn.Name()}) {
			continue
		}
		frames = append(frames, fmt.Sprintf("%s:%d", file, line))
	}
	return frames
}

// Find searches the error chain for the first error where pred returns true.
// Returns nil if no match is found or if pred is nil.
// Example:
//
//	err := err.Find(func(e error) bool { return strings.Contains(e.Error(), "timeout") })
func (e *Error) Find(pred func(error) bool) error {
	if e == nil || pred == nil {
		return nil
	}
	return Find(e, pred)
}

// Format returns a detailed, human-readable string representation of the error,
// including message, code, context, stack, and cause.
// Recursive for causes that are also *Error.
// Example:
//
//	fmt.Println(err.Format())
//	// Output:
//	// Error: failed: cause
//	// Code: 500
//	// Context:
//	//   key: value
//	// Stack:
//	//   1. main.main main.go:42
func (e *Error) Format() string {
	var sb strings.Builder

	// Error message.
	sb.WriteString("Error: " + e.Error() + "\n")

	// Metadata.
	if e.code != 0 {
		sb.WriteString(fmt.Sprintf("Code: %d\n", e.code))
	}

	// Context.
	if ctx := e.contextAtThisLevel(); len(ctx) > 0 {
		sb.WriteString("Context:\n")
		for k, v := range ctx {
			sb.WriteString(fmt.Sprintf("\t%s: %v\n", k, v))
		}
	}

	// Stack trace.
	if e.stack != nil {
		sb.WriteString("Stack:\n")
		for i, frame := range e.Stack() {
			sb.WriteString(fmt.Sprintf("\t%d. %s\n", i+1, frame))
		}
	}

	// Cause.
	if e.cause != nil {
		sb.WriteString("Caused by: ")
		if causeErr, ok := e.cause.(*Error); ok {
			sb.WriteString(causeErr.Format())
		} else {
			sb.WriteString("Error: " + e.cause.Error() + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// contextAtThisLevel returns context specific to this error, excluding inherited context.
// Internal use by Format to isolate context per error level.
func (e *Error) contextAtThisLevel() map[string]interface{} {
	if e.context == nil && e.smallCount == 0 {
		return nil
	}

	ctx := make(map[string]interface{})
	// Add smallContext items.
	for i := 0; i < int(e.smallCount); i++ {
		ctx[e.smallContext[i].key] = e.smallContext[i].value
	}
	// Add map context items.
	if e.context != nil {
		for k, v := range e.context {
			ctx[k] = v
		}
	}
	return ctx
}

// Free resets the error and returns it to the pool if pooling is enabled.
// Safe to call multiple times; no-op if pooling is disabled.
// Call after use to prevent memory leaks when autoFree is false.
// Example:
//
//	defer err.Free()
func (e *Error) Free() {
	if currentConfig.disablePooling {
		return
	}

	e.Reset()

	if e.stack != nil {
		stackPool.Put(e.stack[:cap(e.stack)])
		e.stack = nil
	}
	errorPool.Put(e)
}

// Has checks if the error contains meaningful content (message, template, name, or cause).
// Returns false for nil or empty errors.
// Example:
//
//	if !err.Has() {
//	  return nil
//	}
func (e *Error) Has() bool {
	return e != nil && (e.msg != "" || e.template != "" || e.name != "" || e.cause != nil)
}

// HasContextKey checks if the specified key exists in the error’s context.
// Thread-safe; checks both smallContext and map-based context.
// Example:
//
//	if err.HasContextKey("user_id") {
//	  fmt.Println(err.Context()["user_id"])
//	}
func (e *Error) HasContextKey(key string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.smallCount > 0 {
		for i := int32(0); i < e.smallCount; i++ {
			if e.smallContext[i].key == key {
				return true
			}
		}
	}
	if e.context != nil {
		_, exists := e.context[key]
		return exists
	}
	return false
}

// Increment atomically increases the error’s count by 1 and returns the error.
// Useful for tracking repeated occurrences.
// Example:
//
//	err := err.Increment()
func (e *Error) Increment() *Error {
	atomic.AddUint64(&e.count, 1)
	return e
}

// Is checks if the error matches the target by pointer, name, or cause chain.
// Compatible with errors.Is; also matches by string for standard errors.
// Returns true if the error or its cause matches the target.
// Example:
//
//	if errors.Is(err, errors.New("target")) {
//	  handleTargetError()
//	}
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return e == target
	}
	if e == target {
		return true
	}
	if e.name != "" {
		if te, ok := target.(*Error); ok && te.name != "" && e.name == te.name {
			return true
		}
	}
	// Match standard errors by string.
	if stdErr, ok := target.(error); ok && e.Error() == stdErr.Error() {
		return true
	}
	if e.cause != nil {
		return errors.Is(e.cause, target)
	}
	return false
}

// IsEmpty checks if the error lacks meaningful content (no message, name, template, or cause).
// Returns true for nil or fully empty errors.
// Example:
//
//	if err.IsEmpty() {
//	  return nil
//	}
func (e *Error) IsEmpty() bool {
	if e == nil {
		return true
	}
	return e.msg == "" && e.template == "" && e.name == "" && e.cause == nil
}

// IsNull checks if the error is nil, empty, or contains only SQL NULL values in its context or cause.
// Useful for handling database-related errors.
// Example:
//
//	if err.IsNull() {
//	  return nil
//	}
func (e *Error) IsNull() bool {
	if e == nil || e == emptyError {
		return true
	}
	// If no context or cause, and no content, it’s not null.
	if e.smallCount == 0 && e.context == nil && e.cause == nil {
		return false
	}

	// Check cause first.
	if e.cause != nil {
		var isNull bool
		if ce, ok := e.cause.(*Error); ok {
			isNull = ce.IsNull()
		} else {
			isNull = sqlNull(e.cause)
		}
		if isNull {
			return true
		}
	}

	// Check small context.
	if e.smallCount > 0 {
		allNull := true
		for i := 0; i < int(e.smallCount); i++ {
			isNull := sqlNull(e.smallContext[i].value)
			if !isNull {
				allNull = false
				break
			}
		}
		if !allNull {
			return false
		}
	}

	// Check regular context.
	if e.context != nil {
		allNull := true
		for _, v := range e.context {
			isNull := sqlNull(v)
			if !isNull {
				allNull = false
				break
			}
		}
		if !allNull {
			return false
		}
	}

	// Null if context exists and is all null.
	return e.smallCount > 0 || e.context != nil
}

// jsonBufferPool manages reusable buffers for JSON marshaling to reduce allocations.
var (
	jsonBufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, bufferSize))
		},
	}
)

// MarshalJSON serializes the error to JSON, including name, message, context, cause, stack, and code.
// Causes are recursively serialized if they implement json.Marshaler or are *Error.
// Example:
//
//	data, _ := json.Marshal(err)
//	fmt.Println(string(data))
func (e *Error) MarshalJSON() ([]byte, error) {
	// Get buffer from pool.
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	defer jsonBufferPool.Put(buf)
	buf.Reset()

	// Create new encoder.
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	// Prepare JSON structure.
	je := struct {
		Name    string                 `json:"name,omitempty"`
		Message string                 `json:"message,omitempty"`
		Context map[string]interface{} `json:"context,omitempty"`
		Cause   interface{}            `json:"cause,omitempty"`
		Stack   []string               `json:"stack,omitempty"`
		Code    int                    `json:"code,omitempty"`
	}{
		Name:    e.name,
		Message: e.msg,
		Code:    e.Code(),
	}

	// Add context.
	if ctx := e.Context(); len(ctx) > 0 {
		je.Context = ctx
	}

	// Add stack.
	if e.stack != nil {
		je.Stack = e.Stack()
	}

	// Add cause.
	if e.cause != nil {
		switch c := e.cause.(type) {
		case *Error:
			je.Cause = c
		case json.Marshaler:
			je.Cause = c
		default:
			je.Cause = c.Error()
		}
	}

	// Encode JSON.
	if err := enc.Encode(je); err != nil {
		return nil, err
	}

	// Remove trailing newline.
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// Msgf sets the error’s message using a formatted string and returns the error.
// Overwrites any existing message.
// Example:
//
//	err := err.Msgf("user %s not found", username)
func (e *Error) Msgf(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

// Name returns the error’s name, if set.
// Example:
//
//	if err.Name() == "AuthError" {
//	  handleAuthError()
//	}
func (e *Error) Name() string {
	return e.name
}

// Reset clears all fields of the error, preparing it for reuse in the pool.
// Internal use by Free; does not release stack to stackPool.
// Example:
//
//	err.Reset() // Clear all fields.
func (e *Error) Reset() {
	e.msg = ""
	e.name = ""
	e.template = ""
	e.category = ""
	e.code = 0
	e.count = 0
	e.cause = nil
	e.callback = nil
	e.formatWrapped = false

	if e.context != nil {
		for k := range e.context {
			delete(e.context, k)
		}
	}
	e.smallCount = 0

	if e.stack != nil {
		e.stack = e.stack[:0]
	}
}

// Stack returns a detailed stack trace with function names, files, and line numbers.
// Filters internal frames if configured; returns nil if no stack exists.
// Example:
//
//	for _, frame := range err.Stack() {
//	  fmt.Println(frame) // e.g., "main.main main.go:42"
//	}
func (e *Error) Stack() []string {
	if e.stack == nil {
		return nil
	}

	frames := runtime.CallersFrames(e.stack)
	var trace []string
	for {
		frame, more := frames.Next()
		if frame == (runtime.Frame{}) {
			break
		}

		if currentConfig.filterInternal && isInternalFrame(frame) {
			continue
		}

		trace = append(trace, fmt.Sprintf("%s %s:%d",
			frame.Function,
			frame.File,
			frame.Line))

		if !more {
			break
		}
	}
	return trace
}

// Trace ensures the error has a stack trace, capturing it if absent.
// Returns the error for chaining.
// Example:
//
//	err := errors.New("failed").Trace()
func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(2)
	}
	return e
}

// Transform applies transformations to a copy of the error and returns the new error.
// The original error is unchanged; nil-safe.
// Example:
//
//	newErr := err.Transform(func(e *Error) { e.With("key", "value") })
func (e *Error) Transform(fn func(*Error)) *Error {
	if e == nil || fn == nil {
		return e
	}
	newErr := e.Copy()
	fn(newErr)
	return newErr
}

// Unwrap returns the underlying cause of the error, if any.
// Compatible with errors.Unwrap for chain traversal.
// Example:
//
//	cause := errors.Unwrap(err)
func (e *Error) Unwrap() error {
	return e.cause
}

// UnwrapAll returns a slice of all errors in the chain, starting with this error.
// Each error is isolated to prevent modifications affecting others.
// Example:
//
//	chain := err.UnwrapAll()
//	for _, e := range chain {
//	  fmt.Println(e.Error())
//	}
func (e *Error) UnwrapAll() []error {
	if e == nil {
		return nil
	}
	var chain []error
	current := error(e)
	for current != nil {
		if err, ok := current.(*Error); ok {
			isolated := newError()
			isolated.msg = err.msg
			isolated.name = err.name
			isolated.template = err.template
			isolated.code = err.code
			isolated.category = err.category
			if err.smallCount > 0 {
				isolated.smallCount = err.smallCount
				for i := int32(0); i < err.smallCount; i++ {
					isolated.smallContext[i] = err.smallContext[i]
				}
			}
			if err.context != nil {
				isolated.context = make(map[string]interface{}, len(err.context))
				for k, v := range err.context {
					isolated.context[k] = v
				}
			}
			if err.stack != nil {
				isolated.stack = append([]uintptr(nil), err.stack...)
			}
			chain = append(chain, isolated)
		} else {
			chain = append(chain, current)
		}
		if unwrapper, ok := current.(interface{ Unwrap() error }); ok {
			current = unwrapper.Unwrap()
		} else {
			break
		}
	}
	return chain
}

// Walk traverses the error chain, applying fn to each error.
// Stops if fn is nil or the chain ends.
// Example:
//
//	err.Walk(func(e error) { fmt.Println(e.Error()) })
func (e *Error) Walk(fn func(error)) {
	if e == nil || fn == nil {
		return
	}
	current := error(e)
	for current != nil {
		fn(current)
		if unwrappable, ok := current.(interface{ Unwrap() error }); ok {
			current = unwrappable.Unwrap()
		} else {
			break
		}
	}
}

// With adds key-value pairs to the error's context and returns the error.
// Uses a fixed-size array (smallContext) for up to contextSize items, then switches
// to a map. Thread-safe. Accepts variadic key-value pairs.
// Example:
//
//	err := err.With("key1", value1, "key2", value2)
func (e *Error) With(keyValues ...interface{}) *Error {
	if len(keyValues) == 0 {
		return e
	}

	// Validate that we have an even number of arguments
	if len(keyValues)%2 != 0 {
		keyValues = append(keyValues, "(MISSING)")
	}

	// Fast path for small context when we can add all pairs to smallContext
	if e.smallCount < contextSize && e.context == nil {
		remainingSlots := contextSize - int(e.smallCount)
		if len(keyValues)/2 <= remainingSlots {
			e.mu.Lock()
			// Recheck conditions after acquiring lock
			if e.smallCount < contextSize && e.context == nil {
				for i := 0; i < len(keyValues); i += 2 {
					key, ok := keyValues[i].(string)
					if !ok {
						key = fmt.Sprintf("%v", keyValues[i])
					}
					e.smallContext[e.smallCount] = contextItem{key, keyValues[i+1]}
					e.smallCount++
				}
				e.mu.Unlock()
				return e
			}
			e.mu.Unlock()
		}
	}

	// Slow path - either we have too many pairs or already using map context
	e.mu.Lock()
	defer e.mu.Unlock()

	// Initialize map context if needed
	if e.context == nil {
		e.context = make(map[string]interface{}, max(currentConfig.contextSize, len(keyValues)/2+int(e.smallCount)))
		// Migrate existing smallContext items
		for i := int32(0); i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
		// Reset smallCount since we've moved to map context
		e.smallCount = 0
	}

	// Add all pairs to map context
	for i := 0; i < len(keyValues); i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyValues[i])
		}
		e.context[key] = keyValues[i+1]
	}

	return e
}

// Helper function to get maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// WithCategory sets the error’s category and returns the error.
// Example:
//
//	err := err.WithCategory("validation")
func (e *Error) WithCategory(category ErrorCategory) *Error {
	e.category = string(category)
	return e
}

// WithCode sets an HTTP-like status code and returns the error.
// Example:
//
//	err := err.WithCode(400)
func (e *Error) WithCode(code int) *Error {
	e.code = int32(code)
	return e
}

// WithName sets the error’s name and returns the error.
// Example:
//
//	err := err.WithName("AuthError")
func (e *Error) WithName(name string) *Error {
	e.name = name
	return e
}

// WithRetryable marks the error as retryable in its context and returns the error.
// Example:
//
//	err := err.WithRetryable()
func (e *Error) WithRetryable() *Error {
	return e.With(ctxRetry, true)
}

// WithStack captures a stack trace if none exists and returns the error.
// Skips one frame (caller of WithStack).
// Example:
//
//	err := errors.New("failed").WithStack()
func (e *Error) WithStack() *Error {
	if e.stack == nil {
		e.stack = captureStack(1)
	}
	return e
}

// WithTemplate sets a message template and returns the error.
// Used as a fallback if the message is empty.
// Example:
//
//	err := err.WithTemplate("operation failed")
func (e *Error) WithTemplate(template string) *Error {
	e.template = template
	return e
}

// WithTimeout marks the error as a timeout error in its context and returns the error.
// Example:
//
//	err := err.WithTimeout()
func (e *Error) WithTimeout() *Error {
	return e.With(ctxTimeout, true)
}

// Wrap associates a cause error with this error, creating a chain.
// Returns the error unchanged if cause is nil.
// Example:
//
//	err := errors.New("failed").Wrap(errors.New("cause"))
func (e *Error) Wrap(cause error) *Error {
	if cause == nil {
		return e
	}
	e.cause = cause
	return e
}

// Wrapf wraps a cause error with formatted message and returns the error.
// If cause is nil, returns the error unchanged.
// Example:
//
//	err := errors.New("base").Wrapf(io.EOF, "read failed: %s", "file.txt")
func (e *Error) Wrapf(cause error, format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	if cause != nil {
		e.cause = cause
	}
	return e
}

// WrapNotNil wraps a cause error only if it is non-nil and returns the error.
// Example:
//
//	err := err.WrapNotNil(maybeError)
func (e *Error) WrapNotNil(cause error) *Error {
	if cause != nil {
		e.cause = cause
	}
	return e
}

// WarmPool pre-populates the error pool with count instances.
// Improves performance by reducing initial allocations.
// No-op if pooling is disabled.
// Example:
//
//	errors.WarmPool(1000)
func WarmPool(count int) {
	if currentConfig.disablePooling {
		return
	}
	for i := 0; i < count; i++ {
		e := &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
		errorPool.Put(e)
		stackPool.Put(make([]uintptr, 0, currentConfig.stackDepth))
	}
}

// WarmStackPool pre-populates the stack pool with count slices.
// Improves performance for stack-intensive operations.
// No-op if pooling is disabled.
// Example:
//
//	errors.WarmStackPool(500)
func WarmStackPool(count int) {
	if currentConfig.disablePooling {
		return
	}
	for i := 0; i < count; i++ {
		stackPool.Put(make([]uintptr, 0, currentConfig.stackDepth))
	}
}
