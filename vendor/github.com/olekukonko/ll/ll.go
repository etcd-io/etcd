package ll

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/olekukonko/cat"
	"github.com/olekukonko/ll/lh"
	"github.com/olekukonko/ll/lx"
)

// Logger manages logging configuration and behavior, encapsulating state such as enablement,
// log level, namespaces, context fields, output style, handler, middleware, and formatting.
// It is thread-safe, using a read-write mutex to protect concurrent access to its fields.
type Logger struct {
	mu              sync.RWMutex  // Guards concurrent access to fields
	enabled         bool          // Determines if logging is enabled
	suspend         atomic.Bool   // uses suspend path for most actions eg. skipping namespace checks
	level           lx.LevelType  // Minimum log level (e.g., Debug, Info, Warn, Error)
	atomicLevel     int32         // Shadow copy of level for lock-free checks
	namespaces      *lx.Namespace // Manages namespace enable/disable states
	currentPath     string        // Current namespace path (e.g., "parent/child")
	context         lx.Fields     // Contextual fields included in all logs
	style           lx.StyleType  // Namespace formatting style (FlatPath or NestedPath)
	handler         lx.Handler    // Output handler for logs (e.g., text, JSON)
	middleware      []Middleware  // Middleware functions to process log entries
	prefix          string        // Prefix prepended to log messages
	indent          int           // Number of double spaces for message indentation
	stackBufferSize int           // Buffer size for capturing stack traces
	separator       string        // Separator for namespace paths (e.g., "/")
	entries         atomic.Int64  // Tracks total log entries sent to handler
	fatalExits      bool
	fatalStack      bool
	labels          atomic.Pointer[[]string]
}

// New creates a new Logger with the given namespace and optional configurations.
// It initializes with defaults: disabled, Debug level, flat namespace style, text handler
// to os.Stdout, and an empty middleware chain. Options (e.g., WithHandler, WithLevel) can
// override defaults. The logger is thread-safe via mutex-protected methods.
// Example:
//
//	logger := New("app", WithHandler(lh.NewTextHandler(os.Stdout))).Enable()
//	logger.Info("Starting application") // Output: [app] INFO: Starting application
func New(namespace string, opts ...Option) *Logger {
	logger := &Logger{
		enabled:         lx.DefaultEnabled,            // Defaults to disabled (false)
		level:           lx.LevelDebug,                // Default minimum log level
		atomicLevel:     int32(lx.LevelDebug),         // Initialize atomic level
		namespaces:      defaultStore,                 // Shared namespace store
		currentPath:     namespace,                    // Initial namespace path
		context:         make(lx.Fields, 0, 10),       // Empty context for fields
		style:           lx.FlatPath,                  // Default namespace style ([parent/child])
		handler:         lh.NewTextHandler(os.Stdout), // Default text output to stdout
		middleware:      make([]Middleware, 0),        // Empty middleware chain
		stackBufferSize: 4096,                         // Default stack trace buffer size
		separator:       lx.Slash,                     // Default namespace separator ("/")
	}

	// Apply provided configuration options
	for _, opt := range opts {
		opt(logger)
	}

	return logger
}

// Apply applies one or more functional options to the default/global logger.
// Useful for late configuration (e.g., after migration, attach VictoriaLogs handler,
// set level, add middleware, etc.) without changing existing New() calls.
//
// Example:
//
//	// In main() or init(), after setting up handler
//	ll.Apply(
//	    ll.Handler(vlBatched),
//	    ll.Level(ll.LevelInfo),
//	    ll.Use(rateLimiterMiddleware),
//	)
//
// Returns the default logger for chaining (if needed).
func (l *Logger) Apply(opts ...Option) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, opt := range opts {
		if opt != nil {
			opt(l)
		}
	}
	return l
}

// AddContext adds one or more key-value pairs to the logger's persistent context.
// These fields will be included in **every** subsequent log message from this logger
// (and its child namespace loggers).
//
// It supports variadic key-value pairs (string key, any value).
// Non-string keys or uneven number of arguments will be safely ignored/logged.
//
// Returns the logger for chaining.
//
// Examples:
//
//	logger.AddContext("user", "alice", "env", "prod")
//	logger.AddContext("request_id", reqID, "trace_id", traceID)
//	logger.AddContext("service", "payment")                    // single pair
func (l *Logger) AddContext(pairs ...any) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.context == nil {
		l.context = make(lx.Fields, 0, len(pairs)/2)
	}

	for i := 0; i < len(pairs)-1; i += 2 {
		if key, ok := pairs[i].(string); ok {
			l.context = append(l.context, lx.Field{Key: key, Value: pairs[i+1]})
		}
	}
	return l
}

// Benchmark logs the duration since a start time at Info level, including "start",
// "end", and "duration" fields. It is thread-safe via Fields and log methods.
// Example:
//
//	logger := New("app").Enable()
//	start := time.Now()
//	logger.Benchmark(start) // Output: [app] INFO: benchmark [start=... end=... duration=...]
func (l *Logger) Benchmark(start time.Time) time.Duration {
	duration := time.Since(start)
	l.Fields(
		"duration_ms", duration.Milliseconds(),
		"duration", duration.String(),
	).Infof("benchmark completed")

	return duration
}

// CanLog checks if a log at the given level would be emitted, considering enablement,
// log level, namespaces, sampling, and rate limits. It is thread-safe via shouldLog.
// Example:
//
//	logger := New("app").Enable().Level(lx.LevelWarn)
//	canLog := logger.CanLog(lx.LevelInfo) // false
func (l *Logger) CanLog(level lx.LevelType) bool {
	return l.shouldLog(level)
}

// Clear removes all middleware functions, resetting the middleware chain to empty.
// It is thread-safe using a write lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable().Use(someMiddleware)
//	logger.Clear()
//	logger.Info("No middleware") // Output: [app] INFO: No middleware
func (l *Logger) Clear() *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.middleware = nil
	return l
}

// Clone creates a new logger with the same configuration and namespace as the parent,
// but with a fresh context map to allow independent field additions. It is thread-safe
// using a read lock.
// Example:
//
//	logger := New("app").Enable().Context(map[string]interface{}{"k": "v"})
//	clone := logger.Clone()
//	clone.Info("Cloned") // Output: [app] INFO: Cloned [k=v]
func (l *Logger) Clone() *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		enabled:         l.enabled,              // Copy enablement state
		level:           l.level,                // Copy log level
		atomicLevel:     l.atomicLevel,          // Copy atomic level
		namespaces:      l.namespaces,           // Share namespace store
		currentPath:     l.currentPath,          // Copy namespace path
		context:         make(lx.Fields, 0, 10), // Fresh context map
		style:           l.style,                // Copy namespace style
		handler:         l.handler,              // Copy output handler
		middleware:      l.middleware,           // Copy middleware chain
		prefix:          l.prefix,               // Copy message prefix
		indent:          l.indent,               // Copy indentation level
		stackBufferSize: l.stackBufferSize,      // Copy stack trace buffer size
		separator:       l.separator,            // Default separator ("/")
		suspend:         l.suspend,
	}
}

// Context creates a new logger with additional contextual fields, preserving existing
// fields and adding new ones. It returns a new logger to avoid mutating the parent and
// is thread-safe using a write lock.
// Example:
//
//	logger := New("app").Enable()
//	logger = logger.Context(map[string]interface{}{"user": "alice"})
//	logger.Info("Action") // Output: [app] INFO: Action [user=alice]
func (l *Logger) Context(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create a new logger with inherited configuration
	newLogger := &Logger{
		enabled:         l.enabled,
		level:           l.level,
		atomicLevel:     l.atomicLevel,
		namespaces:      l.namespaces,
		currentPath:     l.currentPath,
		context:         make(lx.Fields, 0, len(l.context)+len(fields)),
		style:           l.style,
		handler:         l.handler,
		middleware:      l.middleware,
		prefix:          l.prefix,
		indent:          l.indent,
		stackBufferSize: l.stackBufferSize,
		separator:       l.separator,
		suspend:         l.suspend,
		fatalExits:      l.fatalExits,
		fatalStack:      l.fatalStack,
	}

	// Copy parent's context fields (in order)
	newLogger.context = append(newLogger.context, l.context...)

	// Add new fields from map
	for k, v := range fields {
		newLogger.context = append(newLogger.context, lx.Field{Key: k, Value: v})
	}

	return newLogger
}

// Debug logs a message at Debug level, formatting it and delegating to the internal
// log method. It is thread-safe.
// Example:
//
//	logger := New("app").Enable().Level(lx.LevelDebug)
//	logger.Debug("Debugging") // Output: [app] DEBUG: Debugging
func (l *Logger) Debug(args ...any) {
	// check if suspended
	if l.suspend.Load() {
		return
	}

	// Skip logging if Debug level is not enabled
	if !l.shouldLog(lx.LevelDebug) {
		return
	}

	l.log(lx.LevelDebug, lx.ClassText, cat.Space(args...), nil, false)
}

// Debugf logs a formatted message at Debug level, delegating to Debug. It is thread-safe.
// Example:
//
//	logger := New("app").Enable().Level(lx.LevelDebug)
//	logger.Debugf("Debug %s", "message") // Output: [app] DEBUG: Debug message
func (l *Logger) Debugf(format string, args ...any) {
	// check if suspended
	if l.suspend.Load() {
		return
	}

	l.Debug(fmt.Sprintf(format, args...))
}

// Disable deactivates logging, suppressing all logs regardless of level or namespace.
// It is thread-safe using a write lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable().Disable()
//	logger.Info("Ignored") // No output
func (l *Logger) Disable() *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = false
	return l
}

// Dump displays a hex and ASCII representation of a value's binary form, using gob
// encoding or direct conversion. It is useful for inspecting binary data structures.
// Example:
//
//	type Data struct { X int; Y string }
//	logger.Dump(Data{42, "test"}) // Outputs hex/ASCII dump
func (l *Logger) Dump(values ...interface{}) {
	// Iterate over each value to dump
	for _, value := range values {
		// Log value description and type
		l.Infof("Dumping %v (%T)", value, value)
		var by []byte
		var err error

		// Convert value to byte slice based on type
		switch v := value.(type) {
		case []byte:
			by = v
		case string:
			by = []byte(v)
		case float32:
			// Convert float32 to 4-byte big-endian
			buf := make([]byte, 4)
			binary.BigEndian.PutUint32(buf, math.Float32bits(v))
			by = buf
		case float64:
			// Convert float64 to 8-byte big-endian
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, math.Float64bits(v))
			by = buf
		case int, int8, int16, int32, int64:
			// Convert signed integer to 8-byte big-endian
			by = make([]byte, 8)
			binary.BigEndian.PutUint64(by, uint64(reflect.ValueOf(v).Int()))
		case uint, uint8, uint16, uint32, uint64:
			// Convert unsigned integer to 8-byte big-endian
			by = make([]byte, 8)
			binary.BigEndian.PutUint64(by, reflect.ValueOf(v).Uint())
		case io.Reader:
			// Read all bytes from io.Reader
			by, err = io.ReadAll(v)
		default:
			// Fallback to JSON marshaling for complex types
			by, err = json.Marshal(v)
		}

		// Log error if conversion fails
		if err != nil {
			l.Errorf("Dump error: %v", err)
			continue
		}

		// Generate hex/ASCII dump
		n := len(by)
		rowcount := 0
		stop := (n / 8) * 8
		k := 0
		s := strings.Builder{}
		// Process 8-byte rows
		for i := 0; i <= stop; i += 8 {
			k++
			if i+8 < n {
				rowcount = 8
			} else {
				rowcount = min(k*8, n) % 8
			}
			// Write position and hex prefix
			s.WriteString(fmt.Sprintf("pos %02d  hex:  ", i))

			// Write hex values
			for j := 0; j < rowcount; j++ {
				s.WriteString(fmt.Sprintf("%02x  ", by[i+j]))
			}
			// Pad with spaces for alignment
			for j := rowcount; j < 8; j++ {
				s.WriteString(fmt.Sprintf("    "))
			}
			// Write ASCII representation
			s.WriteString(fmt.Sprintf("  '%s'\n", viewString(by[i:(i+rowcount)])))
		}
		// Log the hex/ASCII dump
		l.log(lx.LevelNone, lx.ClassDump, s.String(), nil, false)
	}
}

// Output logs each value as pretty-printed JSON for REST debugging.
// Each value is logged on its own line with [file:line] and a blank line after the header.
// Ideal for inspecting outgoing/incoming REST payloads.
func (l *Logger) Output(values ...interface{}) {
	l.output(2, values...)
}

// mark logs the caller's file and line number along with an optional custom name label for tracing execution flow.
func (l *Logger) output(skip int, values ...interface{}) {
	if !l.shouldLog(lx.LevelInfo) {
		return
	}

	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return
	}
	shortFile := file
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		shortFile = file[idx+1:]
	}

	header := fmt.Sprintf("[%s:%d] JSON:\n", shortFile, line)

	for _, v := range values {
		// Always pretty-print with indent
		b, err := json.MarshalIndent(v, "  ", "  ")
		if err != nil {
			b, _ = json.MarshalIndent(map[string]any{
				"value": fmt.Sprintf("%+v", v),
				"error": err.Error(),
			}, "  ", "  ")
		}
		l.log(lx.LevelInfo, lx.ClassJSON, header+string(b), nil, false)
	}
}

// Inspect logs one or more values in a **developer-friendly, deeply introspective format** at Info level.
// It includes the caller file and line number, and reveals **all fields** — including:
//
//   - Private (unexported) fields → prefixed with `(field)`
//   - Embedded structs (inlined)
//   - Pointers and nil values → shown as `*(field)` or `nil`
//   - Full struct nesting and type information
//
// This method uses `NewInspector` under the hood, which performs **full reflection-based traversal**.
// It is **not** meant for production logging or REST APIs — use `Output` for that.
//
// Ideal for:
//   - Debugging complex internal state
//   - Inspecting structs with private fields
//   - Understanding struct embedding and pointer behavior
func (l *Logger) Inspect(values ...interface{}) {
	o := NewInspector(l)
	o.Log(2, values...)
}

// Enable activates logging, allowing logs to be emitted if other conditions (e.g., level,
// namespace) are met. It is thread-safe using a write lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable()
//	logger.Info("Started") // Output: [app] INFO: Started
func (l *Logger) Enable() *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = true
	return l
}

// Enabled checks if the logger is enabled for logging. It is thread-safe using a read lock.
// Example:
//
//	logger := New("app").Enable()
//	if logger.Enabled() {
//	    logger.Info("Logging is enabled") // Output: [app] INFO: Logging is enabled
//	}
func (l *Logger) Enabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// Err adds one or more errors to the logger’s context and logs them at Error level.
// Non-nil errors are stored in the "error" context field (single error or slice) and
// logged as a concatenated string (e.g., "failed 1; failed 2"). It is thread-safe and
// returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable()
//	err1 := errors.New("failed 1")
//	err2 := errors.New("failed 2")
//	logger.Err(err1, err2).Info("Error occurred")
//	// Output: [app] ERROR: failed 1; failed 2
//	//         [app] INFO: Error occurred [error=[failed 1 failed 2]]
func (l *Logger) Err(errs ...error) {
	// Skip logging if Error level is not enabled
	if !l.shouldLog(lx.LevelError) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Initialize context slice if nil
	if l.context == nil {
		l.context = make(lx.Fields, 0, 4)
	}

	// Collect non-nil errors and build log message
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
			// Store single error directly
			l.context = append(l.context, lx.Field{Key: "error", Value: nonNilErrors[0]})
		} else {
			// Store slice of errors
			l.context = append(l.context, lx.Field{Key: "error", Value: nonNilErrors})
		}
		// Log concatenated error messages
		l.log(lx.LevelError, lx.ClassText, builder.String(), nil, false)
	}
}

// Error logs a message at Error level, formatting it and delegating to the internal
// log method. It is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Error("Error occurred") // Output: [app] ERROR: Error occurred
func (l *Logger) Error(args ...any) {
	// check if suspended
	if l.suspend.Load() {
		return
	}

	// Skip logging if Error level is not enabled
	if !l.shouldLog(lx.LevelError) {
		return
	}
	l.log(lx.LevelError, lx.ClassText, cat.Space(args...), nil, false)
}

// Errorf logs a formatted message at Error level, delegating to Error. It is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Errorf("Error %s", "occurred") // Output: [app] ERROR: Error occurred
func (l *Logger) Errorf(format string, args ...any) {
	// check if suspended
	if l.suspend.Load() {
		return
	}

	l.Error(fmt.Errorf(format, args...))
}

// Fatal logs a message at Error level with a stack trace and exits the program with
// exit code 1. It is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Fatal("Fatal error") // Output: [app] ERROR: Fatal error [stack=...], then exits
func (l *Logger) Fatal(args ...any) {
	// check if suspended
	if l.suspend.Load() {
		return
	}

	// Exit immediately if Error level is not enabled
	if !l.shouldLog(lx.LevelError) {
		os.Exit(1)
	}

	l.log(lx.LevelFatal, lx.ClassText, cat.Space(args...), nil, l.fatalStack)
	if l.fatalExits {
		os.Exit(1)
	}
}

// Fatalf logs a formatted message at Error level with a stack trace and exits the program.
// It delegates to Fatal and is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Fatalf("Fatal %s", "error") // Output: [app] ERROR: Fatal error [stack=...], then exits
func (l *Logger) Fatalf(format string, args ...any) {
	// check if suspended
	if l.suspend.Load() {
		return
	}

	l.Fatal(fmt.Sprintf(format, args...))
}

// Field starts a fluent chain for adding fields from a map, creating a FieldBuilder
// for type-safe field addition. It is thread-safe via the FieldBuilder’s logger.
// Example:
//
//	logger := New("app").Enable()
//	logger.Field(map[string]interface{}{"user": "alice"}).Info("Action") // Output: [app] INFO: Action [user=alice]
func (l *Logger) Field(fields map[string]interface{}) *FieldBuilder {
	fb := &FieldBuilder{logger: l, fields: make(lx.Fields, 0, len(fields))}

	if l.suspend.Load() {
		return fb
	}

	// Copy fields from input map to FieldBuilder (preserving map iteration order)
	for k, v := range fields {
		fb.fields = append(fb.fields, lx.Field{Key: k, Value: v})
	}
	return fb
}

// Fields starts a fluent chain for adding fields using variadic key-value pairs.
// It creates a FieldBuilder to attach fields, handling non-string keys or uneven pairs by
// adding an error field. Thread-safe via the FieldBuilder's logger.
// Example:
//
//	logger.Fields("user", "alice").Info("Action") // Output: [app] INFO: Action [user=alice]
func (l *Logger) Fields(pairs ...any) *FieldBuilder {
	fb := &FieldBuilder{logger: l, fields: make(lx.Fields, 0, len(pairs)/2)}

	if l.suspend.Load() {
		return fb
	}

	// Process key-value pairs
	for i := 0; i < len(pairs)-1; i += 2 {
		if key, ok := pairs[i].(string); ok {
			fb.fields = append(fb.fields, lx.Field{Key: key, Value: pairs[i+1]})
		} else {
			// Log error for non-string keys
			fb.fields = append(fb.fields, lx.Field{
				Key:   "error",
				Value: fmt.Errorf("non-string key in Fields: %v", pairs[i]),
			})
		}
	}
	// Log error for uneven pairs
	if len(pairs)%2 != 0 {
		fb.fields = append(fb.fields, lx.Field{
			Key:   "error",
			Value: fmt.Errorf("uneven key-value pairs in Fields: [%v]", pairs[len(pairs)-1]),
		})
	}
	return fb
}

// GetContext returns the logger's context map of persistent key-value fields. It is
// thread-safe using a read lock.
// Example:
//
//	logger := New("app").AddContext("user", "alice")
//	ctx := logger.GetContext() // Returns map[string]interface{}{"user": "alice"}
func (l *Logger) GetContext() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()
	// Convert slice to map for backward compatibility
	contextMap := make(map[string]interface{}, len(l.context))
	for _, pair := range l.context {
		contextMap[pair.Key] = pair.Value
	}
	return contextMap
}

// GetHandler returns the logger's current handler for customization or inspection.
// The returned handler should not be modified concurrently with logger operations.
// Example:
//
//	logger := New("app")
//	handler := logger.GetHandler() // Returns the current handler (e.g., TextHandler)
func (l *Logger) GetHandler() lx.Handler {
	return l.handler
}

// GetLevel returns the minimum log level for the logger. It is thread-safe using a read lock.
// Example:
//
//	logger := New("app").Level(lx.LevelWarn)
//	if logger.GetLevel() == lx.LevelWarn {
//	    logger.Warn("Warning level set") // Output: [app] WARN: Warning level set
//	}
func (l *Logger) GetLevel() lx.LevelType {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// GetPath returns the logger's current namespace path. It is thread-safe using a read lock.
// Example:
//
//	logger := New("app").Namespace("sub")
//	path := logger.GetPath() // Returns "app/sub"
func (l *Logger) GetPath() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.currentPath
}

// GetSeparator returns the logger's namespace separator (e.g., "/"). It is thread-safe
// using a read lock.
// Example:
//
//	logger := New("app").Separator(".")
//	sep := logger.GetSeparator() // Returns "."
func (l *Logger) GetSeparator() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.separator
}

// GetStyle returns the logger's namespace formatting style (FlatPath or NestedPath).
// It is thread-safe using a read lock.
// Example:
//
//	logger := New("app").Style(lx.NestedPath)
//	if logger.GetStyle() == lx.NestedPath {
//	    logger.Info("Nested style") // Output: [app]: INFO: Nested style
//	}
func (l *Logger) GetStyle() lx.StyleType {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.style
}

// Handler sets the handler for processing log entries, configuring the output destination
// and format (e.g., text, JSON). It is thread-safe using a write lock and returns the
// logger for chaining.
// Example:
//
//	logger := New("app").Enable().Handler(lh.NewTextHandler(os.Stdout))
//	logger.Info("Log") // Output: [app] INFO: Log
func (l *Logger) Handler(handler lx.Handler) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handler = handler
	return l
}

// Indent sets the indentation level for log messages, adding two spaces per level. It is
// thread-safe using a write lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable().Indent(2)
//	logger.Info("Indented") // Output: [app] INFO:     Indented
func (l *Logger) Indent(depth int) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.indent = depth
	return l
}

// Info logs a message at Info level, formatting it and delegating to the internal log
// method. It is thread-safe.
// Example:
//
//	logger := New("app").Enable().Style(lx.NestedPath)
//	logger.Info("Started") // Output: [app]: INFO: Started
func (l *Logger) Info(args ...any) {
	if l.suspend.Load() {
		return
	}

	if !l.shouldLog(lx.LevelInfo) {
		return
	}

	l.log(lx.LevelInfo, lx.ClassText, cat.Space(args...), nil, false)
}

// Infof logs a formatted message at Info level, delegating to Info. It is thread-safe.
// Example:
//
//	logger := New("app").Enable().Style(lx.NestedPath)
//	logger.Infof("Started %s", "now") // Output: [app]: INFO: Started now
func (l *Logger) Infof(format string, args ...any) {
	if l.suspend.Load() {
		return
	}

	l.Info(fmt.Sprintf(format, args...))
}

// Len returns the total number of log entries sent to the handler, using atomic operations
// for thread safety.
// Example:
//
//	logger := New("app").Enable()
//	logger.Info("Test")
//	count := logger.Len() // Returns 1
func (l *Logger) Len() int64 {
	return l.entries.Load()
}

// Labels temporarily attaches one or more label names to the logger for the next log entry.
// Labels are typically used for metrics, benchmarking, tracing, or categorizing logs in a structured way.
//
// The labels are stored atomically and intended to be short-lived, applying only to the next
// log operation (or until overwritten by a subsequent call to Labels). Multiple labels can
// be provided as separate string arguments.
//
// Example usage:
//
//	logger := New("app").Enable()
//
//	// Add labels for a specific operation
//	logger.Labels("load_users", "process_orders").Measure(func() {
//	    // ... perform work ...
//	}, func() {
//	    // ... optional callback ...
//	})
func (l *Logger) Labels(names ...string) *Logger {
	l.labels.Store(&names) // store temporarily
	return l
}

// Level sets the minimum log level, ignoring messages below it. It is thread-safe using
// a write lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable().Level(lx.LevelWarn)
//	logger.Info("Ignored") // No output
//	logger.Warn("Logged") // Output: [app] WARN: Logged
func (l *Logger) Level(level lx.LevelType) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	atomic.StoreInt32(&l.atomicLevel, int32(level))
	return l
}

// Line adds vertical spacing (newlines) to the log output, defaulting to 1 if no arguments
// are provided. Multiple values are summed for total lines. It is thread-safe and returns
// the logger for chaining.
// Example:
//
//	logger := New("app").Enable()
//	logger.Line(2).Info("After 2 newlines") // Adds 2 blank lines before logging
//	logger.Line().Error("After 1 newline")  // Defaults to 1
func (l *Logger) Line(lines ...int) *Logger {
	line := 1 // Default to 1 newline
	if len(lines) > 0 {
		line = 0
		// Sum all provided line counts
		for _, n := range lines {
			line += n
		}
		// Ensure at least 1 line
		if line < 1 {
			line = 1
		}
	}
	l.log(lx.LevelNone, lx.ClassRaw, strings.Repeat(lx.Newline, line), nil, false)
	return l
}

// Mark logs the current file and line number where it's called, without any additional debug information.
// It's useful for tracing execution flow without the verbosity of Dbg.
// Example:
//
//	logger.Mark() // *MARK*: [file.go:123]
func (l *Logger) Mark(name ...string) {
	l.mark(2, name...)
}

// mark logs the caller's file and line number along with an optional custom name label for tracing execution flow.
func (l *Logger) mark(skip int, names ...string) {
	// Skip logging if Info level is not enabled
	if !l.shouldLog(lx.LevelInfo) {
		return
	}

	// Get caller information (file, line)
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		l.log(lx.LevelError, lx.ClassText, "Mark: Unable to parse runtime caller", nil, false)
		return
	}

	// Extract just the filename (without full path)
	shortFile := file
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		shortFile = file[idx+1:]
	}

	name := strings.Join(names, l.separator)
	if name == "" {
		name = "MARK"
	}

	// Format as [filename:line]
	out := fmt.Sprintf("[*%s*]: [%s:%d]\n", name, shortFile, line)
	l.log(lx.LevelInfo, lx.ClassRaw, out, nil, false)
}

// Namespace creates a child logger with a sub-namespace appended to the current path,
// inheriting the parent’s configuration but with an independent context. It is thread-safe
// using a read lock.
// Example:
//
//	parent := New("parent").Enable()
//	child := parent.Namespace("child")
//	child.Info("Child log") // Output: [parent/child] INFO: Child log
func (l *Logger) Namespace(name string) *Logger {
	if l.suspend.Load() {
		return l
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	// Construct full namespace path
	fullPath := name
	if l.currentPath != "" {
		fullPath = l.currentPath + l.separator + name
	}

	// Create child logger with inherited configuration
	return &Logger{
		enabled:         l.enabled,
		level:           l.level,
		atomicLevel:     l.atomicLevel,
		namespaces:      l.namespaces,
		currentPath:     fullPath,
		context:         make(lx.Fields, 0, 10),
		style:           l.style,
		handler:         l.handler,
		middleware:      l.middleware,
		prefix:          l.prefix,
		indent:          l.indent,
		stackBufferSize: l.stackBufferSize,
		separator:       l.separator,
		suspend:         l.suspend,
	}
}

// NamespaceDisable disables logging for a namespace and its children, invalidating the
// namespace cache. It is thread-safe via lx.Namespace’s sync.Map and returns the logger
// for chaining.
// Example:
//
//	logger := New("parent").Enable().NamespaceDisable("parent/child")
//	logger.Namespace("child").Info("Ignored") // No output
func (l *Logger) NamespaceDisable(relativePath string) *Logger {
	l.mu.RLock()
	fullPath := l.joinPath(l.currentPath, relativePath)
	l.mu.RUnlock()

	// Disable namespace in shared store
	l.namespaces.Set(fullPath, false)
	return l
}

// NamespaceEnable enables logging for a namespace and its children, invalidating the
// namespace cache. It is thread-safe via lx.Namespace’s sync.Map and returns the logger
// for chaining.
// Example:
//
//	logger := New("parent").Enable().NamespaceEnable("parent/child")
//	logger.Namespace("child").Info("Log") // Output: [parent/child] INFO: Log
func (l *Logger) NamespaceEnable(relativePath string) *Logger {
	l.mu.RLock()
	fullPath := l.joinPath(l.currentPath, relativePath)
	l.mu.RUnlock()

	// Enable namespace in shared store
	l.namespaces.Set(fullPath, true)
	return l
}

// NamespaceEnabled checks if a namespace is enabled, considering parent namespaces and
// caching results for performance. It is thread-safe using a read lock.
// Example:
//
//	logger := New("parent").Enable().NamespaceDisable("parent/child")
//	enabled := logger.NamespaceEnabled("parent/child") // false
func (l *Logger) NamespaceEnabled(relativePath string) bool {
	l.mu.RLock()
	fullPath := l.joinPath(l.currentPath, relativePath)
	separator := l.separator
	if separator == "" {
		separator = lx.Slash
	}
	instanceEnabled := l.enabled
	l.mu.RUnlock()

	// Handle root path case
	if fullPath == "" && relativePath == "" {
		return instanceEnabled
	}

	if fullPath != "" {
		// Check namespace rules
		isEnabledByNSRule, isDisabledByNSRule := l.namespaces.Enabled(fullPath, separator)
		if isDisabledByNSRule {
			return false
		}
		if isEnabledByNSRule {
			return true
		}
	}
	// Fall back to logger's enabled state
	return instanceEnabled
}

// Panic logs a message at Error level with a stack trace and triggers a panic. It is
// thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Panic("Panic error") // Output: [app] ERROR: Panic error [stack=...], then panics
func (l *Logger) Panic(args ...any) {
	// Build message by concatenating arguments with spaces
	msg := cat.Space(args...)

	if l.suspend.Load() {
		panic(msg)
	}

	// Panic immediately if Error level is not enabled
	if !l.shouldLog(lx.LevelError) {
		panic(msg)
	}

	l.log(lx.LevelFatal, lx.ClassText, msg, nil, true)
	panic(msg)
}

// Panicf logs a formatted message at Error level with a stack trace and triggers a panic.
// It delegates to Panic and is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Panicf("Panic %s", "error") // Output: [app] ERROR: Panic error [stack=...], then panics
func (l *Logger) Panicf(format string, args ...any) {
	l.Panic(fmt.Sprintf(format, args...))
}

// Prefix sets a prefix prepended to all log messages. It is thread-safe using a write
// lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable().Prefix("APP: ")
//	logger.Info("Started") // Output: [app] INFO: APP: Started
func (l *Logger) Prefix(prefix string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
	return l
}

// Print logs a message at Info level without format specifiers, minimizing allocations
// by concatenating arguments with spaces. It is thread-safe via the log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.Print("message", "value") // Output: [app] INFO: message value
func (l *Logger) Print(args ...any) {
	if l.suspend.Load() {
		return
	}

	// Skip logging if Info level is not enabled
	if !l.shouldLog(lx.LevelInfo) {
		return
	}
	l.log(lx.LevelNone, lx.ClassRaw, cat.Space(args...), nil, false)
}

// Println logs a message at Info level without format specifiers, minimizing allocations
// by concatenating arguments with spaces. It is thread-safe via the log method.
// Example:
//
//	logger := New("app").Enable()
//	logger.Println("message", "value") // Output: [app] INFO: message value
func (l *Logger) Println(args ...any) {
	if l.suspend.Load() {
		return
	}

	// Skip logging if Info level is not enabled
	if !l.shouldLog(lx.LevelInfo) {
		return
	}
	l.log(lx.LevelNone, lx.ClassRaw, cat.SuffixWith(lx.Space, lx.Newline, args...), nil, false)
}

// Printf logs a formatted message at Info level, delegating to Print. It is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Printf("Message %s", "value") // Output: [app] INFO: Message value
func (l *Logger) Printf(format string, args ...any) {
	if l.suspend.Load() {
		return
	}

	l.Print(fmt.Sprintf(format, args...))
}

// Remove removes middleware by the reference returned from Use, delegating to the
// Middleware’s Remove method for thread-safe removal.
// Example:
//
//	logger := New("app").Enable()
//	mw := logger.Use(someMiddleware)
//	logger.Remove(mw) // Removes middleware
func (l *Logger) Remove(m *Middleware) {
	m.Remove()
}

// Resume reactivates logging for the current logger after it has been suspended.
// It clears the suspend flag, allowing logs to be emitted if other conditions (e.g., level, namespace)
// are met. Thread-safe with a write lock. Returns the logger for method chaining.
// Example:
//
//	logger := New("app").Enable().Suspend()
//	logger.Resume()
//	logger.Info("Resumed") // Output: [app] INFO: Resumed
func (l *Logger) Resume() *Logger {
	l.suspend.Store(false)
	return l
}

// Separator sets the namespace separator for grouping namespaces and log entries (e.g., "/" or ".").
// It is thread-safe using a write lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Separator(".")
//	logger.Namespace("sub").Info("Log") // Output: [app.sub] INFO: Log
func (l *Logger) Separator(separator string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.separator = separator
	return l
}

// Suspend temporarily deactivates logging for the current logger.
// It sets the suspend flag, suppressing all logs regardless of level or namespace until resumed.
// Thread-safe with a write lock. Returns the logger for method chaining.
// Example:
//
//	logger := New("app").Enable()
//	logger.Suspend()
//	logger.Info("Ignored") // No output
func (l *Logger) Suspend() *Logger {
	l.suspend.Store(true)
	return l
}

// Suspended returns whether the logger is currently suspended.
// It provides thread-safe read access to the suspend flag using a write lock.
// Example:
//
//	logger := New("app").Enable().Suspend()
//	if logger.Suspended() {
//	    fmt.Println("Logging is suspended") // Prints message
//	}
func (l *Logger) Suspended() bool {
	return l.suspend.Load()
}

// Stack logs messages at Error level with a stack trace for each provided argument.
// It is thread-safe and skips logging if Debug level is not enabled.
// Example:
//
//	logger := New("app").Enable()
//	logger.Stack("Critical error") // Output: [app] ERROR: Critical error [stack=...]
func (l *Logger) Stack(args ...any) {
	if l.suspend.Load() {
		return
	}

	// Skip logging if Debug level is not enabled
	if !l.shouldLog(lx.LevelDebug) {
		return
	}

	for _, arg := range args {
		l.log(lx.LevelError, lx.ClassText, cat.Concat(arg), nil, true)
	}
}

// Stackf logs a formatted message at Error level with a stack trace, delegating to Stack.
// It is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Stackf("Critical %s", "error") // Output: [app] ERROR: Critical error [stack=...]
func (l *Logger) Stackf(format string, args ...any) {
	if l.suspend.Load() {
		return
	}

	l.Stack(fmt.Sprintf(format, args...))
}

// StackSize sets the buffer size for stack trace capture in Stack, Fatal, and Panic methods.
// It is thread-safe using a write lock and returns the logger for chaining.
// Example:
//
//	logger := New("app").Enable().StackSize(65536)
//	logger.Stack("Error") // Captures up to 64KB stack trace
func (l *Logger) StackSize(size int) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	if size > 0 {
		l.stackBufferSize = size
	}
	return l
}

// Style sets the namespace formatting style (FlatPath or NestedPath). FlatPath uses
// [parent/child], while NestedPath uses [parent]→[child]. It is thread-safe using a write
// lock and returns the logger for chaining.
// Example:
//
//	logger := New("parent/child").Enable().Style(lx.NestedPath)
//	logger.Info("Log") // Output: [parent]→[child]: INFO: Log
func (l *Logger) Style(style lx.StyleType) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.style = style
	return l
}

// Timestamped enables or disables timestamp logging for the logger and optionally sets the timestamp format.
// It is thread-safe, using a write lock to ensure safe concurrent access.
// If the logger's handler supports the lx.Timestamper interface, the timestamp settings are applied.
// The method returns the logger instance to support method chaining.
// Parameters:
//
//	enable: Boolean to enable or disable timestamp logging
//	format: Optional string(s) to specify the timestamp format
func (l *Logger) Timestamped(enable bool, format ...string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	if h, ok := l.handler.(lx.Timestamper); ok {
		h.Timestamped(enable, format...)
	}
	return l
}

// Toggle enables or disables the logger based on the provided boolean value and returns the updated logger instance.
func (l *Logger) Toggle(v bool) *Logger {
	if v {
		l.Resume()
		return l.Enable()
	}

	l.Suspend()
	return l.Disable()
}

// Use adds a middleware function to process log entries before they are handled, returning
// a Middleware handle for removal. Middleware returning a non-nil error stops the log.
// It is thread-safe using a write lock.
// Example:
//
//	logger := New("app").Enable()
//	mw := logger.Use(ll.FuncMiddleware(func(e *lx.Entry) error {
//	    if e.Level < lx.LevelWarn {
//	        return fmt.Errorf("level too low")
//	    }
//	    return nil
//	}))
//	logger.Info("Ignored") // No output
//	mw.Remove()
//	logger.Info("Now logged") // Output: [app] INFO: Now logged
func (l *Logger) Use(fn lx.Handler) *Middleware {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Assign a unique ID to the middleware
	id := len(l.middleware) + 1
	// Append middleware to the chain
	l.middleware = append(l.middleware, Middleware{id: id, fn: fn})

	return &Middleware{
		logger: l,
		id:     id,
	}
}

// Warn logs a message at Warn level, formatting it and delegating to the internal log
// method. It is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Warn("Warning") // Output: [app] WARN: Warning
func (l *Logger) Warn(args ...any) {
	if l.suspend.Load() {
		return
	}

	// Skip logging if Warn level is not enabled
	if !l.shouldLog(lx.LevelWarn) {
		return
	}

	l.log(lx.LevelWarn, lx.ClassText, cat.Space(args...), nil, false)
}

// Warnf logs a formatted message at Warn level, delegating to Warn. It is thread-safe.
// Example:
//
//	logger := New("app").Enable()
//	logger.Warnf("Warning %s", "issued") // Output: [app] WARN: Warning issued
func (l *Logger) Warnf(format string, args ...any) {
	if l.suspend.Load() {
		return
	}

	l.Warn(fmt.Sprintf(format, args...))
}

// joinPath joins a base path and a relative path using the logger's separator, handling
// empty base or relative paths. It is used internally for namespace path construction.
// Example (internal usage):
//
//	logger.joinPath("parent", "child") // Returns "parent/child"
func (l *Logger) joinPath(base, relative string) string {
	if base == "" {
		return relative
	}
	if relative == "" {
		return base
	}
	separator := l.separator
	if separator == "" {
		separator = lx.Slash // Default separator
	}
	return base + separator + relative
}

// log is the internal method for processing a log entry, applying rate limiting, sampling,
// middleware, and context before passing to the handler. Middleware returning a non-nil
// error stops the log. It is thread-safe with read/write locks for configuration and stack
// trace buffer.
// Example (internal usage):
//
//	logger := New("app").Enable()
//	logger.Info("Test") // Calls log(lx.LevelInfo, "Test", nil, false)
func (l *Logger) log(level lx.LevelType, class lx.ClassType, msg string, fields lx.Fields, withStack bool) {
	// Skip logging if level is not enabled
	if !l.shouldLog(level) {
		return
	}

	var stack []byte

	// Capture stack trace if requested
	if withStack {
		l.mu.RLock()
		buf := make([]byte, l.stackBufferSize)
		l.mu.RUnlock()
		n := runtime.Stack(buf, false)
		stack = buf[:n]
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	// Apply prefix and indentation to the message
	var builder strings.Builder
	if l.indent > 0 {
		builder.WriteString(strings.Repeat(lx.DoubleSpace, l.indent))
	}
	if l.prefix != "" {
		builder.WriteString(l.prefix)
	}
	builder.WriteString(msg)
	finalMsg := builder.String()

	// Create combined fields slice - THIS PRESERVES ORDER!
	// Optimized slice allocation
	var combinedFields lx.Fields
	if len(l.context) == 0 {
		combinedFields = fields
	} else if len(fields) == 0 {
		combinedFields = l.context
	} else {
		combinedFields = make(lx.Fields, 0, len(l.context)+len(fields))
		// Add context fields first (in order)
		combinedFields = append(combinedFields, l.context...)
		// Add immediate fields
		combinedFields = append(combinedFields, fields...)
	}

	// Create log entry with ordered fields
	entry := &lx.Entry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   finalMsg,
		Namespace: l.currentPath,
		Fields:    combinedFields, // Already ordered!
		Style:     l.style,
		Class:     class,
		Stack:     stack,
	}

	// Apply middleware, stopping if any returns an error
	for _, mw := range l.middleware {
		if err := mw.fn.Handle(entry); err != nil {
			return
		}
	}

	// Pass to handler if set
	if l.handler != nil {
		_ = l.handler.Handle(entry)
		l.entries.Add(1)
	}
}

// shouldLog determines if a log should be emitted based on enabled state, level, namespaces,
// sampling, and rate limits, caching namespace results for performance. It is thread-safe
// with a read lock.
// Example (internal usage):
//
//	logger := New("app").Enable().Level(lx.LevelWarn)
//	if logger.shouldLog(lx.LevelInfo) { // false
//	    // Log would be skipped
//	}
func (l *Logger) shouldLog(level lx.LevelType) bool {
	// Skip if global logging system is inactive
	if !Active() {
		return false
	}

	//  check for suspend mode
	if l.suspend.Load() {
		return false
	}

	// Atomic fast path: read level without lock
	if level > lx.LevelType(atomic.LoadInt32(&l.atomicLevel)) {
		return false
	}

	separator := l.separator
	if separator == "" {
		separator = lx.Slash
	}

	// Check namespace rules if path is set
	if l.currentPath != "" {
		isEnabledByNSRule, isDisabledByNSRule := l.namespaces.Enabled(l.currentPath, separator)
		if isDisabledByNSRule {
			return false
		}
		if isEnabledByNSRule {
			return true
		}
	}

	// Fall back to logger's enabled state
	if !l.enabled {
		return false
	}

	return true
}
