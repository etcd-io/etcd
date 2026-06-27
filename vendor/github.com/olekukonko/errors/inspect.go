// File: inspect.go
// Updated to support both error and *Error with delegation for cleaner *Error handling

package errors

import (
	stderrs "errors"
	"fmt"
	"strings"
	"time"
)

// Inspect provides detailed examination of an error, handling both single errors and MultiError
func Inspect(err error) {
	if err == nil {
		fmt.Println("No error occurred")
		return
	}

	fmt.Printf("\n=== Error Inspection ===\n")
	fmt.Printf("Top-level error: %v\n", err)
	fmt.Printf("Top-level error type: %T\n", err)

	// Handle *Error directly
	if e, ok := err.(*Error); ok {
		InspectError(e)
		return
	}

	// Handle MultiError
	if multi, ok := err.(*MultiError); ok {
		allErrors := multi.Errors()
		fmt.Printf("\nContains %d errors:\n", len(allErrors))
		for i, e := range allErrors {
			fmt.Printf("\n--- Error %d ---\n", i+1)
			inspectSingleError(e)
		}
	} else {
		// Inspect single error if not MultiError or *Error
		fmt.Println("\n--- Details ---")
		inspectSingleError(err)
	}

	// Additional diagnostics
	fmt.Println("\n--- Diagnostics ---")
	if IsRetryable(err) {
		fmt.Println("- Error chain contains retryable errors")
	}
	if IsTimeout(err) {
		fmt.Println("- Error chain contains timeout errors")
	}
	if code := getErrorCode(err); code != 0 {
		fmt.Printf("- Highest priority error code: %d\n", code)
	}
	fmt.Printf("========================\n\n")
}

// InspectError provides detailed inspection of a specific *Error instance
func InspectError(err *Error) {
	if err == nil {
		fmt.Println("No error occurred")
		return
	}

	fmt.Printf("\n=== Error Inspection (*Error) ===\n")
	fmt.Printf("Top-level error: %v\n", err)
	fmt.Printf("Top-level error type: %T\n", err)

	fmt.Println("\n--- Details ---")
	inspectSingleError(err) // Delegate to handle unwrapping and details

	// Additional diagnostics specific to *Error
	fmt.Println("\n--- Diagnostics ---")
	if IsRetryable(err) {
		fmt.Println("- Error is retryable")
	}
	if IsTimeout(err) {
		fmt.Println("- Error chain contains timeout errors")
	}
	if code := err.Code(); code != 0 {
		fmt.Printf("- Error code: %d\n", code)
	}
	fmt.Printf("========================\n\n")
}

// inspectSingleError handles inspection of a single error (may be part of a chain)
func inspectSingleError(err error) {
	if err == nil {
		fmt.Println("  (nil error)")
		return
	}

	fmt.Printf("  Error: %v\n", err)
	fmt.Printf("  Type: %T\n", err)

	// Handle wrapped errors, including *Error type
	var currentErr error = err
	depth := 0
	for currentErr != nil {
		prefix := strings.Repeat("  ", depth+1)
		if depth > 0 {
			fmt.Printf("%sWrapped Cause (%T): %v\n", prefix, currentErr, currentErr)
		}

		// Check if it's our specific *Error type
		if e, ok := currentErr.(*Error); ok {
			if name := e.Name(); name != "" {
				fmt.Printf("%sName: %s\n", prefix, name)
			}
			if cat := e.Category(); cat != "" {
				fmt.Printf("%sCategory: %s\n", prefix, cat)
			}
			if code := e.Code(); code != 0 {
				fmt.Printf("%sCode: %d\n", prefix, code)
			}
			if ctx := e.Context(); len(ctx) > 0 {
				fmt.Printf("%sContext:\n", prefix)
				for k, v := range ctx {
					fmt.Printf("%s  %s: %v\n", prefix, k, v)
				}
			}
			if stack := e.Stack(); len(stack) > 0 {
				fmt.Printf("%sStack (Top 3):\n", prefix)
				limit := 3
				if len(stack) < limit {
					limit = len(stack)
				}
				for i := 0; i < limit; i++ {
					fmt.Printf("%s  %s\n", prefix, stack[i])
				}
				if len(stack) > limit {
					fmt.Printf("%s  ... (%d more frames)\n", prefix, len(stack)-limit)
				}
			}
		}

		// Unwrap using standard errors.Unwrap and handle *Error Unwrap
		var nextErr error
		// Prioritize *Error's Unwrap if available AND it returns non-nil
		if e, ok := currentErr.(*Error); ok {
			unwrapped := e.Unwrap()
			if unwrapped != nil {
				nextErr = unwrapped
			} else {
				// If *Error.Unwrap returns nil, fall back to standard unwrap
				// This handles cases where *Error might wrap a non-standard error
				// or where its internal cause is deliberately nil.
				nextErr = stderrs.Unwrap(currentErr)
			}
		} else {
			nextErr = stderrs.Unwrap(currentErr) // Fall back to standard unwrap for non-*Error types
		}

		// Prevent infinite loops if Unwrap returns the same error, or stop if no more unwrapping
		if nextErr == currentErr || nextErr == nil {
			break
		}
		currentErr = nextErr
		depth++
		if depth > 10 { // Safety break for very deep or potentially cyclic chains
			fmt.Printf("%s... (chain too deep or potential cycle)\n", strings.Repeat("  ", depth+1))
			break
		}
	}
}

// getErrorCode traverses the error chain to find the highest priority code.
// It uses errors.As to find the first *Error in the chain.
func getErrorCode(err error) int {
	var code int = 0 // Default code
	var target *Error
	if As(err, &target) { // Use the package's As helper
		if target != nil { // Add nil check for safety
			code = target.Code()
		}
	}
	// If the top-level error is *Error and has a code, it might take precedence.
	// This depends on desired logic. Let's keep it simple for now: first code found by As.
	if code == 0 { // Only check top-level if As didn't find one with a code
		if e, ok := err.(*Error); ok {
			code = e.Code()
		}
	}
	return code
}

// handleError demonstrates using Inspect with additional handling logic
func handleError(err error) {
	fmt.Println("\n=== Processing Failure ===")
	Inspect(err) // Use the primary Inspect function

	// Additional handling based on inspection
	code := getErrorCode(err) // Use the helper

	switch {
	case IsTimeout(err):
		fmt.Println("\nAction: Check connectivity or increase timeout")
	case code == 402: // Check code obtained via helper
		fmt.Println("\nAction: Payment processing failed - notify billing")
	default:
		fmt.Println("\nAction: Generic failure handling")
	}
}

// processOrder demonstrates Chain usage with Inspect
func processOrder() error {
	validateInput := func() error { return nil }
	processPayment := func() error { return stderrs.New("credit card declined") }
	sendNotification := func() error { fmt.Println("Notification sent."); return nil }
	logOrder := func() error { fmt.Println("Order logged."); return nil }

	chain := NewChain(ChainWithTimeout(2*time.Second)).
		Step(validateInput).Tag("validation").
		Step(processPayment).Tag("billing").Code(402).Retry(3, 100*time.Millisecond, WithRetryIf(IsRetryable)).
		Step(sendNotification).Optional().
		Step(logOrder)

	err := chain.Run()
	if err != nil {
		handleError(err) // Call the unified error handler
		return err       // Propagate the error if needed
	}
	fmt.Println("Order processed successfully!")
	return nil
}
