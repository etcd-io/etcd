package ll

import (
	"fmt"
	"strings"
	"time"

	"github.com/olekukonko/ll/lx"
)

// Measure executes one or more functions and logs the duration of each.
// It returns the total cumulative duration across all functions.
//
// Each function in `fns` is run sequentially. If a function is `nil`, it is skipped.
//
// Optional labels previously set via `Labels(...)` are applied to the corresponding function
// by position. If there are fewer labels than functions, missing labels are replaced with
// default names like "fn_0", "fn_1", etc. Labels are cleared after the call to prevent reuse.
//
// Example usage:
//
//	logger := New("app").Enable()
//
//	// Optional: add labels for functions
//	logger.Labels("load_users", "process_orders")
//
//	total := logger.Measure(
//	    func() {
//	        // simulate work 1
//	        time.Sleep(100 * time.Millisecond)
//	    },
//	    func() {
//	        // simulate work 2
//	        time.Sleep(200 * time.Millisecond)
//	    },
//	    func() {
//	        // simulate work 3
//	        time.Sleep(50 * time.Millisecond)
//	    },
//	)
//
//	// Logs something like:
//	// [load_users] completed  duration=100ms
//	// [process_orders] completed  duration=200ms
//	// [fn_2] completed  duration=50ms
//
// Returns the sum of durations of all executed functions.
func (l *Logger) Measure(fns ...func()) time.Duration {
	if len(fns) == 0 {
		return 0
	}

	var total time.Duration
	lblPtr := l.labels.Swap(nil)
	var lbls []string
	if lblPtr != nil {
		lbls = *lblPtr
	}

	for i, fn := range fns {
		if fn == nil {
			continue
		}
		// Use SinceBuilder instead of manual timing
		sb := l.Since() // starts timer internally
		fn()
		duration := sb.Fields(
			"index", i,
		).Info(fmt.Sprintf("[%s] completed", func() string {
			if i < len(lbls) && lbls[i] != "" {
				return lbls[i]
			}
			return fmt.Sprintf("fn_%d", i)
		}()))

		total += duration
	}

	return total
}

// Since creates a timer that will log the duration when completed
// If startTime is provided, uses that as the start time; otherwise uses time.Now()
//
//	defer logger.Since().Info("request")        // Auto-start
//	logger.Since(start).Info("request")         // Manual timing
//	logger.Since().If(debug).Debug("timing")    // Conditional
func (l *Logger) Since(startTime ...time.Time) *SinceBuilder {
	start := time.Now()
	if len(startTime) > 0 && !startTime[0].IsZero() {
		start = startTime[0]
	}

	return &SinceBuilder{
		logger:    l,
		start:     start,
		condition: true,
		fields:    nil, // Lazily initialized
	}
}

// SinceBuilder provides a fluent API for logging timed operations
// It mirrors FieldBuilder exactly for field operations
type SinceBuilder struct {
	logger    *Logger
	start     time.Time
	condition bool
	fields    lx.Fields
}

// ---------------------------------------------------------------------
// Conditional Methods (match conditional.go pattern)
// ---------------------------------------------------------------------

// If adds a condition to this timer - only logs if condition is true
func (sb *SinceBuilder) If(condition bool) *SinceBuilder {
	sb.condition = sb.condition && condition
	return sb
}

// IfErr adds an error condition - only logs if err != nil
func (sb *SinceBuilder) IfErr(err error) *SinceBuilder {
	sb.condition = sb.condition && (err != nil)
	return sb
}

// IfAny logs if ANY condition is true
func (sb *SinceBuilder) IfAny(conditions ...bool) *SinceBuilder {
	if !sb.condition {
		return sb
	}

	for _, cond := range conditions {
		if cond {
			return sb
		}
	}
	sb.condition = false
	return sb
}

// IfOne logs if ALL conditions are true
func (sb *SinceBuilder) IfOne(conditions ...bool) *SinceBuilder {
	if !sb.condition {
		return sb
	}

	for _, cond := range conditions {
		if !cond {
			sb.condition = false
			return sb
		}
	}
	return sb
}

// ---------------------------------------------------------------------
// Field Methods - EXACT MATCH with FieldBuilder API
// ---------------------------------------------------------------------

// Fields adds key-value pairs as fields (variadic)
// EXACT match to FieldBuilder.Fields()
func (sb *SinceBuilder) Fields(pairs ...any) *SinceBuilder {
	if sb.logger.suspend.Load() || !sb.condition {
		return sb
	}

	// Lazy initialization
	if sb.fields == nil {
		sb.fields = make(lx.Fields, 0, len(pairs)/2)
	}

	// Process key-value pairs
	for i := 0; i < len(pairs)-1; i += 2 {
		if key, ok := pairs[i].(string); ok {
			sb.fields = append(sb.fields, lx.Field{Key: key, Value: pairs[i+1]})
		} else {
			// Log error for non-string keys (matches Fields behavior)
			sb.fields = append(sb.fields, lx.Field{
				Key:   "error",
				Value: fmt.Errorf("missing key '%v'", pairs[i]),
			})
		}
	}

	// Handle uneven pairs (matches Fields behavior)
	if len(pairs)%2 != 0 {
		sb.fields = append(sb.fields, lx.Field{
			Key:   "error",
			Value: fmt.Errorf("missing key '%v'", pairs[len(pairs)-1]),
		})
	}

	return sb
}

// Field adds fields from a map
// EXACT match to FieldBuilder.Field()
func (sb *SinceBuilder) Field(fields map[string]interface{}) *SinceBuilder {
	if sb.logger.suspend.Load() || !sb.condition || len(fields) == 0 {
		return sb
	}

	// Lazy initialization
	if sb.fields == nil {
		sb.fields = make(lx.Fields, 0, len(fields))
	}

	// Copy fields from input map (preserves iteration order)
	for k, v := range fields {
		sb.fields = append(sb.fields, lx.Field{Key: k, Value: v})
	}

	return sb
}

// Err adds one or more errors as a field
// EXACT match to FieldBuilder.Err()
func (sb *SinceBuilder) Err(errs ...error) *SinceBuilder {
	if sb.logger.suspend.Load() || !sb.condition {
		return sb
	}

	// Lazy initialization
	if sb.fields == nil {
		sb.fields = make(lx.Fields, 0, 2)
	}

	// Collect non-nil errors
	var nonNilErrors []error
	var builder strings.Builder
	count := 0

	for i, err := range errs {
		if err != nil {
			if i > 0 && count > 0 {
				builder.WriteString("; ")
			}
			builder.WriteString(err.Error())
			nonNilErrors = append(nonNilErrors, err)
			count++
		}
	}

	if count > 0 {
		if count == 1 {
			sb.fields = append(sb.fields, lx.Field{Key: "error", Value: nonNilErrors[0]})
		} else {
			sb.fields = append(sb.fields, lx.Field{Key: "error", Value: nonNilErrors})
		}
		// Note: Unlike FieldBuilder.Err(), we DON'T log immediately
		// The error will be included in the timing log
	}

	return sb
}

// Merge adds additional key-value pairs to the fields
// EXACT match to FieldBuilder.Merge()
func (sb *SinceBuilder) Merge(pairs ...any) *SinceBuilder {
	if sb.logger.suspend.Load() || !sb.condition {
		return sb
	}

	// Lazy initialization
	if sb.fields == nil {
		sb.fields = make(lx.Fields, 0, len(pairs)/2)
	}

	// Process pairs as key-value
	for i := 0; i < len(pairs)-1; i += 2 {
		if key, ok := pairs[i].(string); ok {
			sb.fields = append(sb.fields, lx.Field{Key: key, Value: pairs[i+1]})
		} else {
			sb.fields = append(sb.fields, lx.Field{
				Key:   "error",
				Value: fmt.Errorf("non-string key in Merge: %v", pairs[i]),
			})
		}
	}

	if len(pairs)%2 != 0 {
		sb.fields = append(sb.fields, lx.Field{
			Key:   "error",
			Value: fmt.Errorf("uneven key-value pairs in Merge: [%v]", pairs[len(pairs)-1]),
		})
	}

	return sb
}

// ---------------------------------------------------------------------
// Logging Methods (match logger pattern)
// ---------------------------------------------------------------------

// Debug logs the duration at Debug level with message
func (sb *SinceBuilder) Debug(msg string) time.Duration {
	return sb.logAtLevel(lx.LevelDebug, msg)
}

// Info logs the duration at Info level with message
func (sb *SinceBuilder) Info(msg string) time.Duration {
	return sb.logAtLevel(lx.LevelInfo, msg)
}

// Warn logs the duration at Warn level with message
func (sb *SinceBuilder) Warn(msg string) time.Duration {
	return sb.logAtLevel(lx.LevelWarn, msg)
}

// Error logs the duration at Error level with message
func (sb *SinceBuilder) Error(msg string) time.Duration {
	return sb.logAtLevel(lx.LevelError, msg)
}

// Log is an alias for Info (for backward compatibility)
func (sb *SinceBuilder) Log(msg string) time.Duration {
	return sb.Info(msg)
}

// logAtLevel internal method that handles the actual logging
func (sb *SinceBuilder) logAtLevel(level lx.LevelType, msg string) time.Duration {
	// Fast path - don't even compute duration if we're not logging
	if !sb.condition || sb.logger.suspend.Load() || !sb.logger.shouldLog(level) {
		return time.Since(sb.start)
	}

	duration := time.Since(sb.start)

	// Build final fields in this order:
	// 1. Logger context fields (from logger.context)
	// 2. Builder fields (from sb.fields)
	// 3. Duration fields (always last)

	// Pre-allocate with exact capacity
	totalFields := 0
	if sb.logger.context != nil {
		totalFields += len(sb.logger.context)
	}
	if sb.fields != nil {
		totalFields += len(sb.fields)
	}
	totalFields += 2 // duration_ms, duration

	fields := make(lx.Fields, 0, totalFields)

	// Add logger context fields first (preserves order)
	if sb.logger.context != nil {
		fields = append(fields, sb.logger.context...)
	}

	// Add builder fields
	if sb.fields != nil {
		fields = append(fields, sb.fields...)
	}

	// Add duration fields last (so they're visible at the end)
	fields = append(fields,
		lx.Field{Key: "duration_ms", Value: duration.Milliseconds()},
		lx.Field{Key: "duration", Value: duration.String()},
	)

	sb.logger.log(level, lx.ClassTimed, msg, fields, false)
	return duration
}

// ---------------------------------------------------------------------
// Utility Methods
// ---------------------------------------------------------------------

// Reset allows reusing the builder with a new start time
// Zero-allocation - keeps fields slice capacity
func (sb *SinceBuilder) Reset(startTime ...time.Time) *SinceBuilder {
	sb.start = time.Now()
	if len(startTime) > 0 && !startTime[0].IsZero() {
		sb.start = startTime[0]
	}
	sb.condition = true
	if sb.fields != nil {
		sb.fields = sb.fields[:0] // Keep capacity, zero length
	}
	return sb
}

// Elapsed returns the current duration without logging
func (sb *SinceBuilder) Elapsed() time.Duration {
	return time.Since(sb.start)
}
