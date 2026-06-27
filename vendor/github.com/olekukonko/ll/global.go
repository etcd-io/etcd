package ll

import (
	"sync/atomic"
	"time"

	"github.com/olekukonko/ll/lx"
)

// defaultLogger is the global logger instance for package-level logging functions.
// It provides a shared logger for convenience, allowing logging without explicitly creating
// a logger instance. The logger is initialized with default settings: enabled, Debug level,
// flat namespace style, and a text handler to os.Stdout. It is thread-safe due to the Logger
// struct’s mutex.
var defaultLogger = New("")

// Handler sets the handler for the default logger.
// It configures the output destination and format (e.g., text, JSON) for logs emitted by
// defaultLogger. Returns the default logger for method chaining, enabling fluent configuration.
// Example:
//
//	ll.Handler(lh.NewJSONHandler(os.Stdout)).Enable()
//	ll.Info("Started") // Output: {"level":"INFO","message":"Started"}
func Handler(handler lx.Handler) *Logger {
	return defaultLogger.Handler(handler)
}

// Level sets the minimum log level for the default logger.
// It determines which log messages (Debug, Info, Warn, Error) are emitted. Messages below
// the specified level are ignored. Returns the default logger for method chaining.
// Example:
//
//	ll.Level(lx.LevelWarn)
//	ll.Info("Ignored") // No output
//	ll.Warn("Logged")  // Output: [] WARN: Logged
func Level(level lx.LevelType) *Logger {
	return defaultLogger.Level(level)
}

// Style sets the namespace style for the default logger.
// It controls how namespace paths are formatted in logs (FlatPath: [parent/child],
// NestedPath: [parent]→[child]). Returns the default logger for method chaining.
// Example:
//
//	ll.Style(lx.NestedPath)
//	ll.Info("Test") // Output: []: INFO: Test
func Style(style lx.StyleType) *Logger {
	return defaultLogger.Style(style)
}

// NamespaceEnable enables logging for a namespace and its children using the default logger.
// It activates logging for the specified namespace path (e.g., "app/db") and all its
// descendants. Returns the default logger for method chaining. Thread-safe via the Logger’s mutex.
// Example:
//
//	ll.NamespaceEnable("app/db")
//	ll.Clone().Namespace("db").Info("Query") // Output: [app/db] INFO: Query
func NamespaceEnable(path string) *Logger {
	return defaultLogger.NamespaceEnable(path)
}

// NamespaceDisable disables logging for a namespace and its children using the default logger.
// It suppresses logging for the specified namespace path and all its descendants. Returns
// the default logger for method chaining. Thread-safe via the Logger’s mutex.
// Example:
//
//	ll.NamespaceDisable("app/db")
//	ll.Clone().Namespace("db").Info("Query") // No output
func NamespaceDisable(path string) *Logger {
	return defaultLogger.NamespaceDisable(path)
}

// Namespace creates a child logger with a sub-namespace appended to the current path.
// The child inherits the default logger’s configuration but has an independent context.
// Thread-safe with read lock. Returns the new logger for further configuration or logging.
// Example:
//
//	logger := ll.Namespace("app")
//	logger.Info("Started") // Output: [app] INFO: Started
func Namespace(name string) *Logger {
	return defaultLogger.Namespace(name)
}

// Info logs a message at Info level with variadic arguments using the default logger.
// It concatenates the arguments with spaces and delegates to defaultLogger’s Info method.
// Thread-safe via the Logger’s log method.
// Example:
//
//	ll.Info("Service", "started") // Output: [] INFO: Service started
func Info(args ...any) {
	defaultLogger.Info(args...)
}

// Infof logs a message at Info level with a format string using the default logger.
// It formats the message using the provided format string and arguments, then delegates to
// defaultLogger’s Infof method. Thread-safe via the Logger’s log method.
// Example:
//
//	ll.Infof("Service %s", "started") // Output: [] INFO: Service started
func Infof(format string, args ...any) {
	defaultLogger.Infof(format, args...)
}

// Debug logs a message at Debug level with variadic arguments using the default logger.
// It concatenates the arguments with spaces and delegates to defaultLogger’s Debug method.
// Used for debugging information, typically disabled in production. Thread-safe.
// Example:
//
//	ll.Level(lx.LevelDebug)
//	ll.Debug("Debugging", "mode") // Output: [] DEBUG: Debugging mode
func Debug(args ...any) {
	defaultLogger.Debug(args...)
}

// Debugf logs a message at Debug level with a format string using the default logger.
// It formats the message and delegates to defaultLogger’s Debugf method. Used for debugging
// information, typically disabled in production. Thread-safe.
// Example:
//
//	ll.Level(lx.LevelDebug)
//	ll.Debugf("Debug %s", "mode") // Output: [] DEBUG: Debug mode
func Debugf(format string, args ...any) {
	defaultLogger.Debugf(format, args...)
}

// Warn logs a message at Warn level with variadic arguments using the default logger.
// It concatenates the arguments with spaces and delegates to defaultLogger’s Warn method.
// Used for warning conditions that do not halt execution. Thread-safe.
// Example:
//
//	ll.Warn("Low", "memory") // Output: [] WARN: Low memory
func Warn(args ...any) {
	defaultLogger.Warn(args...)
}

// Warnf logs a message at Warn level with a format string using the default logger.
// It formats the message and delegates to defaultLogger’s Warnf method. Used for warning
// conditions that do not halt execution. Thread-safe.
// Example:
//
//	ll.Warnf("Low %s", "memory") // Output: [] WARN: Low memory
func Warnf(format string, args ...any) {
	defaultLogger.Warnf(format, args...)
}

// Error logs a message at Error level with variadic arguments using the default logger.
// It concatenates the arguments with spaces and delegates to defaultLogger’s Error method.
// Used for error conditions requiring attention. Thread-safe.
// Example:
//
//	ll.Error("Database", "failure") // Output: [] ERROR: Database failure
func Error(args ...any) {
	defaultLogger.Error(args...)
}

// Errorf logs a message at Error level with a format string using the default logger.
// It formats the message and delegates to defaultLogger’s Errorf method. Used for error
// conditions requiring attention. Thread-safe.
// Example:
//
//	ll.Errorf("Database %s", "failure") // Output: [] ERROR: Database failure
func Errorf(format string, args ...any) {
	defaultLogger.Errorf(format, args...)
}

// Stack logs a message at Error level with a stack trace and variadic arguments using the default logger.
// It concatenates the arguments with spaces and delegates to defaultLogger’s Stack method.
// Thread-safe.
// Example:
//
//	ll.Stack("Critical", "error") // Output: [] ERROR: Critical error [stack=...]
func Stack(args ...any) {
	defaultLogger.Stack(args...)
}

// Stackf logs a message at Error level with a stack trace and a format string using the default logger.
// It formats the message and delegates to defaultLogger’s Stackf method. Thread-safe.
// Example:
//
//	ll.Stackf("Critical %s", "error") // Output: [] ERROR: Critical error [stack=...]
func Stackf(format string, args ...any) {
	defaultLogger.Stackf(format, args...)
}

// Fatal logs a message at Error level with a stack trace and variadic arguments using the default logger,
// then exits. It concatenates the arguments with spaces, logs with a stack trace, and terminates
// with exit code 1. Thread-safe.
// Example:
//
//	ll.Fatal("Fatal", "error") // Output: [] ERROR: Fatal error [stack=...], then exits
func Fatal(args ...any) {
	defaultLogger.Fatal(args...)
}

// Fatalf logs a formatted message at Error level with a stack trace using the default logger,
// then exits. It formats the message, logs with a stack trace, and terminates with exit code 1.
// Thread-safe.
// Example:
//
//	ll.Fatalf("Fatal %s", "error") // Output: [] ERROR: Fatal error [stack=...], then exits
func Fatalf(format string, args ...any) {
	defaultLogger.Fatalf(format, args...)
}

// Panic logs a message at Error level with a stack trace and variadic arguments using the default logger,
// then panics. It concatenates the arguments with spaces, logs with a stack trace, and triggers a panic.
// Thread-safe.
// Example:
//
//	ll.Panic("Panic", "error") // Output: [] ERROR: Panic error [stack=...], then panics
func Panic(args ...any) {
	defaultLogger.Panic(args...)
}

// Panicf logs a formatted message at Error level with a stack trace using the default logger,
// then panics. It formats the message, logs with a stack trace, and triggers a panic. Thread-safe.
// Example:
//
//	ll.Panicf("Panic %s", "error") // Output: [] ERROR: Panic error [stack=...], then panics
func Panicf(format string, args ...any) {
	defaultLogger.Panicf(format, args...)
}

// If creates a conditional logger that logs only if the condition is true using the default logger.
func If(condition bool) *Conditional {
	return defaultLogger.If(condition)
}

// IfErr creates a conditional logger that logs only if the error is non-nil using the default logger.
func IfErr(err error) *Conditional {
	return defaultLogger.IfErr(err)
}

// IfErrAny creates a conditional logger that logs only if AT LEAST ONE error is non-nil using the default logger.
func IfErrAny(errs ...error) *Conditional {
	return defaultLogger.IfErrAny(errs...)
}

// IfErrOne creates a conditional logger that logs only if ALL errors are non-nil using the default logger.
func IfErrOne(errs ...error) *Conditional {
	return defaultLogger.IfErrOne(errs...)
}

// Context creates a new logger with additional contextual fields using the default logger.
// It preserves existing context fields and adds new ones, returning a new logger instance
// to avoid mutating the default logger. Thread-safe with write lock.
// Example:
//
//	logger := ll.Context(map[string]interface{}{"user": "alice"})
//	logger.Info("Action") // Output: [] INFO: Action [user=alice]
func Context(fields map[string]interface{}) *Logger {
	return defaultLogger.Context(fields)
}

// AddContext adds a key-value pair to the default logger’s context, modifying it directly.
// It mutates the default logger’s context and is thread-safe using a write lock.
// Example:
//
//	ll.AddContext("user", "alice")
//	ll.Info("Action") // Output: [] INFO: Action [user=alice]
func AddContext(pairs ...any) *Logger {
	return defaultLogger.AddContext(pairs...)
}

// GetContext returns the default logger’s context map of persistent key-value fields.
// It provides thread-safe read access to the context using a read lock.
// Example:
//
//	ll.AddContext("user", "alice")
//	ctx := ll.GetContext() // Returns map[string]interface{}{"user": "alice"}k
func GetContext() map[string]interface{} {
	return defaultLogger.GetContext()
}

// GetLevel returns the minimum log level for the default logger.
// It provides thread-safe read access to the level field using a read lock.
// Example:
//
//	ll.Level(lx.LevelWarn)
//	if ll.GetLevel() == lx.LevelWarn {
//	    ll.Warn("Warning level set") // Output: [] WARN: Warning level set
//	}
func GetLevel() lx.LevelType {
	return defaultLogger.GetLevel()
}

// GetPath returns the default logger’s current namespace path.
// It provides thread-safe read access to the currentPath field using a read lock.
// Example:
//
//	logger := ll.Namespace("app")
//	path := logger.GetPath() // Returns "app"
func GetPath() string {
	return defaultLogger.GetPath()
}

// GetSeparator returns the default logger’s namespace separator (e.g., "/").
// It provides thread-safe read access to the separator field using a read lock.
// Example:
//
//	ll.Separator(".")
//	sep := ll.GetSeparator() // Returns "."
func GetSeparator() string {
	return defaultLogger.GetSeparator()
}

// GetStyle returns the default logger’s namespace formatting style (FlatPath or NestedPath).
// It provides thread-safe read access to the style field using a read lock.
// Example:
//
//	ll.Style(lx.NestedPath)
//	if ll.GetStyle() == lx.NestedPath {
//	    ll.Info("Nested style") // Output: []: INFO: Nested style
//	}
func GetStyle() lx.StyleType {
	return defaultLogger.GetStyle()
}

// GetHandler returns the default logger’s current handler for customization or inspection.
// The returned handler should not be modified concurrently with logger operations.
// Example:
//
//	handler := ll.GetHandler() // Returns the current handler (e.g., TextHandler)
func GetHandler() lx.Handler {
	return defaultLogger.GetHandler()
}

// Separator sets the namespace separator for the default logger (e.g., "/" or ".").
// It updates the separator used in namespace paths. Thread-safe with write lock.
// Returns the default logger for method chaining.
// Example:
//
//	ll.Separator(".")
//	ll.Namespace("app").Info("Log") // Output: [app] INFO: Log
func Separator(separator string) *Logger {
	return defaultLogger.Separator(separator)
}

// Prefix sets a prefix to be prepended to all log messages of the default logger.
// The prefix is applied before the message in the log output. Thread-safe with write lock.
// Returns the default logger for method chaining.
// Example:
//
//	ll.Prefix("APP: ")
//	ll.Info("Started") // Output: [] INFO: APP: Started
func Prefix(prefix string) *Logger {
	return defaultLogger.Prefix(prefix)
}

// StackSize sets the buffer size for stack trace capture in the default logger.
// It configures the maximum size for stack traces in Stack, Fatal, and Panic methods.
// Thread-safe with write lock. Returns the default logger for chaining.
// Example:
//
//	ll.StackSize(65536)
//	ll.Stack("Error") // Captures up to 64KB stack trace
func StackSize(size int) *Logger {
	return defaultLogger.StackSize(size)
}

// Use adds a middleware function to process log entries before they are handled by the default logger.
// It registers the middleware and returns a Middleware handle for removal. Middleware returning
// a non-nil error stops the log. Thread-safe with write lock.
// Example:
//
//	mw := ll.Use(ll.FuncMiddleware(func(e *lx.Entry) error {
//	    if e.Level < lx.LevelWarn {
//	        return fmt.Errorf("level too low")
//	    }
//	    return nil
//	}))
//	ll.Info("Ignored") // No output
//	mw.Remove()
//	ll.Info("Logged") // Output: [] INFO: Logged
func Use(fn lx.Handler) *Middleware {
	return defaultLogger.Use(fn)
}

// Remove removes middleware by the reference returned from Use for the default logger.
// It delegates to the Middleware’s Remove method for thread-safe removal.
// Example:
//
//	mw := ll.Use(someMiddleware)
//	ll.Remove(mw) // Removes middleware
func Remove(m *Middleware) {
	defaultLogger.Remove(m)
}

// Clear removes all middleware functions from the default logger.
// It resets the middleware chain to empty, ensuring no middleware is applied.
// Thread-safe with write lock. Returns the default logger for chaining.
// Example:
//
//	ll.Use(someMiddleware)
//	ll.Clear()
//	ll.Info("No middleware") // Output: [] INFO: No middleware
func Clear() *Logger {
	return defaultLogger.Clear()
}

// CanLog checks if a log at the given level would be emitted by the default logger.
// It considers enablement, log level, namespaces, sampling, and rate limits.
// Thread-safe via the Logger’s shouldLog method.
// Example:
//
//	ll.Level(lx.LevelWarn)
//	canLog := ll.CanLog(lx.LevelInfo) // false
func CanLog(level lx.LevelType) bool {
	return defaultLogger.CanLog(level)
}

// NamespaceEnabled checks if a namespace is enabled in the default logger.
// It evaluates the namespace hierarchy, considering parent namespaces, and caches the result
// for performance. Thread-safe with read lock.
// Example:
//
//	ll.NamespaceDisable("app/db")
//	enabled := ll.NamespaceEnabled("app/db") // false
func NamespaceEnabled(path string) bool {
	return defaultLogger.NamespaceEnabled(path)
}

// Print logs a message at Info level without format specifiers using the default logger.
// It concatenates variadic arguments with spaces, minimizing allocations, and delegates
// to defaultLogger’s Print method. Thread-safe via the Logger’s log method.
// Example:
//
//	ll.Print("message", "value") // Output: [] INFO: message value
func Print(args ...any) {
	defaultLogger.Print(args...)
}

// Println logs a message at Info level without format specifiers, minimizing allocations
// by concatenating arguments with spaces. It is thread-safe via the log method.
// Example:
//
//	ll.Println("message", "value") // Output: [] INFO: message value [New Line]
func Println(args ...any) {
	defaultLogger.Println(args...)
}

// Printf logs a message at Info level with a format string using the default logger.
// It formats the message and delegates to defaultLogger’s Printf method. Thread-safe via
// the Logger’s log method.
// Example:
//
//	ll.Printf("Message %s", "value") // Output: [] INFO: Message value
func Printf(format string, args ...any) {
	defaultLogger.Printf(format, args...)
}

// Len returns the total number of log entries sent to the handler by the default logger.
// It provides thread-safe access to the entries counter using atomic operations.
// Example:
//
//	ll.Info("Test")
//	count := ll.Len() // Returns 1
func Len() int64 {
	return defaultLogger.Len()
}

// Measure is a benchmarking helper that measures and returns the duration of a function’s execution.
// It logs the duration at Info level with a "duration" field using defaultLogger. The function
// is executed once, and the elapsed time is returned. Thread-safe via the Logger’s mutex.
// Example:
//
//	duration := ll.Measure(func() { time.Sleep(time.Millisecond) })
//	// Output: [] INFO: function executed [duration=~1ms]
func Measure(fns ...func()) time.Duration {
	return defaultLogger.Measure(fns...)
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
func Labels(names ...string) *Logger {
	return defaultLogger.Labels(names...)
}

// Since creates a timer that will log the duration when completed
// If startTime is provided, uses that as the start time; otherwise uses time.Now()
//
//	defer logger.Since().Info("request")        // Auto-start
//	logger.Since(start).Info("request")         // Manual timing
//	logger.Since().If(debug).Debug("timing")    // Conditional
func Since(start ...time.Time) *SinceBuilder {
	return defaultLogger.Since(start...)
}

// Benchmark logs the duration since a start time at Info level using the default logger.
// It calculates the time elapsed since the provided start time and logs it with "start",
// "end", and "duration" fields. Thread-safe via the Logger’s mutex.
// Example:
//
//	start := time.Now()
//	time.Sleep(time.Millisecond)
//	ll.Benchmark(start) // Output: [] INFO: benchmark [start=... end=... duration=...]
func Benchmark(start time.Time) {
	defaultLogger.Benchmark(start)
}

// Clone returns a new logger with the same configuration as the default logger.
// It creates a copy of defaultLogger’s settings (level, style, namespaces, etc.) but with
// an independent context, allowing customization without affecting the global logger.
// Thread-safe via the Logger’s Clone method.
// Example:
//
//	logger := ll.Clone().Namespace("sub")
//	logger.Info("Sub-logger") // Output: [sub] INFO: Sub-logger
func Clone() *Logger {
	return defaultLogger.Clone()
}

// Err adds one or more errors to the default logger’s context and logs them.
// It stores non-nil errors in the "error" context field and logs their concatenated string
// representations (e.g., "failed 1; failed 2") at the Error level. Thread-safe via the Logger’s mutex.
// Example:
//
//	err1 := errors.New("failed 1")
//	ll.Err(err1)
//	ll.Info("Error occurred") // Output: [] ERROR: failed 1
//	                         //         [] INFO: Error occurred [error=failed 1]
func Err(errs ...error) {
	defaultLogger.Err(errs...)
}

// Start activates the global logging system.
// If the system was shut down, this re-enables all logging operations,
// subject to individual logger and namespace configurations.
// Thread-safe via atomic operations.
// Example:
//
//	ll.Shutdown()
//	ll.Info("Ignored") // No output
//	ll.Start()
//	ll.Info("Logged")  // Output: [] INFO: Logged
func Start() {
	atomic.StoreInt32(&systemActive, 1)
}

// Shutdown deactivates the global logging system.
// All logging operations are skipped, regardless of individual logger or namespace configurations,
// until Start() is called again. Thread-safe via atomic operations.
// Example:
//
//	ll.Shutdown()
//	ll.Info("Ignored") // No output
func Shutdown() {
	atomic.StoreInt32(&systemActive, 0)
}

// Active returns true if the global logging system is currently active.
// Thread-safe via atomic operations.
// Example:
//
//	if ll.Active() {
//	    ll.Info("System active") // Output: [] INFO: System active
//	}
func Active() bool {
	return atomic.LoadInt32(&systemActive) == 1
}

// Enable activates logging for the default logger.
// It allows logs to be emitted if other conditions (level, namespace) are met.
// Thread-safe with write lock. Returns the default logger for method chaining.
// Example:
//
//	ll.Disable()
//	ll.Info("Ignored") // No output
//	ll.Enable()
//	ll.Info("Logged")  // Output: [] INFO: Logged
func Enable() *Logger {
	return defaultLogger.Enable()
}

// Disable deactivates logging for the default logger.
// It suppresses all logs, regardless of level or namespace. Thread-safe with write lock.
// Returns the default logger for method chaining.
// Example:
//
//	ll.Disable()
//	ll.Info("Ignored") // No output
func Disable() *Logger {
	return defaultLogger.Disable()
}

// Dbg logs debug information including the source file, line number, and expression value
// using the default logger. It captures the calling line of code and displays both the
// expression and its value. Useful for debugging without temporary print statements.
// Example:
//
//	x := 42
//	ll.Dbg(x) // Output: [file.go:123] x = 42
func Dbg(any ...interface{}) {
	defaultLogger.dbg(2, any...)
}

// Dump displays a hex and ASCII representation of a value’s binary form using the default logger.
// It serializes the value using gob encoding or direct conversion and shows a hex/ASCII dump.
// Useful for inspecting binary data structures.
// Example:
//
//	ll.Dump([]byte{0x41, 0x42}) // Outputs hex/ASCII dump
func Dump(values ...interface{}) {
	defaultLogger.Dump(values...)
}

// Enabled returns whether the default logger is enabled for logging.
// It provides thread-safe read access to the enabled field using a read lock.
// Example:
//
//	ll.Enable()
//	if ll.Enabled() {
//	    ll.Info("Logging enabled") // Output: [] INFO: Logging enabled
//	}
func Enabled() bool {
	return defaultLogger.Enabled()
}

// Fields starts a fluent chain for adding fields using variadic key-value pairs with the default logger.
// It creates a FieldBuilder to attach fields, handling non-string keys or uneven pairs by
// adding an error field. Thread-safe via the FieldBuilder’s logger.
// Example:
//
//	ll.Fields("user", "alice").Info("Action") // Output: [] INFO: Action [user=alice]
func Fields(pairs ...any) *FieldBuilder {
	return defaultLogger.Fields(pairs...)
}

// Field starts a fluent chain for adding fields from a map with the default logger.
// It creates a FieldBuilder to attach fields from a map, supporting type-safe field addition.
// Thread-safe via the FieldBuilder’s logger.
// Example:
//
//	ll.Field(map[string]interface{}{"user": "alice"}).Info("Action") // Output: [] INFO: Action [user=alice]
func Field(fields map[string]interface{}) *FieldBuilder {
	return defaultLogger.Field(fields)
}

// Line adds vertical spacing (newlines) to the log output using the default logger.
// If no arguments are provided, it defaults to 1 newline. Multiple values are summed to
// determine the total lines. Useful for visually separating log sections. Thread-safe.
// Example:
//
//	ll.Line(2).Info("After two newlines") // Adds 2 blank lines before: [] INFO: After two newlines
func Line(lines ...int) *Logger {
	return defaultLogger.Line(lines...)
}

// Indent sets the indentation level for all log messages of the default logger.
// Each level adds two spaces to the log message, useful for hierarchical output.
// Thread-safe with write lock. Returns the default logger for method chaining.
// Example:
//
//	ll.Indent(2)
//	ll.Info("Indented") // Output: [] INFO:     Indented
func Indent(depth int) *Logger {
	return defaultLogger.Indent(depth)
}

// Mark logs the current file and line number where it's called, without any additional debug information.
// It's useful for tracing execution flow without the verbosity of Dbg.
// Example:
//
//	logger.Mark() // *MARK*: [file.go:123]
func Mark(names ...string) {
	defaultLogger.mark(2, names...)

}

// Output logs data in a human-readable JSON format at Info level, including caller file and line information.
// It is similar to Dbg but formats the output as JSON for better readability. It is thread-safe and respects
// the logger’s configuration (e.g., enabled, level, suspend, handler, middleware).
func Output(values ...interface{}) {
	defaultLogger.output(2, values...)

}

// Inspect logs one or more values in a **developer-friendly, deeply introspective format** at Info level.
// It includes the caller file and line number, and reveals **all fields** — including:
func Inspect(values ...interface{}) {
	o := NewInspector(defaultLogger)
	o.Log(2, values...)
}

func Apply(opts ...Option) *Logger {
	return defaultLogger.Apply(opts...)

}

func Toggle(v bool) *Logger {
	return defaultLogger.Toggle(v)
}
