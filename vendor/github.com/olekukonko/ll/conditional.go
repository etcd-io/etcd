package ll

// Conditional enables conditional logging based on a boolean condition.
// It wraps a logger with a condition that determines whether logging operations are executed,
// optimizing performance by skipping expensive operations (e.g., field computation, message formatting)
// when the condition is false. The struct supports fluent chaining for adding fields and logging.
type Conditional struct {
	logger    *Logger // Associated logger instance for logging operations
	condition bool    // Whether logging is allowed (true to log, false to skip)
}

// If creates a conditional logger that logs only if the condition is true.
// It returns a Conditional struct that wraps the logger, enabling conditional logging methods.
// This method is typically called on a Logger instance to start a conditional chain.
// Thread-safe via the underlying logger's mutex.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Info("Logged")   // Output: [app] INFO: Logged
//	logger.If(false).Info("Ignored") // No output
func (l *Logger) If(condition bool) *Conditional {
	return &Conditional{logger: l, condition: condition}
}

// IfAny creates a conditional logger that logs only if at least one condition is true.
// It evaluates a variadic list of boolean conditions, setting the condition to true if any
// is true (logical OR). Returns a new Conditional with the result. Thread-safe via the
// underlying logger.
// Example:
//
//	logger := New("app").Enable()
//	logger.IfAny(false, true).Info("Logged")   // Output: [app] INFO: Logged
//	logger.IfAny(false, false).Info("Ignored") // No output
func (cl *Conditional) IfAny(conditions ...bool) *Conditional {
	result := false
	// Check each condition; set result to true if any is true
	for _, cond := range conditions {
		if cond {
			result = true
			break
		}
	}
	return &Conditional{logger: cl.logger, condition: result}
}

// IfErr creates a conditional logger that logs only if the error is non-nil.
// It's designed for the common pattern of checking errors before logging.
// Example:
//
//	err := doSomething()
//	logger.IfErr(err).Error("Operation failed") // Only logs if err != nil
func (l *Logger) IfErr(err error) *Conditional {
	return l.If(err != nil)
}

// IfErrAny creates a conditional logger that logs only if AT LEAST ONE error is non-nil.
// It evaluates a variadic list of errors, setting the condition to true if any
// is non-nil (logical OR). Useful when any error should trigger logging.
// Example:
//
//	err1 := validate(input)
//	err2 := authorize(user)
//	logger.IfErrAny(err1, err2).Error("Either check failed") // Logs if EITHER error exists
func (l *Logger) IfErrAny(errs ...error) *Conditional {
	for _, err := range errs {
		if err != nil {
			return l.If(true) // Any non-nil error makes it true
		}
	}
	return l.If(false) // False only if all errors are nil
}

// IfErrOne creates a conditional logger that logs only if ALL errors are non-nil.
// It evaluates a variadic list of errors, setting the condition to true only if
// all are non-nil (logical AND). Useful when you need all errors to be present.
// Example:
//
//	err1 := validate(input)
//	err2 := authorize(user)
//	logger.IfErrOne(err1, err2).Error("Both checks failed") // Logs only if BOTH errors exist
func (l *Logger) IfErrOne(errs ...error) *Conditional {
	for _, err := range errs {
		if err == nil {
			return l.If(false) // Any nil error makes it false
		}
	}
	return l.If(len(errs) > 0) // True only if we have at least one error and all are non-nil
}

// IfErr creates a conditional logger that logs only if the error is non-nil.
// Returns a new Conditional with the error check result.
// Example:
//
//	err := doSomething()
//	logger.If(true).IfErr(err).Error("Failed") // Only logs if condition true AND err != nil
func (cl *Conditional) IfErr(err error) *Conditional {
	return cl.IfOne(err != nil)
}

// IfErrAny creates a conditional logger that logs only if AT LEAST ONE error is non-nil.
// Returns a new Conditional with the logical OR result of error checks.
// Example:
//
//	err1 := validate(input)
//	err2 := authorize(user)
//	logger.If(true).IfErrAny(err1, err2).Error("Either failed") // Logs if condition true AND either error exists
func (cl *Conditional) IfErrAny(errs ...error) *Conditional {
	for _, err := range errs {
		if err != nil {
			return &Conditional{logger: cl.logger, condition: cl.condition && true}
		}
	}
	return &Conditional{logger: cl.logger, condition: false}
}

// IfErrOne creates a conditional logger that logs only if ALL errors are non-nil.
// Returns a new Conditional with the logical AND result of error checks.
// Example:
//
//	err1 := validate(input)
//	err2 := authorize(user)
//	logger.If(true).IfErrOne(err1, err2).Error("Both failed") // Logs if condition true AND both errors exist
func (cl *Conditional) IfErrOne(errs ...error) *Conditional {
	for _, err := range errs {
		if err == nil {
			return &Conditional{logger: cl.logger, condition: false}
		}
	}
	return &Conditional{logger: cl.logger, condition: cl.condition && len(errs) > 0}
}

// IfOne creates a conditional logger that logs only if all conditions are true.
// It evaluates a variadic list of boolean conditions, setting the condition to true only if
// all are true (logical AND). Returns a new Conditional with the result. Thread-safe via the
// underlying logger.
// Example:
//
//	logger := New("app").Enable()
//	logger.IfOne(true, true).Info("Logged")   // Output: [app] INFO: Logged
//	logger.IfOne(true, false).Info("Ignored") // No output
func (cl *Conditional) IfOne(conditions ...bool) *Conditional {
	result := true
	// Check each condition; set result to false if any is false
	for _, cond := range conditions {
		if !cond {
			result = false
			break
		}
	}
	return &Conditional{logger: cl.logger, condition: result}
}

// Debug logs a message at Debug level with variadic arguments if the condition is true.
// It concatenates the arguments with spaces and delegates to the logger's Debug method if the
// condition is true. Skips processing if false, optimizing performance. Thread-safe via the
// logger's log method.
// Example:
//
//	logger := New("app").Enable().Level(lx.LevelDebug)
//	logger.If(true).Debug("Debugging", "mode")   // Output: [app] DEBUG: Debugging mode
//	logger.If(false).Debug("Debugging", "ignored") // No output
func (cl *Conditional) Debug(args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Debug method
	cl.logger.Debug(args...)
}

// Debugf logs a message at Debug level with a format string if the condition is true.
// It formats the message and delegates to the logger's Debugf method if the condition is true.
// Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable().Level(lx.LevelDebug)
//	logger.If(true).Debugf("Debug %s", "mode")   // Output: [app] DEBUG: Debug mode
//	logger.If(false).Debugf("Debug %s", "ignored") // No output
func (cl *Conditional) Debugf(format string, args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Debugf method
	cl.logger.Debugf(format, args...)
}

// Error logs a message at Error level with variadic arguments if the condition is true.
// It concatenates the arguments with spaces and delegates to the logger's Error method if the
// condition is true. Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Error("Error", "occurred")   // Output: [app] ERROR: Error occurred
//	logger.If(false).Error("Error", "ignored") // No output
func (cl *Conditional) Error(args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Error method
	cl.logger.Error(args...)
}

// Errorf logs a message at Error level with a format string if the condition is true.
// It formats the message and delegates to the logger's Errorf method if the condition is true.
// Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Errorf("Error %s", "occurred")   // Output: [app] ERROR: Error occurred
//	logger.If(false).Errorf("Error %s", "ignored") // No output
func (cl *Conditional) Errorf(format string, args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Errorf method
	cl.logger.Errorf(format, args...)
}

// Fatal logs a message at Error level with a stack trace and variadic arguments if the condition is true,
// then exits. It concatenates the arguments with spaces and delegates to the logger's Fatal method
// if the condition is true, terminating the program with exit code 1. Skips processing if false.
// Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Fatal("Fatal", "error")   // Output: [app] ERROR: Fatal error [stack=...], then exits
//	logger.If(false).Fatal("Fatal", "ignored") // No output, no exit
func (cl *Conditional) Fatal(args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Fatal method
	cl.logger.Fatal(args...)
}

// Fatalf logs a formatted message at Error level with a stack trace if the condition is true, then exits.
// It formats the message and delegates to the logger's Fatalf method if the condition is true,
// terminating the program with exit code 1. Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Fatalf("Fatal %s", "error")   // Output: [app] ERROR: Fatal error [stack=...], then exits
//	logger.If(false).Fatalf("Fatal %s", "ignored") // No output, no exit
func (cl *Conditional) Fatalf(format string, args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Fatalf method
	cl.logger.Fatalf(format, args...)
}

// Field starts a fluent chain for adding fields from a map, if the condition is true.
// It returns a FieldBuilder to attach fields from a map, skipping processing if the condition
// is false. Thread-safe via the FieldBuilder's logger.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Field(map[string]interface{}{"user": "alice"}).Info("Logged") // Output: [app] INFO: Logged [user=alice]
//	logger.If(false).Field(map[string]interface{}{"user": "alice"}).Info("Ignored") // No output
func (cl *Conditional) Field(fields map[string]interface{}) *FieldBuilder {
	// Skip field processing if condition is false
	if !cl.condition {
		return &FieldBuilder{logger: cl.logger, fields: nil}
	}
	// Delegate to logger's Field method
	return cl.logger.Field(fields)
}

// Fields starts a fluent chain for adding fields using variadic key-value pairs, if the condition is true.
// It returns a FieldBuilder to attach fields, skipping field processing if the condition is false
// to optimize performance. Thread-safe via the FieldBuilder's logger.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Fields("user", "alice").Info("Logged") // Output: [app] INFO: Logged [user=alice]
//	logger.If(false).Fields("user", "alice").Info("Ignored") // No output, no field processing
func (cl *Conditional) Fields(pairs ...any) *FieldBuilder {
	// Skip field processing if condition is false
	if !cl.condition {
		return &FieldBuilder{logger: cl.logger, fields: nil}
	}
	// Delegate to logger's Fields method
	return cl.logger.Fields(pairs...)
}

// Info logs a message at Info level with variadic arguments if the condition is true.
// It concatenates the arguments with spaces and delegates to the logger's Info method if the
// condition is true. Skips processing if false, optimizing performance. Thread-safe via the
// logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Info("Action", "started")   // Output: [app] INFO: Action started
//	logger.If(false).Info("Action", "ignored") // No output
func (cl *Conditional) Info(args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Info method
	cl.logger.Info(args...)
}

// Infof logs a message at Info level with a format string if the condition is true.
// It formats the message using the provided format string and arguments, delegating to the
// logger's Infof method if the condition is true. Skips processing if false, optimizing performance.
// Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Infof("Action %s", "started")   // Output: [app] INFO: Action started
//	logger.If(false).Infof("Action %s", "ignored") // No output
func (cl *Conditional) Infof(format string, args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Infof method
	cl.logger.Infof(format, args...)
}

// Panic logs a message at Error level with a stack trace and variadic arguments if the condition is true,
// then panics. It concatenates the arguments with spaces and delegates to the logger's Panic method
// if the condition is true, triggering a panic. Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Panic("Panic", "error")   // Output: [app] ERROR: Panic error [stack=...], then panics
//	logger.If(false).Panic("Panic", "ignored") // No output, no panic
func (cl *Conditional) Panic(args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Panic method
	cl.logger.Panic(args...)
}

// Panicf logs a formatted message at Error level with a stack trace if the condition is true, then panics.
// It formats the message and delegates to the logger's Panicf method if the condition is true,
// triggering a panic. Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Panicf("Panic %s", "error")   // Output: [app] ERROR: Panic error [stack=...], then panics
//	logger.If(false).Panicf("Panic %s", "ignored") // No output, no panic
func (cl *Conditional) Panicf(format string, args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Panicf method
	cl.logger.Panicf(format, args...)
}

// Stack logs a message at Error level with a stack trace and variadic arguments if the condition is true.
// It concatenates the arguments with spaces and delegates to the logger's Stack method if the
// condition is true. Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Stack("Critical", "error")   // Output: [app] ERROR: Critical error [stack=...]
//	logger.If(false).Stack("Critical", "ignored") // No output
func (cl *Conditional) Stack(args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Stack method
	cl.logger.Stack(args...)
}

// Stackf logs a message at Error level with a stack trace and a format string if the condition is true.
// It formats the message and delegates to the logger's Stackf method if the condition is true.
// Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Stackf("Critical %s", "error")   // Output: [app] ERROR: Critical error [stack=...]
//	logger.If(false).Stackf("Critical %s", "ignored") // No output
func (cl *Conditional) Stackf(format string, args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Stackf method
	cl.logger.Stackf(format, args...)
}

// Warn logs a message at Warn level with variadic arguments if the condition is true.
// It concatenates the arguments with spaces and delegates to the logger's Warn method if the
// condition is true. Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Warn("Warning", "issued")   // Output: [app] WARN: Warning issued
//	logger.If(false).Warn("Warning", "ignored") // No output
func (cl *Conditional) Warn(args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Warn method
	cl.logger.Warn(args...)
}

// Warnf logs a message at Warn level with a format string if the condition is true.
// It formats the message and delegates to the logger's Warnf method if the condition is true.
// Skips processing if false. Thread-safe via the logger's log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.If(true).Warnf("Warning %s", "issued")   // Output: [app] WARN: Warning issued
//	logger.If(false).Warnf("Warning %s", "ignored") // No output
func (cl *Conditional) Warnf(format string, args ...any) {
	// Skip logging if condition is false
	if !cl.condition {
		return
	}
	// Delegate to logger's Warnf method
	cl.logger.Warnf(format, args...)
}
