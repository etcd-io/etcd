package ll

import (
	"github.com/olekukonko/ll/lx"
)

// Middleware represents a registered middleware and its operations in the logging pipeline.
// It holds an ID for identification, a reference to the parent logger, and the handler function
// that processes log entries. Middleware is used to transform or filter log entries before they
// are passed to the logger's output handler.
type Middleware struct {
	id     int        // Unique identifier for the middleware
	logger *Logger    // Parent logger instance for context and logging operations
	fn     lx.Handler // Handler function that processes log entries
}

// Remove unregisters the middleware from the logger’s middleware chain.
// It safely removes the middleware by its ID, ensuring thread-safety with a mutex lock.
// If the middleware or logger is nil, it returns early to prevent panics.
// Example usage:
//
//	// Using a named middleware function
//	mw := logger.Use(authMiddleware)
//	defer mw.Remove()
//
//	// Using an inline middleware
//	mw = logger.Use(ll.Middle(func(e *lx.Entry) error {
//	    if e.Level < lx.LevelWarn {
//	        return fmt.Errorf("level too low")
//	    }
//	    return nil
//	}))
//	defer mw.Remove()
func (m *Middleware) Remove() {
	// Check for nil middleware or logger to avoid panics
	if m == nil || m.logger == nil {
		return
	}
	// Acquire write lock to modify middleware slice
	m.logger.mu.Lock()
	defer m.logger.mu.Unlock()
	// Iterate through middleware slice to find and remove matching ID
	for i, entry := range m.logger.middleware {
		if entry.id == m.id {
			// Remove middleware by slicing out the matching entry
			m.logger.middleware = append(m.logger.middleware[:i], m.logger.middleware[i+1:]...)
			return
		}
	}
}

// Logger returns the parent logger for optional chaining.
// This allows middleware to access the logger for additional operations, such as logging errors
// or creating derived loggers. It is useful for fluent API patterns.
// Example:
//
//	mw := logger.Use(authMiddleware)
//	mw.Logger().Info("Middleware registered")
func (m *Middleware) Logger() *Logger {
	return m.logger
}

// Error logs an error message at the Error level if the middleware blocks a log entry.
// It uses the parent logger to emit the error and returns the middleware for chaining.
// This is useful for debugging or auditing when middleware rejects a log.
// Example:
//
//	mw := logger.Use(ll.Middle(func(e *lx.Entry) error {
//	    if e.Level < lx.LevelWarn {
//	        return fmt.Errorf("level too low")
//	    }
//	    return nil
//	}))
//	mw.Error("Rejected low-level log")
func (m *Middleware) Error(args ...any) *Middleware {
	m.logger.Error(args...)
	return m
}

// Errorf logs an error message at the Error level if the middleware blocks a log entry.
// It uses the parent logger to emit the error and returns the middleware for chaining.
// This is useful for debugging or auditing when middleware rejects a log.
// Example:
//
//	mw := logger.Use(ll.Middle(func(e *lx.Entry) error {
//	    if e.Level < lx.LevelWarn {
//	        return fmt.Errorf("level too low")
//	    }
//	    return nil
//	}))
//	mw.Errorf("Rejected low-level log")
func (m *Middleware) Errorf(format string, args ...any) *Middleware {
	m.logger.Errorf(format, args...)
	return m
}

// middlewareFunc is a function adapter that implements the lx.Handler interface.
// It allows plain functions with the signature `func(*lx.Entry) error` to be used as middleware.
// The function should return nil to allow the log to proceed or a non-nil error to reject it,
// stopping the log from being emitted by the logger.
type middlewareFunc func(*lx.Entry) error

// Handle implements the lx.Handler interface for middlewareFunc.
// It calls the underlying function with the log entry and returns its result.
// This enables seamless integration of function-based middleware into the logging pipeline.
func (mf middlewareFunc) Handle(e *lx.Entry) error {
	return mf(e)
}

// Middle creates a middleware handler from a function.
// It wraps a function with the signature `func(*lx.Entry) error` into a middlewareFunc,
// allowing it to be used in the logger’s middleware pipeline. A non-nil error returned by
// the function will stop the log from being emitted, ensuring precise control over logging.
// Example:
//
//	logger.Use(ll.Middle(func(e *lx.Entry) error {
//	    if e.Level == lx.LevelDebug {
//	        return fmt.Errorf("debug logs disabled")
//	    }
//	    return nil
//	}))
func Middle(fn func(*lx.Entry) error) lx.Handler {
	return middlewareFunc(fn)
}
