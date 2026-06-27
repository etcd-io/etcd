package errors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// As wraps errors.As, using custom type assertion for *Error types.
// Falls back to standard errors.As for non-*Error types.
// Returns false if either err or target is nil.
func As(err error, target interface{}) bool {
	if err == nil || target == nil {
		return false
	}

	// First try our custom *Error handling
	if e, ok := err.(*Error); ok {
		return e.As(target)
	}

	// Fall back to standard errors.As
	return errors.As(err, target)
}

// Code returns the status code of an error, if it is an *Error.
// Returns 500 as a default for non-*Error types to indicate an internal error.
func Code(err error) int {
	if e, ok := err.(*Error); ok {
		return e.Code()
	}
	return DefaultCode
}

// Context extracts the context map from an error, if it is an *Error.
// Returns nil for non-*Error types or if no context is present.
func Context(err error) map[string]interface{} {
	if e, ok := err.(*Error); ok {
		return e.Context()
	}
	return nil
}

// Convert transforms any error into an *Error, preserving its message and wrapping it if needed.
// Returns nil if the input is nil; returns the original if already an *Error.
// Uses multiple strategies: direct assertion, errors.As, manual unwrapping, and fallback creation.
func Convert(err error) *Error {
	if err == nil {
		return nil
	}

	// First try direct type assertion (fast path)
	if e, ok := err.(*Error); ok {
		return e
	}

	// Try using errors.As (more flexible)
	var e *Error
	if errors.As(err, &e) {
		return e
	}

	// Manual unwrapping as fallback
	visited := make(map[error]bool)
	for unwrapped := err; unwrapped != nil; {
		if visited[unwrapped] {
			break // Cycle detected
		}
		visited[unwrapped] = true
		if e, ok := unwrapped.(*Error); ok {
			return e
		}
		unwrapped = errors.Unwrap(unwrapped)
	}

	// Final fallback: create new error with original message and wrap it
	return New(err.Error()).Wrap(err)
}

// Count returns the occurrence count of an error, if it is an *Error.
// Returns 0 for non-*Error types.
func Count(err error) uint64 {
	if e, ok := err.(*Error); ok {
		return e.Count()
	}
	return 0
}

// Find searches the error chain for the first error matching pred.
// Returns nil if no match is found or pred is nil; traverses both Unwrap() and Cause() chains.
func Find(err error, pred func(error) bool) error {
	for current := err; current != nil; {
		if pred(current) {
			return current
		}

		// Attempt to unwrap using Unwrap() or Cause()
		switch v := current.(type) {
		case interface{ Unwrap() error }:
			current = v.Unwrap()
		case interface{ Cause() error }:
			current = v.Cause()
		default:
			return nil
		}
	}
	return nil
}

// From transforms any error into an *Error, preserving its message and wrapping it if needed.
// Alias of Convert; returns nil if input is nil, original if already an *Error.
func From(err error) *Error {
	return Convert(err)
}

// FromContext creates an *Error from a context and an existing error.
// Enhances the error with context info: timeout status, deadline, or cancellation.
// Returns nil if input error is nil; does not store context values directly.
func FromContext(ctx context.Context, err error) *Error {
	if err == nil {
		return nil
	}

	e := New(err.Error())

	// Handle context errors
	switch ctx.Err() {
	case context.DeadlineExceeded:
		e.WithTimeout()
		if deadline, ok := ctx.Deadline(); ok {
			e.With("deadline", deadline.Format(time.RFC3339))
		}
	case context.Canceled:
		e.With("cancelled", true)
	}

	return e
}

// Category returns the category of an error, if it is an *Error.
// Returns an empty string for non-*Error types or unset categories.
func Category(err error) string {
	if e, ok := err.(*Error); ok {
		return e.category
	}
	return ""
}

// Has checks if an error contains meaningful content.
// Returns true for non-nil standard errors or *Error with content (msg, name, template, or cause).
func Has(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Has()
	}
	return err != nil
}

// HasContextKey checks if the error's context contains the specified key.
// Returns false for non-*Error types or if the key is not present in the context.
func HasContextKey(err error, key string) bool {
	if e, ok := err.(*Error); ok {
		ctx := e.Context()
		if ctx != nil {
			_, exists := ctx[key]
			return exists
		}
	}
	return false
}

// Is wraps errors.Is, using custom matching for *Error types.
// Falls back to standard errors.Is for non-*Error types; returns true if err equals target.
func Is(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}

	if e, ok := err.(*Error); ok {
		return e.Is(target)
	}

	// Use standard errors.Is for non-Error types
	return errors.Is(err, target)
}

// IsError checks if an error is an instance of *Error.
// Returns true only for this package's custom error type; false for nil or other types.
func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

// IsEmpty checks if an error has no meaningful content.
// Returns true for nil errors, empty *Error instances, or standard errors with whitespace-only messages.
func IsEmpty(err error) bool {
	if err == nil {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.IsEmpty()
	}
	return strings.TrimSpace(err.Error()) == ""
}

// IsNull checks if an error is nil or represents a NULL value.
// Delegates to *Error’s IsNull for custom errors; uses sqlNull for others.
func IsNull(err error) bool {
	if err == nil {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.IsNull()
	}
	return sqlNull(err)
}

// IsRetryable checks if an error is retryable.
// For *Error, checks context for retry flag; for others, looks for "retry" or timeout in message.
// Returns false for nil errors; thread-safe for *Error types.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		e.mu.RLock()
		defer e.mu.RUnlock()
		// Check smallContext directly if context map isn’t populated
		for i := int32(0); i < e.smallCount; i++ {
			if e.smallContext[i].key == ctxRetry {
				if val, ok := e.smallContext[i].value.(bool); ok {
					return val
				}
			}
		}
		// Check regular context
		if e.context != nil {
			if val, ok := e.context[ctxRetry].(bool); ok {
				return val
			}
		}
		// Check cause recursively
		if e.cause != nil {
			return IsRetryable(e.cause)
		}
	}
	lowerMsg := strings.ToLower(err.Error())
	return IsTimeout(err) || strings.Contains(lowerMsg, "retry")
}

// IsTimeout checks if an error indicates a timeout.
// For *Error, checks context for timeout flag; for others, looks for "timeout" in message.
// Returns false for nil errors.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		if val, ok := e.Context()[ctxTimeout].(bool); ok {
			return val
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

// Merge combines multiple errors into a single *Error.
// Aggregates messages with "; " separator, merges contexts and stacks; returns nil if no errors provided.
func Merge(errs ...error) *Error {
	if len(errs) == 0 {
		return nil
	}
	var messages []string
	combined := New("")
	for _, err := range errs {
		if err == nil {
			continue
		}
		messages = append(messages, err.Error())
		if e, ok := err.(*Error); ok {
			if e.stack != nil && combined.stack == nil {
				combined.WithStack() // Capture stack from first *Error with stack
			}
			if ctx := e.Context(); ctx != nil {
				for k, v := range ctx {
					combined.With(k, v)
				}
			}
			if e.cause != nil {
				combined.Wrap(e.cause)
			}
		} else {
			combined.Wrap(err)
		}
	}
	if len(messages) > 0 {
		combined.msg = strings.Join(messages, "; ")
	}
	return combined
}

// Name returns the name of an error, if it is an *Error.
// Returns an empty string for non-*Error types or unset names.
func Name(err error) string {
	if e, ok := err.(*Error); ok {
		return e.name
	}
	return ""
}

// UnwrapAll returns a slice of all errors in the chain, including the root error.
// Traverses both Unwrap() and Cause() chains; returns nil if err is nil.
func UnwrapAll(err error) []error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e.UnwrapAll()
	}
	var result []error
	Walk(err, func(e error) {
		result = append(result, e)
	})
	return result
}

// Stack extracts the stack trace from an error, if it is an *Error.
// Returns nil for non-*Error types or if no stack is present.
func Stack(err error) []string {
	if e, ok := err.(*Error); ok {
		return e.Stack()
	}
	return nil
}

// Transform applies transformations to an error, returning a new *Error.
// Creates a new *Error from non-*Error types before applying fn; returns nil if err is nil.
func Transform(err error, fn func(*Error)) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		newErr := e.Copy()
		fn(newErr)
		return newErr
	}
	// If not an *Error, create a new one and transform it
	newErr := New(err.Error())
	fn(newErr)
	return newErr
}

// Unwrap returns the underlying cause of an error, if it implements Unwrap.
// For *Error, returns cause; for others, returns the error itself; nil if err is nil.
func Unwrap(err error) error {
	for current := err; current != nil; {
		if e, ok := current.(*Error); ok {
			if e.cause == nil {
				return current
			}
			current = e.cause
		} else {
			return current
		}
	}
	return nil
}

// Walk traverses the error chain, applying fn to each error.
// Supports both Unwrap() and Cause() interfaces; stops at nil or non-unwrappable errors.
func Walk(err error, fn func(error)) {
	for current := err; current != nil; {
		fn(current)

		// Attempt to unwrap using Unwrap() or Cause()
		switch v := current.(type) {
		case interface{ Unwrap() error }:
			current = v.Unwrap()
		case interface{ Cause() error }:
			current = v.Cause()
		default:
			return
		}
	}
}

// With adds a key-value pair to an error's context, if it is an *Error.
// Returns the original error unchanged if not an *Error; no-op for non-*Error types.
func With(err error, key string, value interface{}) error {
	if e, ok := err.(*Error); ok {
		return e.With(key, value)
	}
	return err
}

// WithStack converts any error to an *Error and captures a stack trace.
// Returns nil if input is nil; adds stack to existing *Error or wraps non-*Error types.
func WithStack(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e.WithStack()
	}
	return New(err.Error()).WithStack().Wrap(err)
}

// Wrap creates a new *Error that wraps another error with additional context.
// Uses a copy of the provided wrapper *Error; returns nil if err is nil.
func Wrap(err error, wrapper *Error) *Error {
	if err == nil {
		return nil
	}
	if wrapper == nil {
		wrapper = newError()
	}
	newErr := wrapper.Copy()
	newErr.cause = err
	return newErr
}

// Wrapf creates a new formatted *Error that wraps another error.
// Formats the message and sets the cause; returns nil if err is nil.
func Wrapf(err error, format string, args ...interface{}) *Error {
	if err == nil {
		return nil
	}
	e := newError()
	e.msg = fmt.Sprintf(format, args...)
	e.cause = err
	return e
}

// Err creates a new Error with the given message and wraps the provided error as its cause.
func Err(msg string, err error) *Error {
	return New(msg).Wrap(err)
}
