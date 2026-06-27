package errors

import (
	"context"
	"fmt"
	"log/slog" // Standard structured logging package
	"reflect"
	"strings"
	"time"
)

// Chain executes functions sequentially with enhanced error handling.
// Logging is optional and configured via a slog.Handler.
type Chain struct {
	steps      []chainStep        // List of steps to execute
	errors     []error            // Accumulated errors during execution
	config     chainConfig        // Chain-wide configuration
	lastStep   *chainStep         // Pointer to the last added step for configuration
	logHandler slog.Handler       // Optional logging handler (nil means no logging)
	cancel     context.CancelFunc // Function to cancel the context
}

// chainStep represents a single step in the chain.
type chainStep struct {
	execute  func() error // Function to execute for this step
	optional bool         // If true, errors don't stop the chain
	config   stepConfig   // Step-specific configuration
}

// chainConfig holds chain-wide settings.
type chainConfig struct {
	timeout   time.Duration // Maximum duration for the entire chain
	maxErrors int           // Maximum number of errors before stopping (-1 for unlimited)
	autoWrap  bool          // Whether to automatically wrap errors with additional context
}

// stepConfig holds configuration for an individual step.
type stepConfig struct {
	context      map[string]interface{} // Arbitrary key-value pairs for context
	category     ErrorCategory          // Category for error classification
	code         int                    // Numeric error code
	retry        *Retry                 // Retry policy for the step
	logOnFail    bool                   // Whether to log errors automatically
	metricsLabel string                 // Label for metrics (not used in this code)
	logAttrs     []slog.Attr            // Additional attributes for logging
}

// ChainOption defines a function that configures a Chain.
type ChainOption func(*Chain)

// NewChain creates a new Chain with the given options.
// Logging is disabled by default (logHandler is nil).
func NewChain(opts ...ChainOption) *Chain {
	c := &Chain{
		config: chainConfig{
			autoWrap:  true, // Enable error wrapping by default
			maxErrors: -1,   // No limit on errors by default
		},
		// logHandler is nil, meaning no logging unless explicitly configured
	}
	// Apply each configuration option
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ChainWithLogHandler sets a custom slog.Handler for logging.
// If handler is nil, logging is effectively disabled.
func ChainWithLogHandler(handler slog.Handler) ChainOption {
	return func(c *Chain) {
		c.logHandler = handler
	}
}

// ChainWithTimeout sets a timeout for the entire chain.
func ChainWithTimeout(d time.Duration) ChainOption {
	return func(c *Chain) {
		c.config.timeout = d
	}
}

// ChainWithMaxErrors sets the maximum number of errors allowed.
// A value <= 0 means no limit.
func ChainWithMaxErrors(max int) ChainOption {
	return func(c *Chain) {
		if max <= 0 {
			c.config.maxErrors = -1 // No limit
		} else {
			c.config.maxErrors = max
		}
	}
}

// ChainWithAutoWrap enables or disables automatic error wrapping.
func ChainWithAutoWrap(auto bool) ChainOption {
	return func(c *Chain) {
		c.config.autoWrap = auto
	}
}

// Step adds a new step to the chain with the provided function.
// The function must return an error or nil.
func (c *Chain) Step(fn func() error) *Chain {
	if fn == nil {
		// Panic to enforce valid input
		panic("Chain.Step: provided function cannot be nil")
	}
	// Create a new step with default configuration
	step := chainStep{execute: fn, config: stepConfig{}}
	c.steps = append(c.steps, step)
	// Update lastStep to point to the newly added step
	c.lastStep = &c.steps[len(c.steps)-1]
	return c
}

// Call adds a step by wrapping a function with arguments.
// It uses reflection to validate and invoke the function.
func (c *Chain) Call(fn interface{}, args ...interface{}) *Chain {
	// Wrap the function and arguments into an executable step
	wrappedFn, err := c.wrapCallable(fn, args...)
	if err != nil {
		// Panic on setup errors to catch them early
		panic(fmt.Sprintf("Chain.Call setup error: %v", err))
	}
	// Add the wrapped function as a step
	step := chainStep{execute: wrappedFn, config: stepConfig{}}
	c.steps = append(c.steps, step)
	c.lastStep = &c.steps[len(c.steps)-1]
	return c
}

// Optional marks the last step as optional.
// Optional steps don't stop the chain on error.
func (c *Chain) Optional() *Chain {
	if c.lastStep == nil {
		// Panic if no step exists to mark as optional
		panic("Chain.Optional: must call Step() or Call() before Optional()")
	}
	c.lastStep.optional = true
	return c
}

// WithLog adds logging attributes to the last step.
func (c *Chain) WithLog(attrs ...slog.Attr) *Chain {
	if c.lastStep == nil {
		// Panic if no step exists to configure
		panic("Chain.WithLog: must call Step() or Call() before WithLog()")
	}
	// Append attributes to the step's logging configuration
	c.lastStep.config.logAttrs = append(c.lastStep.config.logAttrs, attrs...)
	return c
}

// Timeout sets a timeout for the entire chain.
func (c *Chain) Timeout(d time.Duration) *Chain {
	c.config.timeout = d
	return c
}

// MaxErrors sets the maximum number of errors allowed.
func (c *Chain) MaxErrors(max int) *Chain {
	if max <= 0 {
		c.config.maxErrors = -1 // No limit
	} else {
		c.config.maxErrors = max
	}
	return c
}

// With adds a key-value pair to the last step's context.
func (c *Chain) With(key string, value interface{}) *Chain {
	if c.lastStep == nil {
		// Panic if no step exists to configure
		panic("Chain.With: must call Step() or Call() before With()")
	}
	// Initialize context map if nil
	if c.lastStep.config.context == nil {
		c.lastStep.config.context = make(map[string]interface{})
	}
	// Add the key-value pair
	c.lastStep.config.context[key] = value
	return c
}

// Tag sets an error category for the last step.
func (c *Chain) Tag(category ErrorCategory) *Chain {
	if c.lastStep == nil {
		// Panic if no step exists to configure
		panic("Chain.Tag: must call Step() or Call() before Tag()")
	}
	c.lastStep.config.category = category
	return c
}

// Code sets a numeric error code for the last step.
func (c *Chain) Code(code int) *Chain {
	if c.lastStep == nil {
		// Panic if no step exists to configure
		panic("Chain.Code: must call Step() or Call() before Code()")
	}
	c.lastStep.config.code = code
	return c
}

// Retry configures retry behavior for the last step.
// Retry configures retry behavior for the last step.
func (c *Chain) Retry(maxAttempts int, delay time.Duration, opts ...RetryOption) *Chain {
	if c.lastStep == nil {
		panic("Chain.Retry: must call Step() or Call() before Retry()")
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	// Define default retry options
	retryOpts := []RetryOption{
		WithMaxAttempts(maxAttempts),
		WithDelay(delay),
		WithRetryIf(func(err error) bool { return IsRetryable(err) }),
	}

	// Add logging for retry attempts if a handler is configured
	if c.logHandler != nil {
		step := c.lastStep
		retryOpts = append(retryOpts, WithOnRetry(func(attempt int, err error) {
			// Prepare logging attributes
			logAttrs := []slog.Attr{
				slog.Int("attempt", attempt),
				slog.Int("max_attempts", maxAttempts),
			}
			// Enhance the error with step context
			enhancedErr := c.enhanceError(err, step)
			// Log the retry attempt
			c.logError(enhancedErr, fmt.Sprintf("Retrying step (attempt %d/%d)", attempt, maxAttempts), step.config, logAttrs...)
		}))
	}

	// Append any additional retry options
	retryOpts = append(retryOpts, opts...)
	// Create and assign the retry configuration
	c.lastStep.config.retry = NewRetry(retryOpts...)
	return c
}

// LogOnFail enables automatic logging of errors for the last step.
func (c *Chain) LogOnFail() *Chain {
	if c.lastStep == nil {
		// Panic if no step exists to configure
		panic("Chain.LogOnFail: must call Step() or Call() before LogOnFail()")
	}
	c.lastStep.config.logOnFail = true
	return c
}

// Run executes the chain, stopping on the first non-optional error.
// It returns the first error encountered or nil if all steps succeed.
func (c *Chain) Run() error {
	// Create a context with timeout or cancellation
	ctx, cancel := c.getContextAndCancel()
	defer cancel()
	c.cancel = cancel
	// Clear any previous errors
	c.errors = c.errors[:0]

	// Execute each step in sequence
	for i := range c.steps {
		step := &c.steps[i]
		// Check if the context has been canceled
		select {
		case <-ctx.Done():
			err := ctx.Err()
			// Enhance the error with step context
			enhancedErr := c.enhanceError(err, step)
			c.errors = append(c.errors, enhancedErr)
			// Log the context error
			c.logError(enhancedErr, "Chain stopped due to context error before step", step.config)
			return enhancedErr
		default:
		}

		// Execute the step
		err := c.executeStep(ctx, step)
		if err != nil {
			// Enhance the error with step context
			enhancedErr := c.enhanceError(err, step)
			c.errors = append(c.errors, enhancedErr)
			// Log the error if required
			if step.config.logOnFail || !step.optional {
				logMsg := "Chain stopped due to error in step"
				if step.optional {
					logMsg = "Optional step failed"
				}
				c.logError(enhancedErr, logMsg, step.config)
			}
			// Stop execution if the step is not optional
			if !step.optional {
				return enhancedErr
			}
		}
	}
	// Return nil if all steps completed successfully
	return nil
}

// RunAll executes all steps, collecting errors without stopping.
// It returns a MultiError containing all errors or nil if none occurred.
func (c *Chain) RunAll() error {
	ctx, cancel := c.getContextAndCancel()
	defer cancel()
	c.cancel = cancel
	c.errors = c.errors[:0]
	multi := NewMultiError()

	for i := range c.steps {
		step := &c.steps[i]
		select {
		case <-ctx.Done():
			err := ctx.Err()
			enhancedErr := c.enhanceError(err, step)
			c.errors = append(c.errors, enhancedErr)
			multi.Add(enhancedErr)
			c.logError(enhancedErr, "Chain stopped due to context error before step (RunAll)", step.config)
			goto endRunAll
		default:
		}

		err := c.executeStep(ctx, step)
		if err != nil {
			enhancedErr := c.enhanceError(err, step)
			c.errors = append(c.errors, enhancedErr)
			multi.Add(enhancedErr)
			if step.config.logOnFail && c.logHandler != nil {
				c.logError(enhancedErr, "Step failed during RunAll", step.config)
			}
			if c.config.maxErrors > 0 && multi.Count() >= c.config.maxErrors {
				if c.logHandler != nil {
					// Create a logger to log the max errors condition
					logger := slog.New(c.logHandler)
					logger.LogAttrs(
						context.Background(),
						slog.LevelError,
						fmt.Sprintf("Stopping RunAll after reaching max errors (%d)", c.config.maxErrors),
						slog.Int("max_errors", c.config.maxErrors),
					)
				}
				goto endRunAll
			}
		}
	}

endRunAll:
	return multi.Single()
}

// Errors returns a copy of the collected errors.
func (c *Chain) Errors() []error {
	if len(c.errors) == 0 {
		return nil
	}
	// Create a copy to prevent external modification
	errs := make([]error, len(c.errors))
	copy(errs, c.errors)
	return errs
}

// Len returns the number of steps in the chain.
func (c *Chain) Len() int {
	return len(c.steps)
}

// HasErrors checks if any errors were collected.
func (c *Chain) HasErrors() bool {
	return len(c.errors) > 0
}

// LastError returns the most recent error or nil if none exist.
func (c *Chain) LastError() error {
	if len(c.errors) > 0 {
		return c.errors[len(c.errors)-1]
	}
	return nil
}

// Reset clears the chain's steps, errors, and context.
func (c *Chain) Reset() {
	if c.cancel != nil {
		// Cancel any active context
		c.cancel()
		c.cancel = nil
	}
	// Clear steps and errors
	c.steps = c.steps[:0]
	c.errors = c.errors[:0]
	c.lastStep = nil
}

// Unwrap returns the collected errors (alias for Errors).
func (c *Chain) Unwrap() []error {
	return c.errors
}

// getContextAndCancel creates a context based on the chain's timeout.
// It returns a context and its cancellation function.
func (c *Chain) getContextAndCancel() (context.Context, context.CancelFunc) {
	parentCtx := context.Background()
	if c.config.timeout > 0 {
		// Create a context with a timeout
		return context.WithTimeout(parentCtx, c.config.timeout)
	}
	// Create a cancellable context
	return context.WithCancel(parentCtx)
}

// logError logs an error with step-specific context and attributes.
// It only logs if a handler is configured and the error is non-nil.
func (c *Chain) logError(err error, msg string, config stepConfig, additionalAttrs ...slog.Attr) {
	// Skip logging if no handler is set or error is nil
	if c == nil || c.logHandler == nil || err == nil {
		return
	}

	// Create a logger on demand using the configured handler
	logger := slog.New(c.logHandler)

	// Initialize attributes with error and timestamp
	allAttrs := make([]slog.Attr, 0, 5+len(config.logAttrs)+len(additionalAttrs))
	allAttrs = append(allAttrs, slog.Any("error", err))
	allAttrs = append(allAttrs, slog.Time("timestamp", time.Now()))

	// Add step-specific metadata
	if config.category != "" {
		allAttrs = append(allAttrs, slog.String("category", string(config.category)))
	}
	if config.code != 0 {
		allAttrs = append(allAttrs, slog.Int("code", config.code))
	}
	for k, v := range config.context {
		allAttrs = append(allAttrs, slog.Any(k, v))
	}
	allAttrs = append(allAttrs, config.logAttrs...)
	allAttrs = append(allAttrs, additionalAttrs...)

	// Add stack trace and error name if the error is of type *Error
	if e, ok := err.(*Error); ok {
		if stack := e.Stack(); len(stack) > 0 {
			// Format stack trace, truncating if too long
			stackStr := "\n\t" + strings.Join(stack, "\n\t")
			if len(stackStr) > 1000 {
				stackStr = stackStr[:1000] + "..."
			}
			allAttrs = append(allAttrs, slog.String("stacktrace", stackStr))
		}
		if name := e.Name(); name != "" {
			allAttrs = append(allAttrs, slog.String("error_name", name))
		}
	}

	// Log the error at ERROR level with all attributes
	// Use a defer to catch any panics during logging
	defer func() {
		if r := recover(); r != nil {
			// Print to stdout to avoid infinite recursion
			fmt.Printf("ERROR: Recovered from panic during logging: %v\nAttributes: %v\n", r, allAttrs)
		}
	}()
	logger.LogAttrs(context.Background(), slog.LevelError, msg, allAttrs...)
}

// wrapCallable wraps a function and its arguments into an executable step.
// It uses reflection to validate the function and arguments.
func (c *Chain) wrapCallable(fn interface{}, args ...interface{}) (func() error, error) {
	val := reflect.ValueOf(fn)
	typ := val.Type()

	// Ensure the provided value is a function
	if typ.Kind() != reflect.Func {
		return nil, fmt.Errorf("provided 'fn' is not a function (got %T)", fn)
	}
	// Check if the number of arguments matches the function's signature
	if typ.NumIn() != len(args) {
		return nil, fmt.Errorf("function expects %d arguments, but %d were provided", typ.NumIn(), len(args))
	}

	// Prepare argument values
	argVals := make([]reflect.Value, len(args))
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	for i, arg := range args {
		expectedType := typ.In(i)
		var providedVal reflect.Value
		if arg != nil {
			providedVal = reflect.ValueOf(arg)
			// Check if the argument type is assignable to the expected type
			if !providedVal.Type().AssignableTo(expectedType) {
				// Special case for error interfaces
				if expectedType.Kind() == reflect.Interface && expectedType.Implements(errorType) && providedVal.Type().Implements(errorType) {
					// Allow error interface
				} else {
					return nil, fmt.Errorf("argument %d type mismatch: expected %s, got %s", i, expectedType, providedVal.Type())
				}
			}
		} else {
			// Handle nil arguments for nullable types
			switch expectedType.Kind() {
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
				providedVal = reflect.Zero(expectedType)
			default:
				return nil, fmt.Errorf("argument %d is nil, but expected non-nillable type %s", i, expectedType)
			}
		}
		argVals[i] = providedVal
	}

	// Validate the function's return type
	if typ.NumOut() > 1 || (typ.NumOut() == 1 && !typ.Out(0).Implements(errorType)) {
		return nil, fmt.Errorf("function must return either no values or a single error (got %d return values)", typ.NumOut())
	}

	// Return a wrapped function that calls the original with the provided arguments
	return func() error {
		results := val.Call(argVals)
		if len(results) == 1 && results[0].Interface() != nil {
			return results[0].Interface().(error)
		}
		return nil
	}, nil
}

// executeStep runs a single step, applying retries if configured.
// This version is synchronous and avoids the bugs caused by the previous goroutine-based implementation.
func (c *Chain) executeStep(ctx context.Context, step *chainStep) error {
	// First, check if the context has already been canceled before starting the step.
	// This allows the chain to fail fast.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Context is still active, proceed.
	}

	// If the step has retry logic configured...
	if step.config.retry != nil {
		// Create a new retry instance that is aware of the chain's context.
		// The retry executor will be responsible for checking ctx.Done() between attempts.
		retryExecutor := step.config.retry.Transform(WithContext(ctx))

		// Execute the step's function directly. The retry mechanism will manage the loop,
		// delays, and context cancellation checks. We pass step.execute without any
		// extra goroutine wrappers.
		return retryExecutor.Execute(step.execute)
	}

	// For a simple, non-retrying step, execute the function directly and synchronously
	// in the current goroutine. This is the simplest, fastest, and most correct approach.
	// It ensures that database connections are used and returned to the pool sequentially,
	// preventing the deadlock issue.
	return step.execute()
}

// enhanceError wraps an error with additional context from the step.
func (c *Chain) enhanceError(err error, step *chainStep) error {
	if err == nil || !c.config.autoWrap {
		// Return the error unchanged if nil or autoWrap is disabled
		return err
	}

	// Initialize the base error
	var baseError *Error
	if e, ok := err.(*Error); ok {
		// Copy existing *Error to preserve its properties
		baseError = e.Copy()
	} else {
		// Create a new *Error wrapping the original
		baseError = New(err.Error()).Wrap(err).WithStack()
	}

	if step != nil {
		// Add step-specific context to the error
		if step.config.category != "" && baseError.Category() == "" {
			baseError.WithCategory(step.config.category)
		}
		if step.config.code != 0 && baseError.Code() == 0 {
			baseError.WithCode(step.config.code)
		}
		for k, v := range step.config.context {
			baseError.With(k, v)
		}
		for _, attr := range step.config.logAttrs {
			baseError.With(attr.Key, attr.Value.Any())
		}
		if step.config.retry != nil && !baseError.HasContextKey(ctxRetry) {
			// Mark the error as retryable if retries are configured
			baseError.WithRetryable()
		}
	}

	return baseError
}
