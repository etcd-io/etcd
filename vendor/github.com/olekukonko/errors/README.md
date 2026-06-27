# Enhanced Error Handling for Go with Context, Stack Traces, Monitoring, and More

[![Go Reference](https://pkg.go.dev/badge/github.com/olekukonko/errors.svg)](https://pkg.go.dev/github.com/olekukonko/errors)
[![Go Report Card](https://goreportcard.com/badge/github.com/olekukonko/errors)](https://goreportcard.com/report/github.com/olekukonko/errors)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Benchmarks](https://img.shields.io/badge/benchmarks-included-success)](README.md#benchmarks)

A production-grade error handling library for Go, offering zero-cost abstractions, stack traces, multi-error support, retries, and advanced monitoring through two complementary packages: `errors` (core) and `errmgr` (management).

## Features

### `errors` Package (Core)
- **Performance Optimized**
  - Optional memory pooling (12 ns/op with pooling)
  - Lazy stack trace collection (205 ns/op with stack)
  - Small context optimization (≤4 items, 40 ns/op)
  - Lock-free configuration reads

- **Debugging Tools**
  - Full stack traces with internal frame filtering
  - Error wrapping and chaining
  - Structured context attachment
  - JSON serialization (662 ns/op)

- **Advanced Utilities**
  - Configurable retry mechanism
  - Multi-error aggregation with sampling
  - HTTP status code support
  - Callback triggers for cleanup or side effects

### `errmgr` Package (Management)
- **Production Monitoring**
  - Error occurrence counting
  - Threshold-based alerting
  - Categorized metrics
  - Predefined error templates

## Installation

```bash
go get github.com/olekukonko/errors@latest
```

## Package Overview

- **`errors`**: Core error handling with creation, wrapping, context, stack traces, retries, and multi-error support.
- **`errmgr`**: Error management with templates, monitoring, and predefined errors for consistent application use.

---

> [!NOTE]  
> ✓ added support for `errors.Errorf("user %w not found", errors.New("bob"))`  
> ✓ added support for `sequential chain` execution
``

## Using the `errors` Package

### Basic Error Creation

#### Simple Error
```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
)

func main() {
  // Fast error with no stack trace
  err := errors.New("connection failed")
  fmt.Println(err) // "connection failed"

  // Standard error, no allocation, same speed
  stdErr := errors.Std("connection failed")
  fmt.Println(stdErr) // "connection failed"
}
```

#### Formatted Error
```go
// main.go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
)

func main() {
  // Formatted error without stack trace
  errNoWrap := errors.Newf("user %s not found", "bob")
  fmt.Println(errNoWrap) // Output: "user bob not found"

  // Standard formatted error, no fmt.Errorf needed (using own pkg)
  stdErrNoWrap := errors.Stdf("user %s not found", "bob")
  fmt.Println(stdErrNoWrap) // Output: "user bob not found"

  // Added support for %w (compatible with fmt.Errorf output)
  // errors.Errorf is alias of errors.Newf
  errWrap := errors.Errorf("user %w not found", errors.New("bob"))
  fmt.Println(errWrap) // Output: "user bob not found"

  // Standard formatted error for comparison
  stdErrWrap := fmt.Errorf("user %w not found", fmt.Errorf("bob"))
  fmt.Println(stdErrWrap) // Output: "user bob not found"
}
```

#### Error with Stack Trace
```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
)

func main() {
  // Create an error with stack trace using Trace
  err := errors.Trace("critical issue")
  fmt.Println(err)         // Output: "critical issue"
  fmt.Println(err.Stack()) // Output: e.g., ["main.go:15", "caller.go:42"]

  // Convert basic error to traceable with WithStack
  errS := errors.New("critical issue")
  errS = errS.WithStack()   // Add stack trace and update error
  fmt.Println(errS)         // Output: "critical issue"
  fmt.Println(errS.Stack()) // Output: e.g., ["main.go:19", "caller.go:42"]
}
```

#### Named Error
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Create a named error with stack trace
	err := errors.Named("InputError")
	fmt.Println(err.Name()) // Output: "InputError"
	fmt.Println(err)        // Output: "InputError"
}
```

### Adding Context

#### Basic Context
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Create an error with context
	err := errors.New("processing failed").
		With("id", "123").
		With("attempt", 3).
		With("retryable", true)
	fmt.Println("Error:", err)                    // Output: "processing failed"
	fmt.Println("Full context:", errors.Context(err)) // Output: map[id:123 attempt:3 retryable:true]
}
```

#### Context with Wrapped Standard Error
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Wrap a standard error and add context
	err := errors.New("processing failed").
		With("id", "123")
	wrapped := fmt.Errorf("wrapped: %w", err)
	fmt.Println("Wrapped error:", wrapped)             // Output: "wrapped: processing failed"
	fmt.Println("Direct context:", errors.Context(wrapped)) // Output: nil

	// Convert to access context
	e := errors.Convert(wrapped)
	fmt.Println("Converted context:", e.Context()) // Output: map[id:123]
}
```

#### Adding Context to Standard Error
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Convert a standard error and add context
	stdErr := fmt.Errorf("standard error")
	converted := errors.Convert(stdErr).
		With("source", "legacy").
		With("severity", "high")
	fmt.Println("Message:", converted.Error()) // Output: "standard error"
	fmt.Println("Context:", converted.Context()) // Output: map[source:legacy severity:high]
}
```

#### Complex Context
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Create an error with complex context
	err := errors.New("database operation failed").
		With("query", "SELECT * FROM users").
		With("params", map[string]interface{}{
			"limit":  100,
			"offset": 0,
		}).
		With("duration_ms", 45.2)
	fmt.Println("Complex error context:")
	for k, v := range errors.Context(err) {
		fmt.Printf("%s: %v (%T)\n", k, v, v)
	}
	// Output:
	// query: SELECT * FROM users (string)
	// params: map[limit:100 offset:0] (map[string]interface {})
	// duration_ms: 45.2 (float64)
}
```

### Stack Traces

#### Adding Stack to Any Error
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Add stack trace to a standard error
	err := fmt.Errorf("basic error")
	enhanced := errors.WithStack(err)
	fmt.Println("Error with stack:")
	fmt.Println("Message:", enhanced.Error()) // Output: "basic error"
	fmt.Println("Stack:", enhanced.Stack())   // Output: e.g., "main.go:15"
}
```

#### Chaining with Stack
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
	"time"
)

func main() {
	// Create an enhanced error and add stack/context
	err := errors.New("validation error").
		With("field", "email")
	stackErr := errors.WithStack(err).
		With("timestamp", time.Now()).
		WithCode(500)
	fmt.Println("Message:", stackErr.Error()) // Output: "validation error"
	fmt.Println("Context:", stackErr.Context()) // Output: map[field:email timestamp:...]
	fmt.Println("Stack:")
	for _, frame := range stackErr.Stack() {
		fmt.Println(frame)
	}
}
```
### Stack Traces with `WithStack()`
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
	"math/rand"
	"time"
)

func basicFunc() error {
	return fmt.Errorf("basic error")
}

func enhancedFunc() *errors.Error {
	return errors.New("enhanced error")
}

func main() {
	// 1. Package-level WithStack - works with ANY error type
	err1 := basicFunc()
	enhanced1 := errors.WithStack(err1) // Handles basic errors
	fmt.Println("Package-level WithStack:")
	fmt.Println(enhanced1.Stack())

	// 2. Method-style WithStack - only for *errors.Error
	err2 := enhancedFunc()
	enhanced2 := err2.WithStack() // More natural chaining
	fmt.Println("\nMethod-style WithStack:")
	fmt.Println(enhanced2.Stack())

	// 3. Combined usage in real-world scenario
	result := processData()
	if result != nil {
		// Use package-level when type is unknown
		stackErr := errors.WithStack(result)

		// Then use method-style for chaining
		finalErr := stackErr.
			With("timestamp", time.Now()).
			WithCode(500)

		fmt.Println("\nCombined Usage:")
		fmt.Println("Message:", finalErr.Error())
		fmt.Println("Context:", finalErr.Context())
		fmt.Println("Stack:")
		for _, frame := range finalErr.Stack() {
			fmt.Println(frame)
		}
	}
}

func processData() error {
	// Could return either basic or enhanced error
	if rand.Intn(2) == 0 {
		return fmt.Errorf("database error")
	}
	return errors.New("validation error").With("field", "email")
}
```

### Error Wrapping and Chaining

#### Basic Wrapping
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Wrap an error with additional context
	lowErr := errors.New("low-level failure")
	highErr := errors.Wrapf(lowErr, "high-level operation failed: %s", "details")
	fmt.Println(highErr)           // Output: "high-level operation failed: details: low-level failure"
	fmt.Println(errors.Unwrap(highErr)) // Output: "low-level failure"
}
```

#### Walking Error Chain
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Create a chained error
	dbErr := errors.New("connection timeout").
		With("server", "db01.prod")
	bizErr := errors.New("failed to process user 12345").
		With("user_id", "12345").
		Wrap(dbErr)
	apiErr := errors.New("API request failed").
		WithCode(500).
		Wrap(bizErr)

	// Walk the error chain
	fmt.Println("Error Chain:")
	for i, e := range errors.UnwrapAll(apiErr) {
		fmt.Printf("%d. %s\n", i+1, e)
	}
	// Output:
	// 1. API request failed
	// 2. failed to process user 12345
	// 3. connection timeout
}
```

### Type Assertions

#### Using Is
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Check error type with Is
	err := errors.Named("AuthError")
	wrapped := errors.Wrapf(err, "login failed")
	if errors.Is(wrapped, err) {
		fmt.Println("Is an AuthError") // Output: "Is an AuthError"
	}
}
```

#### Using As
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Extract error type with As
	err := errors.Named("AuthError")
	wrapped := errors.Wrapf(err, "login failed")
	var authErr *errors.Error
	if wrapped.As(&authErr) {
		fmt.Println("Extracted:", authErr.Name()) // Output: "Extracted: AuthError"
	}
}
```

### Retry Mechanism

#### Basic Retry
```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
  "math/rand"
  "time"
)

func main() {
  // Simulate a flaky operation
  attempts := 0
  retry := errors.NewRetry(
    errors.WithMaxAttempts(3),
    errors.WithDelay(100*time.Millisecond),
  )
  err := retry.Execute(func() error {
    attempts++
    if rand.Intn(2) == 0 {
      return errors.New("temporary failure").WithRetryable()
    }
    return nil
  })
  if err != nil {
    fmt.Printf("Failed after %d attempts: %v\n", attempts, err)
  } else {
    fmt.Printf("Succeeded after %d attempts\n", attempts)
  }
}

```

#### Retry with Context Timeout
```go
package main

import (
	"context"
	"fmt"
	"github.com/olekukonko/errors"
	"time"
)

func main() {
	// Retry with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	retry := errors.NewRetry(
		errors.WithContext(ctx),
		errors.WithMaxAttempts(5),
		errors.WithDelay(200*time.Millisecond),
	)
	err := retry.Execute(func() error {
		return errors.New("operation failed").WithRetryable()
	})
	if errors.Is(err, context.DeadlineExceeded) {
		fmt.Println("Operation timed out:", err)
	} else if err != nil {
		fmt.Println("Operation failed:", err)
	}
}
```


### Retry Comprehensive

```go
package main

import (
  "context"
  "fmt"
  "github.com/olekukonko/errors"
  "math/rand"
  "time"
)

// DatabaseClient simulates a flaky database connection
type DatabaseClient struct {
  healthyAfterAttempt int
}

func (db *DatabaseClient) Query() error {
  if db.healthyAfterAttempt > 0 {
    db.healthyAfterAttempt--
    return errors.New("database connection failed").
      With("attempt_remaining", db.healthyAfterAttempt).
      WithRetryable() // Mark as retryable
  }
  return nil
}

// ExternalService simulates an unreliable external API
func ExternalService() error {
  if rand.Intn(100) < 30 { // 30% failure rate
    return errors.New("service unavailable").
      WithCode(503).
      WithRetryable()
  }
  return nil
}

func main() {
  // Configure retry with exponential backoff and jitter
  retry := errors.NewRetry(
    errors.WithMaxAttempts(5),
    errors.WithDelay(200*time.Millisecond),
    errors.WithMaxDelay(2*time.Second),
    errors.WithJitter(true),
    errors.WithBackoff(errors.ExponentialBackoff{}),
    errors.WithOnRetry(func(attempt int, err error) {
      // Calculate delay using the same logic as in Execute
      baseDelay := 200 * time.Millisecond
      maxDelay := 2 * time.Second
      delay := errors.ExponentialBackoff{}.Backoff(attempt, baseDelay)
      if delay > maxDelay {
        delay = maxDelay
      }
      fmt.Printf("Attempt %d failed: %v (retrying in %v)\n",
        attempt,
        err.Error(),
        delay)
    }),
  )

  // Scenario 1: Database connection with known recovery point
  db := &DatabaseClient{healthyAfterAttempt: 3}
  fmt.Println("Starting database operation...")
  err := retry.Execute(func() error {
    return db.Query()
  })
  if err != nil {
    fmt.Printf("Database operation failed after %d attempts: %v\n", retry.Attempts(), err)
  } else {
    fmt.Println("Database operation succeeded!")
  }

  // Scenario 2: External service with random failures
  fmt.Println("\nStarting external service call...")
  var lastAttempts int
  start := time.Now()

  // Using ExecuteReply to demonstrate return values
  result, err := errors.ExecuteReply[string](retry, func() (string, error) {
    lastAttempts++
    if err := ExternalService(); err != nil {
      return "", err
    }
    return "service response data", nil
  })

  duration := time.Since(start)

  if err != nil {
    fmt.Printf("Service call failed after %d attempts (%.2f sec): %v\n",
      lastAttempts,
      duration.Seconds(),
      err)
  } else {
    fmt.Printf("Service call succeeded after %d attempts (%.2f sec): %s\n",
      lastAttempts,
      duration.Seconds(),
      result)
  }

  // Scenario 3: Context cancellation with more visibility
  fmt.Println("\nStarting operation with timeout...")
  ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
  defer cancel()

  timeoutRetry := retry.Transform(
    errors.WithContext(ctx),
    errors.WithMaxAttempts(10),
    errors.WithOnRetry(func(attempt int, err error) {
      fmt.Printf("Timeout scenario attempt %d: %v\n", attempt, err)
    }),
  )

  startTimeout := time.Now()
  err = timeoutRetry.Execute(func() error {
    time.Sleep(300 * time.Millisecond) // Simulate long operation
    return errors.New("operation timed out")
  })

  if errors.Is(err, context.DeadlineExceeded) {
    fmt.Printf("Operation cancelled by timeout after %.2f sec: %v\n",
      time.Since(startTimeout).Seconds(),
      err)
  } else if err != nil {
    fmt.Printf("Operation failed: %v\n", err)
  } else {
    fmt.Println("Operation succeeded (unexpected)")
  }
}
```

### Multi-Error Aggregation

#### Form Validation
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Validate a form with multiple errors
	multi := errors.NewMultiError()
	multi.Add(errors.New("name is required"))
	multi.Add(errors.New("email is invalid"))
	multi.Add(errors.New("password too short"))
	if multi.Has() {
		fmt.Println(multi) // Output: "errors(3): name is required; email is invalid; password too short"
		fmt.Printf("Total errors: %d\n", multi.Count())
	}
}
```

#### Sampling Multi-Errors
```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
)

func main() {
  // Simulate many errors with sampling
  multi := errors.NewMultiError(
    errors.WithSampling(10), // 10% sampling
    errors.WithLimit(5),
  )
  for i := 0; i < 100; i++ {
    multi.Add(errors.Newf("error %d", i))
  }
  fmt.Println(multi)
  fmt.Printf("Captured %d out of 100 errors\n", multi.Count())
}
```

### Additional Examples

#### Using Callbacks
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Add a callback to an error
	err := errors.New("transaction failed").
		Callback(func() {
			fmt.Println("Reversing transaction...")
		})
	fmt.Println(err) // Output: "transaction failed" + "Reversing transaction..."
	err.Free()
}
```

#### Copying Errors
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Copy an error and modify the copy
	original := errors.New("base error").With("key", "value")
	copied := original.Copy().With("extra", "data")
	fmt.Println("Original:", original, original.Context()) // Output: "base error" map[key:value]
	fmt.Println("Copied:", copied, copied.Context())       // Output: "base error" map[key:value extra:data]
}
```

#### Transforming Errors
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Transform an error with additional context
	err := errors.New("base error")
	transformed := errors.Transform(err, func(e *errors.Error) {
		e.With("env", "prod").
			WithCode(500).
			WithStack()
	})
	fmt.Println(transformed.Error())          // Output: "base error"
	fmt.Println(transformed.Context())        // Output: map[env:prod]
	fmt.Println(transformed.Code())           // Output: 500
	fmt.Println(len(transformed.Stack()) > 0) // Output: true
	transformed.Free()
}
```

### Transformation and Enrichment

```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
)

func process() error {
  return errors.New("base error")
}

func main() {
  err := process()
  transformedErr := errors.Transform(err, func(e *errors.Error) {
    e.With("env", "prod").
      WithCode(500).
      WithStack()
  })

  // No type assertion needed now
  fmt.Println(transformedErr.Error())          // "base error"
  fmt.Println(transformedErr.Context())        // map[env:prod]
  fmt.Println(transformedErr.Code())           // 500
  fmt.Println(len(transformedErr.Stack()) > 0) // true
  transformedErr.Free()                        // Clean up

  stdErr := process()
  convertedErr := errors.Convert(stdErr) // Convert standard error to *Error
  convertedErr.With("source", "external").
    WithCode(400).
          Callback(func() {
            fmt.Println("Converted error processed...")
          })
  fmt.Println("Converted Error:", convertedErr.Error())
  fmt.Println("Context:", convertedErr.Context())
  fmt.Println("Code:", convertedErr.Code())
  convertedErr.Free()

}

```

#### Fast Stack Trace
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Get a lightweight stack trace
	err := errors.Trace("lightweight error")
	fastStack := err.FastStack()
	fmt.Println("Fast Stack:")
	for _, frame := range fastStack {
		fmt.Println(frame) // Output: e.g., "main.go:15"
	}
}
```

#### WarmStackPool
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Pre-warm the stack pool
	errors.WarmStackPool(10)
	err := errors.Trace("pre-warmed error")
	fmt.Println("Stack after warming pool:")
	for _, frame := range err.Stack() {
		fmt.Println(frame)
	}
}
```

### Multi-Error Aggregation

```go
package main

import (
  "fmt"
  "net/mail"
  "strings"
  "time"

  "github.com/olekukonko/errors"
)

type UserForm struct {
  Name     string
  Email    string
  Password string
  Birthday string
}

func validateUser(form UserForm) *errors.MultiError {
  multi := errors.NewMultiError(
    errors.WithLimit(10),
    errors.WithFormatter(customFormat),
  )

  // Name validation
  if form.Name == "" {
    multi.Add(errors.New("name is required"))
  } else if len(form.Name) > 50 {
    multi.Add(errors.New("name cannot exceed 50 characters"))
  }

  // Email validation
  if form.Email == "" {
    multi.Add(errors.New("email is required"))
  } else {
    if _, err := mail.ParseAddress(form.Email); err != nil {
      multi.Add(errors.New("invalid email format"))
    }
    if !strings.Contains(form.Email, "@") {
      multi.Add(errors.New("email must contain @ symbol"))
    }
  }

  // Password validation
  if len(form.Password) < 8 {
    multi.Add(errors.New("password must be at least 8 characters"))
  }
  if !strings.ContainsAny(form.Password, "0123456789") {
    multi.Add(errors.New("password must contain at least one number"))
  }
  if !strings.ContainsAny(form.Password, "!@#$%^&*") {
    multi.Add(errors.New("password must contain at least one special character"))
  }

  // Birthday validation
  if form.Birthday != "" {
    if _, err := time.Parse("2006-01-02", form.Birthday); err != nil {
      multi.Add(errors.New("birthday must be in YYYY-MM-DD format"))
    } else if bday, _ := time.Parse("2006-01-02", form.Birthday); time.Since(bday).Hours()/24/365 < 13 {
      multi.Add(errors.New("must be at least 13 years old"))
    }
  }

  return multi
}

func customFormat(errs []error) string {
  var sb strings.Builder
  sb.WriteString("🚨 Validation Errors:\n")
  for i, err := range errs {
    sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err))
  }
  sb.WriteString(fmt.Sprintf("\nTotal issues found: %d\n", len(errs)))
  return sb.String()
}

func main() {
  fmt.Println("=== User Registration Validation ===")

  user := UserForm{
    Name:     "", // Empty name
    Email:    "invalid-email",
    Password: "weak",
    Birthday: "2015-01-01", // Under 13
  }

  // Generate multiple validation errors
  validationErrors := validateUser(user)

  if validationErrors.Has() {
    fmt.Println(validationErrors)

    // Detailed error analysis
    fmt.Println("\n🔍 Error Analysis:")
    fmt.Printf("Total errors: %d\n", validationErrors.Count())
    fmt.Printf("First error: %v\n", validationErrors.First())
    fmt.Printf("Last error: %v\n", validationErrors.Last())

    // Categorized errors with consistent formatting
    fmt.Println("\n📋 Error Categories:")
    if emailErrors := validationErrors.Filter(contains("email")); emailErrors.Has() {
      fmt.Println("Email Issues:")
      if emailErrors.Count() == 1 {
        fmt.Println(customFormat([]error{emailErrors.First()}))
      } else {
        fmt.Println(emailErrors)
      }
    }
    if pwErrors := validationErrors.Filter(contains("password")); pwErrors.Has() {
      fmt.Println("Password Issues:")
      if pwErrors.Count() == 1 {
        fmt.Println(customFormat([]error{pwErrors.First()}))
      } else {
        fmt.Println(pwErrors)
      }
    }
    if ageErrors := validationErrors.Filter(contains("13 years")); ageErrors.Has() {
      fmt.Println("Age Restriction:")
      if ageErrors.Count() == 1 {
        fmt.Println(customFormat([]error{ageErrors.First()}))
      } else {
        fmt.Println(ageErrors)
      }
    }
  }

  // System Error Aggregation Example
  fmt.Println("\n=== System Error Aggregation ===")
  systemErrors := errors.NewMultiError(
    errors.WithLimit(5),
    errors.WithFormatter(systemErrorFormat),
  )

  // Simulate system errors
  systemErrors.Add(errors.New("database connection timeout").WithRetryable())
  systemErrors.Add(errors.New("API rate limit exceeded").WithRetryable())
  systemErrors.Add(errors.New("disk space low"))
  systemErrors.Add(errors.New("database connection timeout").WithRetryable()) // Duplicate
  systemErrors.Add(errors.New("cache miss"))
  systemErrors.Add(errors.New("database connection timeout").WithRetryable()) // Over limit

  fmt.Println(systemErrors)
  fmt.Printf("\nSystem Status: %d active issues\n", systemErrors.Count())

  // Filter retryable errors
  if retryable := systemErrors.Filter(errors.IsRetryable); retryable.Has() {
    fmt.Println("\n🔄 Retryable Errors:")
    fmt.Println(retryable)
  }
}

func systemErrorFormat(errs []error) string {
  var sb strings.Builder
  sb.WriteString("⚠️ System Alerts:\n")
  for i, err := range errs {
    sb.WriteString(fmt.Sprintf("  %d. %s", i+1, err))
    if errors.IsRetryable(err) {
      sb.WriteString(" (retryable)")
    }
    sb.WriteString("\n")
  }
  return sb.String()
}

func contains(substr string) func(error) bool {
  return func(err error) bool {
    return strings.Contains(err.Error(), substr)
  }
}
```

### Chain Execution

#### Sequential Task Processing
```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
  "time"
)

// validateOrder checks order input.
func validateOrder() error {
  return nil // Simulate successful validation
}

// processKYC handles payment processing.
func processKYC() error {
  return nil // Simulate successful validation
}

// processPayment handles payment processing.
func processPayment() error {
  return errors.New("payment declined") // Simulate payment failure
}

// generateInvoice creates an invoice.
func generateInvoice() error {
  return errors.New("invoicing unavailable") // Simulate invoicing issue
}

// sendNotification sends a confirmation.
func sendNotification() error {
  return errors.New("notification failed") // Simulate notification failure
}

// processOrder simulates a multi-step order processing workflow.
func processOrder() error {
  c := errors.NewChain()

  // Validate order input
  c.Step(validateOrder).Tag("validation")

  // KYC  Process
  c.Step(validateOrder).Tag("validation")

  // Process payment with retries
  c.Step(processPayment).Tag("billing").Retry(3, 100*time.Millisecond)

  // Generate invoice
  c.Step(generateInvoice).Tag("invoicing")

  // Send notification (optional)
  c.Step(sendNotification).Tag("notification").Optional()

  return c.Run()
}

func main() {
  if err := processOrder(); err != nil {
    // Print error to stderr and exit
    errors.Inspect(err)
  }
  fmt.Println("Order processed successfully")
}
```

#### Sequential Task Processing 2
```go
package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/errors"
)

// validate simulates a validation check that fails.
func validate(name string) error {
	return errors.Newf("validation for %s failed", name)
}

// validateOrder checks order input.
func validateOrder() error {
	return nil // Simulate successful validation
}

// verifyKYC handles Know Your Customer verification.
func verifyKYC(name string) error {
	return validate(name) // Simulate KYC validation failure
}

// processPayment handles payment processing.
func processPayment() error {
	return nil // Simulate successful payment
}

// processOrder coordinates the order processing workflow.
func processOrder() error {
	chain := errors.NewChain().
		Step(validateOrder).     // Step 1: Validate order
		Call(verifyKYC, "john"). // Step 2: Verify customer
		Step(processPayment)     // Step 3: Process payment

	if err := chain.Run(); err != nil {
		return errors.Errorf("processing order: %w", err)
	}
	return nil
}

func main() {
	if err := processOrder(); err != nil {
		// Print the full error chain to stderr
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		// Output
		// ERROR: processing order: validation for john failed

		// For debugging, you could print the stack trace:
		// errors.Inspect(err)
		os.Exit(1)
	}

	fmt.Println("order processed successfully")
}

```


#### Retry with Timeout
```go
package main

import (
  "context"
  "fmt"
  "github.com/olekukonko/errors"
  "time"
)

func main() {
  c := errors.NewChain(
    errors.ChainWithTimeout(1*time.Second),
  ).
          Step(func() error {
            time.Sleep(2 * time.Second)
            return errors.New("fetch failed")
          }).
    Tag("api").
    Retry(3, 200*time.Millisecond)

  err := c.Run()
  if err != nil {
    var deadlineErr error
    if errors.As(err, &deadlineErr) && deadlineErr == context.DeadlineExceeded {
      fmt.Println("Fetch timed out")
    } else {
      fmt.Printf("Fetch failed: %v\n", err)
    }
    return
  }
  fmt.Println("Fetch succeeded")
}
```

#### Collecting All Errors
```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
)

func main() {
  c := errors.NewChain(
    errors.ChainWithMaxErrors(2),
  ).
    Step(func() error { return errors.New("task 1 failed") }).Tag("task1").
    Step(func() error { return nil }).Tag("task2").
    Step(func() error { return errors.New("task 3 failed") }).Tag("task3")

  err := c.RunAll()
  if err != nil {
    errors.Inspect(err)
    return
  }
  fmt.Println("All tasks completed successfully")
}

```

---

## Using the `errmgr` Package

### Predefined Errors

#### Static Error
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors/errmgr"
)

func main() {
	// Use a predefined static error
	err := errmgr.ErrNotFound
	fmt.Println(err)        // Output: "not found"
	fmt.Println(err.Code()) // Output: 404
}
```

#### Templated Error
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors/errmgr"
)

func main() {
	// Use a templated error with category
	err := errmgr.ErrDBQuery("SELECT failed")
	fmt.Println(err)           // Output: "database query failed: SELECT failed"
	fmt.Println(err.Category()) // Output: "database"
}
```

### Error Monitoring

#### Basic Monitoring
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors/errmgr"
	"time"
)

func main() {
	// Define and monitor an error
	netErr := errmgr.Define("NetError", "network issue: %s")
	monitor := errmgr.NewMonitor("NetError")
	errmgr.SetThreshold("NetError", 2)
	defer monitor.Close()

	go func() {
		for alert := range monitor.Alerts() {
			fmt.Printf("Alert: %s, count: %d\n", alert.Error(), alert.Count())
		}
	}()

	for i := 0; i < 4; i++ {
		err := netErr(fmt.Sprintf("attempt %d", i))
		err.Free()
	}
	time.Sleep(100 * time.Millisecond)
}
```

#### Realistic Monitoring
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
	"github.com/olekukonko/errors/errmgr"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Define our error types
	netErr := errmgr.Define("NetError", "network connection failed: %s (attempt %d)")
	dbErr := errmgr.Define("DBError", "database operation failed: %s")

	// Create monitors with different buffer sizes
	netMonitor := errmgr.NewMonitorBuffered("NetError", 10) // Larger buffer for network errors
	dbMonitor := errmgr.NewMonitorBuffered("DBError", 5)    // Smaller buffer for DB errors
	defer netMonitor.Close()
	defer dbMonitor.Close()

	// Set different thresholds
	errmgr.SetThreshold("NetError", 3) // Alert after 3 network errors
	errmgr.SetThreshold("DBError", 2)  // Alert after 2 database errors

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Alert handler goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case alert, ok := <-netMonitor.Alerts():
				if !ok {
					fmt.Println("Network alert channel closed")
					return
				}
				handleAlert("NETWORK", alert)
			case alert, ok := <-dbMonitor.Alerts():
				if !ok {
					fmt.Println("Database alert channel closed")
					return
				}
				handleAlert("DATABASE", alert)
			case <-time.After(2 * time.Second):
				// Periodic check for shutdown
				continue
			}
		}
	}()

	// Simulate operations with potential failures
	go func() {
		for i := 1; i <= 15; i++ {
			// Simulate different error scenarios
			if i%4 == 0 {
				// Database error
				err := dbErr("connection timeout")
				fmt.Printf("DB Operation %d: Failed\n", i)
				err.Free()
			} else {
				// Network error
				var errMsg string
				switch {
				case i%3 == 0:
					errMsg = "timeout"
				case i%5 == 0:
					errMsg = "connection reset"
				default:
					errMsg = "unknown error"
				}

				err := netErr(errMsg, i)
				fmt.Printf("Network Operation %d: Failed with %q\n", i, errMsg)
				err.Free()
			}

			// Random delay between operations
			time.Sleep(time.Duration(100+(i%200)) * time.Millisecond)
		}
	}()

	// Wait for shutdown signal or completion
	select {
	case <-sigChan:
		fmt.Println("\nReceived shutdown signal...")
	case <-time.After(5 * time.Second):
		fmt.Println("Completion timeout reached...")
	}

	// Cleanup
	fmt.Println("Initiating shutdown...")
	netMonitor.Close()
	dbMonitor.Close()

	// Wait for the alert handler to finish
	select {
	case <-done:
		fmt.Println("Alert handler shutdown complete")
	case <-time.After(1 * time.Second):
		fmt.Println("Alert handler shutdown timeout")
	}

	fmt.Println("Application shutdown complete")
}

func handleAlert(service string, alert *errors.Error) {
	if alert == nil {
		fmt.Printf("[%s] Received nil alert\n", service)
		return
	}

	fmt.Printf("[%s ALERT] %s (total occurrences: %d)\n",
		service, alert.Error(), alert.Count())

	if alert.Count() > 5 {
		fmt.Printf("[%s CRITICAL] High error rate detected!\n", service)
	}
}
```

---

## Performance Optimization

### Configuration Tuning
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Tune error package configuration
	errors.Configure(errors.Config{
		StackDepth:     32,    // Limit stack frames
		ContextSize:    4,     // Optimize small contexts
		DisablePooling: false, // Enable pooling
	})
	err := errors.New("configured error")
	fmt.Println(err) // Output: "configured error"
}
```

### Using `Free()` for Performance
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Use Free() to return error to pool
	err := errors.New("temp error")
	fmt.Println(err) // Output: "temp error"
	err.Free()       // Immediate pool return, reduces GC pressure
}
```

### Benchmarks
Real performance data (Apple M3 Pro, Go 1.21):
```
goos: darwin
goarch: arm64
pkg: github.com/olekukonko/errors
cpu: Apple M3 Pro
BenchmarkBasic_New-12            99810412    12.00 ns/op    0 B/op    0 allocs/op
BenchmarkStack_WithStack-12      5879510    205.6 ns/op   24 B/op    1 allocs/op
BenchmarkContext_Small-12       29600850    40.34 ns/op   16 B/op    1 allocs/op
BenchmarkWrapping_Simple-12    100000000    11.73 ns/op    0 B/op    0 allocs/op
```
- **New with Pooling**: 12 ns/op, 0 allocations
- **WithStack**: 205 ns/op, minimal allocation
- **Context**: 40 ns/op for small contexts
- Run: `go test -bench=. -benchmem`

## Migration Guide

### From Standard Library
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Before: Standard library error
	err1 := fmt.Errorf("error: %v", "oops")
	fmt.Println(err1)

	// After: Enhanced error with context and stack
	err2 := errors.Newf("error: %v", "oops").
		With("source", "api").
		WithStack()
	fmt.Println(err2)
}
```

### From `pkg/errors`
```go
package main

import (
  "fmt"
  "github.com/olekukonko/errors"
)

func main() {
  // Before: pkg/errors (assuming similar API)
  // err := pkgerrors.Wrap(err, "context")

  // After: Enhanced wrapping
  err := errors.New("low-level").
    Msgf("context: %s", "details").
    WithStack()
  fmt.Println(err)
}
```

### Compatibility with `errors.Is` and `errors.As`
```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Check compatibility with standard library
	err := errors.Named("MyError")
	wrapped := errors.Wrapf(err, "outer")
	if errors.Is(wrapped, err) { // Stdlib compatible
		fmt.Println("Matches MyError") // Output: "Matches MyError"
	}
}
```

## FAQ

- **When to use `Copy()`?**
  - Use ` SOCIALCopy()` to create a modifiable duplicate of an error without altering the original.

- **When to use `Free()`?**
  - Use in performance-critical loops; otherwise, autofree handles it (Go 1.24+).

- **How to handle cleanup?**
  - Use `Callback()` for automatic actions like rollbacks or logging.

- **How to add stack traces later?**
  - Use `WithStack()` to upgrade a simple error:
    ```go
    package main

    import (
        "fmt"
        "github.com/olekukonko/errors"
    )

    func main() {
        err := errors.New("simple")
        err = err.WithStack()
        fmt.Println(err.Stack())
    }
    ```

## Contributing
- Fork, branch, commit, and PR—see [CONTRIBUTING.md](#).

## License
MIT License - See [LICENSE](LICENSE).
