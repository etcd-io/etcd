// Package errors provides utilities for error handling, including a flexible retry mechanism.
package errors

import (
	"context"
	"math/rand"
	"time"
)

// BackoffStrategy defines the interface for calculating retry delays.
type BackoffStrategy interface {
	// Backoff returns the delay for a given attempt based on the base delay.
	Backoff(attempt int, baseDelay time.Duration) time.Duration
}

// ConstantBackoff provides a fixed delay for each retry attempt.
type ConstantBackoff struct{}

// Backoff returns the base delay regardless of the attempt number.
// Implements BackoffStrategy with a constant delay.
func (c ConstantBackoff) Backoff(_ int, baseDelay time.Duration) time.Duration {
	return baseDelay
}

// ExponentialBackoff provides an exponentially increasing delay for retry attempts.
type ExponentialBackoff struct{}

// Backoff returns a delay that doubles with each attempt, starting from the base delay.
// Uses bit shifting for efficient exponential growth (e.g., baseDelay * 2^(attempt-1)).
func (e ExponentialBackoff) Backoff(attempt int, baseDelay time.Duration) time.Duration {
	if attempt <= 1 {
		return baseDelay
	}
	return baseDelay * time.Duration(1<<uint(attempt-1))
}

// LinearBackoff provides a linearly increasing delay for retry attempts.
type LinearBackoff struct{}

// Backoff returns a delay that increases linearly with each attempt (e.g., baseDelay * attempt).
// Implements BackoffStrategy with linear progression.
func (l LinearBackoff) Backoff(attempt int, baseDelay time.Duration) time.Duration {
	return baseDelay * time.Duration(attempt)
}

// RetryOption configures a Retry instance.
// Defines a function type for setting retry parameters.
type RetryOption func(*Retry)

// Retry represents a retryable operation with configurable backoff and retry logic.
// Supports multiple attempts, delay strategies, jitter, and context-aware cancellation.
type Retry struct {
	maxAttempts int              // Maximum number of attempts (including initial try)
	delay       time.Duration    // Base delay for backoff calculations
	maxDelay    time.Duration    // Maximum delay cap to prevent excessive waits
	retryIf     func(error) bool // Condition to determine if retry should occur
	onRetry     func(int, error) // Callback executed after each failed attempt
	backoff     BackoffStrategy  // Strategy for calculating retry delays
	jitter      bool             // Whether to add random jitter to delays
	ctx         context.Context  // Context for cancellation and deadlines
}

// NewRetry creates a new Retry instance with the given options.
// Defaults: 3 attempts, 100ms base delay, 10s max delay, exponential backoff with jitter,
// and retrying on IsRetryable errors; ensures retryIf is never nil.
func NewRetry(options ...RetryOption) *Retry {
	r := &Retry{
		maxAttempts: 3,
		delay:       100 * time.Millisecond,
		maxDelay:    10 * time.Second,
		retryIf:     func(err error) bool { return IsRetryable(err) },
		onRetry:     nil,
		backoff:     ExponentialBackoff{},
		jitter:      true,
		ctx:         context.Background(),
	}
	for _, opt := range options {
		opt(r)
	}
	// Ensure retryIf is never nil, falling back to IsRetryable
	if r.retryIf == nil {
		r.retryIf = func(err error) bool { return IsRetryable(err) }
	}
	return r
}

// addJitter adds ±25% jitter to avoid thundering herd problems.
// Returns a duration adjusted by a random value between -25% and +25% of the input; not thread-safe.
func addJitter(d time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(d/2))) - (d / 4)
	return d + jitter
}

// Attempts returns the configured maximum number of retry attempts.
// Includes the initial attempt in the count.
func (r *Retry) Attempts() int {
	return r.maxAttempts
}

// Execute runs the provided function with the configured retry logic.
// Returns nil on success or the last error if all attempts fail; respects context cancellation.
func (r *Retry) Execute(fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if r.retryIf != nil && !r.retryIf(err) {
			return err
		}

		if r.onRetry != nil {
			r.onRetry(attempt, err)
		}

		// Don't delay after last attempt
		if attempt == r.maxAttempts {
			break
		}

		// Calculate delay with backoff
		delay := r.backoff.Backoff(attempt, r.delay)
		if r.maxDelay > 0 && delay > r.maxDelay {
			delay = r.maxDelay
		}
		if r.jitter {
			delay = addJitter(delay)
		}

		// Wait with context
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// ExecuteContext runs the provided function with retry logic, respecting context cancellation.
// Returns nil on success or the last error if all attempts fail or context is cancelled.
func (r *Retry) ExecuteContext(ctx context.Context, fn func() error) error {
	var lastErr error

	// If the retry instance already has a context, use it. Otherwise, use the provided one.
	// If both are provided, maybe create a derived context? For now, prioritize the one from WithContext.
	execCtx := r.ctx
	if execCtx == context.Background() && ctx != nil { // Use provided ctx if retry ctx is default and provided one isn't nil
		execCtx = ctx
	} else if ctx == nil { // Ensure we always have a non-nil context
		execCtx = context.Background()
	}
	// Note: This logic might need refinement depending on how contexts should interact.
	// A safer approach might be: if r.ctx != background, use it. Else use provided ctx.

	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		// Check context before executing the function
		select {
		case <-execCtx.Done():
			return execCtx.Err() // Return context error immediately
		default:
			// Context is okay, proceed
		}

		err := fn()
		if err == nil {
			return nil // Success
		}

		// Check if retry is applicable based on the error
		if r.retryIf != nil && !r.retryIf(err) {
			return err // Not retryable, return the error
		}

		lastErr = err // Store the last encountered error

		// Execute the OnRetry callback if configured
		if r.onRetry != nil {
			r.onRetry(attempt, err)
		}

		// Exit loop if this was the last attempt
		if attempt == r.maxAttempts {
			break
		}

		// --- Calculate and apply delay ---
		currentDelay := r.backoff.Backoff(attempt, r.delay)
		if r.maxDelay > 0 && currentDelay > r.maxDelay { // Check maxDelay > 0 before capping
			currentDelay = r.maxDelay
		}
		if r.jitter {
			currentDelay = addJitter(currentDelay)
		}
		if currentDelay < 0 { // Ensure delay isn't negative after jitter
			currentDelay = 0
		}
		// --- Wait for the delay or context cancellation ---
		select {
		case <-execCtx.Done():
			// If context is cancelled during the wait, return the context error
			// Often more informative than returning the last application error.
			return execCtx.Err()
		case <-time.After(currentDelay):
			// Wait finished, continue to the next attempt
		}
	}

	// All attempts failed, return the last error encountered
	return lastErr
}

// Transform creates a new Retry instance with modified configuration.
// Copies all settings from the original Retry and applies the given options.
func (r *Retry) Transform(opts ...RetryOption) *Retry {
	newRetry := &Retry{
		maxAttempts: r.maxAttempts,
		delay:       r.delay,
		maxDelay:    r.maxDelay,
		retryIf:     r.retryIf,
		onRetry:     r.onRetry,
		backoff:     r.backoff,
		jitter:      r.jitter,
		ctx:         r.ctx,
	}
	for _, opt := range opts {
		opt(newRetry)
	}
	return newRetry
}

// WithBackoff sets the backoff strategy using the BackoffStrategy interface.
// Returns a RetryOption; no-op if strategy is nil, retaining the existing strategy.
func WithBackoff(strategy BackoffStrategy) RetryOption {
	return func(r *Retry) {
		if strategy != nil {
			r.backoff = strategy
		}
	}
}

// WithContext sets the context for cancellation and deadlines.
// Returns a RetryOption; retains context.Background if ctx is nil.
func WithContext(ctx context.Context) RetryOption {
	return func(r *Retry) {
		if ctx != nil {
			r.ctx = ctx
		}
	}
}

// WithDelay sets the initial delay between retries.
// Returns a RetryOption; ensures non-negative delay by setting negatives to 0.
func WithDelay(delay time.Duration) RetryOption {
	return func(r *Retry) {
		if delay < 0 {
			delay = 0
		}
		r.delay = delay
	}
}

// WithJitter enables or disables jitter in the backoff delay.
// Returns a RetryOption; toggles random delay variation.
func WithJitter(jitter bool) RetryOption {
	return func(r *Retry) {
		r.jitter = jitter
	}
}

// WithMaxAttempts sets the maximum number of retry attempts.
// Returns a RetryOption; ensures at least 1 attempt by adjusting lower values.
func WithMaxAttempts(maxAttempts int) RetryOption {
	return func(r *Retry) {
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		r.maxAttempts = maxAttempts
	}
}

// WithMaxDelay sets the maximum delay between retries.
// Returns a RetryOption; ensures non-negative delay by setting negatives to 0.
func WithMaxDelay(maxDelay time.Duration) RetryOption {
	return func(r *Retry) {
		if maxDelay < 0 {
			maxDelay = 0
		}
		r.maxDelay = maxDelay
	}
}

// WithOnRetry sets a callback to execute after each failed attempt.
// Returns a RetryOption; callback receives attempt number and error.
func WithOnRetry(onRetry func(attempt int, err error)) RetryOption {
	return func(r *Retry) {
		r.onRetry = onRetry
	}
}

// WithRetryIf sets the condition under which to retry.
// Returns a RetryOption; retains IsRetryable default if retryIf is nil.
func WithRetryIf(retryIf func(error) bool) RetryOption {
	return func(r *Retry) {
		if retryIf != nil {
			r.retryIf = retryIf
		}
	}
}

// ExecuteReply runs the provided function with retry logic and returns its result.
// Returns the result and nil on success, or zero value and last error on failure; generic type T.
func ExecuteReply[T any](r *Retry, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		// Check if retry is applicable; return immediately if not retryable
		if r.retryIf != nil && !r.retryIf(err) {
			return zero, err
		}

		lastErr = err
		if r.onRetry != nil {
			r.onRetry(attempt, err)
		}

		if attempt == r.maxAttempts {
			break
		}

		// Calculate delay with backoff, cap at maxDelay, and apply jitter if enabled
		currentDelay := r.backoff.Backoff(attempt, r.delay)
		if currentDelay > r.maxDelay {
			currentDelay = r.maxDelay
		}
		if r.jitter {
			currentDelay = addJitter(currentDelay)
		}

		// Wait with respect to context cancellation or timeout
		select {
		case <-r.ctx.Done():
			return zero, r.ctx.Err()
		case <-time.After(currentDelay):
		}
	}
	return zero, lastErr
}
