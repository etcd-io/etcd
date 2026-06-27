# ll - A Modern Structured Logging Library for Go

`ll` is a high-performance, production-ready logging library for Go, designed to provide **hierarchical namespaces**, **structured logging**, **middleware pipelines**, **conditional logging**, and support for multiple output formats, including text, JSON, colorized logs, syslog, VictoriaLogs, and compatibility with Go's `slog`. It's ideal for applications requiring fine-grained log control, extensibility, and scalability.

## Key Features

- **Logging Enabled by Default** - Zero configuration to start logging
- **Hierarchical Namespaces** - Organize logs with fine-grained control over subsystems (e.g., "app/db")
- **Structured Logging** - Add key-value metadata for machine-readable logs
- **Middleware Pipeline** - Customize log processing with rate limiting, sampling, and deduplication
- **Conditional & Error-Based Logging** - Optimize performance with fluent `If`, `IfErr`, `IfAny`, `IfOne` chains
- **Multiple Output Formats** - Text, JSON, colorized ANSI, syslog, VictoriaLogs, and `slog` integration
- **Advanced Debugging Utilities** - Source-aware `Dbg()`, hex/ASCII `Dump()`, private field `Inspect()`, and stack traces
- **Production Ready** - Buffered batching, log rotation, duplicate suppression, and rate limiting
- **Thread-Safe** - Built for high-concurrency with atomic operations, sharded mutexes, and lock-free fast paths
- **Performance Optimized** - Zero allocations for disabled logs, sync.Pool buffers, LRU caching for source files

## Installation

Install `ll` using Go modules:

```bash
go get github.com/olekukonko/ll
```

Requires Go 1.21 or later.

## Quick Start

```go
package main

import "github.com/olekukonko/ll"

func main() {
    // Logger is ENABLED by default - no .Enable() needed!
    logger := ll.New("app")
    
    // Basic logging - works immediately
    logger.Info("Server starting")     // Output: [app] INFO: Server starting
    logger.Warn("Memory high")         // Output: [app] WARN: Memory high
    logger.Error("Connection failed")  // Output: [app] ERROR: Connection failed
    
    // Structured fields
    logger.Fields("user", "alice", "status", 200).Info("Login successful")
    // Output: [app] INFO: Login successful [user=alice status=200]
}
```

**That's it. No `.Enable()`, no handlers to configure—it just works.**

## Core Concepts

### 1. Enabled by Default, Configurable When Needed

Unlike many logging libraries that require explicit enabling, `ll` **logs immediately**. This eliminates boilerplate and reduces the chance of missing logs in production.

```go
// This works out of the box:
ll.Info("Service started")  // Output: [] INFO: Service started

// But you still have full control:
ll.Disable()  // Global shutdown
ll.Enable()   // Reactivate
```

### 2. Hierarchical Namespaces

Organize logs hierarchically with precise control over subsystems:

```go
// Create a logger hierarchy
root := ll.New("app")
db := root.Namespace("database")
cache := root.Namespace("cache").Style(lx.NestedPath)

// Control logging per namespace
root.NamespaceEnable("app/database")    // Enable database logs
root.NamespaceDisable("app/cache")      // Disable cache logs

db.Info("Connected")     // Output: [app/database] INFO: Connected
cache.Info("Hit")        // No output (disabled)
```

### 3. Structured Logging with Ordered Fields

Fields maintain insertion order and support fluent chaining:

```go
// Fluent key-value pairs
logger.
    Fields("request_id", "req-123").
    Fields("user", "alice").
    Fields("duration_ms", 42).
    Info("Request processed")

// Map-based fields
logger.Field(map[string]interface{}{
    "method": "POST",
    "path": "/api/users",
}).Debug("API call")

// Persistent context (included in ALL subsequent logs)
logger.AddContext("environment", "production", "version", "1.2.3")
logger.Info("Deployed")  // Output: ... [environment=production version=1.2.3]
```

### 4. Conditional & Error-Based Logging

Optimize performance with fluent conditional chains that **completely skip processing** when conditions are false:

```go
// Boolean conditions
logger.If(debugMode).Debug("Detailed diagnostics")  // No overhead when false
logger.If(featureEnabled).Info("Feature used")

// Error conditions
err := db.Query()
logger.IfErr(err).Error("Query failed")              // Logs only if err != nil

// Multiple conditions - ANY true
logger.IfErrAny(err1, err2, err3).Fatal("System failure")

// Multiple conditions - ALL true
logger.IfErrOne(validateErr, authErr).Error("Both checks failed")

// Chain conditions
logger.
    If(debugMode).
    IfErr(queryErr).
    Fields("query", sql).
    Debug("Query debug")
```

**Performance**: When conditions are false, the logger returns immediately with zero allocations.

### 5. Powerful Debugging Toolkit

`ll` includes advanced debugging utilities not found in standard logging libraries:

#### Dbg() - Source-Aware Variable Inspection
Captures both variable name AND value from your source code:

```go
x := 42
user := &User{Name: "Alice"}
ll.Dbg(x, user)  
// Output: [file.go:123] x = 42, *user = &{Name:Alice}
```

#### Dump() - Hex/ASCII Binary Inspection
Perfect for protocol debugging and binary data:

```go
ll.Handler(lh.NewColorizedHandler(os.Stdout))
ll.Dump([]byte("hello\nworld"))
// Output: Colorized hex/ASCII dump with offset markers
```

#### Inspect() - Private Field Reflection
Reveals unexported fields, embedded structs, and pointer internals:

```go
type secret struct {
    password string  // unexported!
}

s := secret{password: "hunter2"}
ll.Inspect(s)
// Output: [file.go:123] INSPECT: {
//   "(password)": "hunter2"  // Note the parentheses
// }
```

#### Stack() - Configurable Stack Traces
```go
ll.StackSize(8192)  // Larger buffer for deep stacks
ll.Stack("Critical failure")
// Output: ERROR: Critical failure [stack=goroutine 1 [running]...]
```

#### Mark() - Execution Flow Tracing
```go
func process() {
    ll.Mark()           // *MARK*: [file.go:123]
    ll.Mark("phase1")   // *phase1*: [file.go:124]
    // ... work ...
}
```

### 6. Production-Ready Handlers

```go
import (
    "github.com/olekukonko/ll"
    "github.com/olekukonko/ll/lh"
    "github.com/olekukonko/ll/l3rd/syslog"
    "github.com/olekukonko/ll/l3rd/victoria"
)

// JSON for structured logging
logger.Handler(lh.NewJSONHandler(os.Stdout))

// Colorized for development
logger.Handler(lh.NewColorizedHandler(os.Stdout, 
    lh.WithColorTheme("dark"),
    lh.WithColorIntensity(lh.IntensityVibrant),
))

// Buffered for high throughput (100 entries or 10 seconds)
buffered := lh.NewBuffered(
    lh.NewJSONHandler(os.Stdout),
    lh.WithBatchSize(100),
    lh.WithFlushInterval(10 * time.Second),
)
logger.Handler(buffered)
defer buffered.Close()  // Ensures flush on exit

// Syslog integration
syslogHandler, _ := syslog.New(
    syslog.WithTag("myapp"),
    syslog.WithFacility(syslog.LOG_LOCAL0),
)
logger.Handler(syslogHandler)

// VictoriaLogs (cloud-native)
victoriaHandler, _ := victoria.New(
    victoria.WithURL("http://victoria-logs:9428"),
    victoria.WithAppName("payment-service"),
    victoria.WithEnvironment("production"),
    victoria.WithBatching(200, 5*time.Second),
)
logger.Handler(victoriaHandler)
```

### 7. Middleware Pipeline

Transform, filter, or reject logs with a middleware pipeline:

```go
// Rate limiting - 10 logs per second maximum
rateLimiter := lm.NewRateLimiter(lx.LevelInfo, 10, time.Second)
logger.Use(rateLimiter)

// Sampling - 10% of debug logs
sampler := lm.NewSampling(lx.LevelDebug, 0.1)
logger.Use(sampler)

// Deduplication - suppress identical logs for 2 seconds
deduper := lh.NewDedup(logger.GetHandler(), 2*time.Second)
logger.Handler(deduper)

// Custom middleware
logger.Use(ll.Middle(func(e *lx.Entry) error {
    if strings.Contains(e.Message, "password") {
        return fmt.Errorf("sensitive information redacted")
    }
    return nil
}))
```

### 8. Global Convenience API

Use package-level functions for quick logging without creating loggers:

```go
import "github.com/olekukonko/ll"

func main() {
    ll.Info("Server starting")           // Global logger
    ll.Fields("port", 8080).Info("Listening")
    
    // Conditional logging at package level
    ll.If(simulation).Debug("Test mode")
    ll.IfErr(err).Error("Startup failed")
    
    // Debug utilities
    ll.Dbg(config)
    ll.Dump(requestBody)
    ll.Inspect(complexStruct)
}
```

## Real-World Examples

### Web Server with Structured Logging

```go
package main

import (
    "github.com/olekukonko/ll"
    "github.com/olekukonko/ll/lh"
    "net/http"
    "time"
)

func main() {
    // Root logger - enabled by default
    log := ll.New("server")
    
    // JSON output for production
    log.Handler(lh.NewJSONHandler(os.Stdout))
    
    // Request logger with context
    http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        reqLog := log.Namespace("http").Fields(
            "method", r.Method,
            "path", r.URL.Path,
            "request_id", r.Header.Get("X-Request-ID"),
        )
        
        start := time.Now()
        reqLog.Info("request started")
        
        // ... handle request ...
        
        reqLog.Fields(
            "status", 200,
            "duration_ms", time.Since(start).Milliseconds(),
        ).Info("request completed")
    })
    
    log.Info("Server listening on :8080")
    http.ListenAndServe(":8080", nil)
}
```

### Microservice with VictoriaLogs

```go
package main

import (
    "github.com/olekukonko/ll"
    "github.com/olekukonko/ll/l3rd/victoria"
)

func main() {
    // Production setup
    vlHandler, _ := victoria.New(
        victoria.WithURL("http://logs.internal:9428"),
        victoria.WithAppName("payment-api"),
        victoria.WithEnvironment("production"),
        victoria.WithVersion("1.2.3"),
        victoria.WithBatching(500, 2*time.Second),
        victoria.WithRetry(3),
    )
    defer vlHandler.Close()
    
    logger := ll.New("payment").
        Handler(vlHandler).
        AddContext("region", "us-east-1")
    
    logger.Info("Payment service initialized")
    
    // Conditional error handling
    if err := processPayment(); err != nil {
        logger.IfErr(err).
            Fields("payment_id", paymentID).
            Error("Payment processing failed")
    }
}
```

## Performance

`ll` is engineered for high-performance environments:

| Operation | Time/op | Allocations |
|-----------|---------|-------------|
| **Disabled log** | **15.9 ns** | **0 allocs** |
| Simple text log | 176 ns | 2 allocs |
| With 2 fields | 383 ns | 4 allocs |
| JSON output | 1006 ns | 13 allocs |
| Namespace lookup (cached) | 550 ns | 6 allocs |
| Deduplication | 214 ns | 2 allocs |

**Key optimizations**:
- Zero allocations when logs are skipped (conditional, disabled)
- Atomic operations for hot paths
- Sync.Pool for buffer reuse
- LRU cache for source file lines (Dbg)
- Sharded mutexes for deduplication

## Why Choose `ll`?

| Feature | `ll` | `slog` | `zap` | `logrus` |
|---------|------|--------|-------|----------|
| **Enabled by default** | ✅ | ❌ | ❌ | ❌ |
| Hierarchical namespaces | ✅ | ❌ | ❌ | ❌ |
| Conditional logging | ✅ | ❌ | ❌ | ❌ |
| Error-based conditions | ✅ | ❌ | ❌ | ❌ |
| Source-aware Dbg() | ✅ | ❌ | ❌ | ❌ |
| Private field inspection | ✅ | ❌ | ❌ | ❌ |
| Hex/ASCII Dump() | ✅ | ❌ | ❌ | ❌ |
| Middleware pipeline | ✅ | ❌ | ✅ (limited) | ❌ |
| Deduplication | ✅ | ❌ | ❌ | ❌ |
| Rate limiting | ✅ | ❌ | ❌ | ❌ |
| VictoriaLogs support | ✅ | ❌ | ❌ | ❌ |
| Syslog support | ✅ | ❌ | ❌ | ✅ |
| Zero-allocs disabled logs | ✅ | ❌ | ❌ | ❌ |
| Thread-safe | ✅ | ✅ | ✅ | ✅ |

## Documentation

- [GoDoc](https://pkg.go.dev/github.com/olekukonko/ll) - Full API documentation
- [Examples](_example/) - Runable example code
- [Benchmarks](tests/ll_bench_test.go) - Performance benchmarks

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.