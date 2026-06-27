// pool.go
package errors

import (
	"sync"
	"sync/atomic"
)

// ErrorPool is a high-performance, thread-safe pool for reusing *Error instances.
// Reduces allocation overhead by recycling errors; tracks hit/miss statistics.
type ErrorPool struct {
	pool      sync.Pool // Underlying pool for storing *Error instances
	poolStats struct {  // Embedded struct for pool usage statistics
		hits   atomic.Int64 // Number of times an error was reused from the pool
		misses atomic.Int64 // Number of times a new error was created due to pool miss
	}
}

// NewErrorPool creates a new ErrorPool instance.
// Initializes the pool with a New function that returns a fresh *Error with default smallContext.
func NewErrorPool() *ErrorPool {
	return &ErrorPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Error{
					smallContext: [contextSize]contextItem{},
				}
			},
		},
	}
}

// Get retrieves an *Error from the pool or creates a new one if pooling is disabled or pool is empty.
// Resets are handled by Put; thread-safe; updates hit/miss stats when pooling is enabled.
func (ep *ErrorPool) Get() *Error {
	if currentConfig.disablePooling {
		return &Error{
			smallContext: [contextSize]contextItem{},
		}
	}

	e := ep.pool.Get().(*Error)
	if e == nil { // Pool returned nil (unlikely due to New func, but handled for safety)
		ep.poolStats.misses.Add(1)
		return &Error{
			smallContext: [contextSize]contextItem{},
		}
	}
	ep.poolStats.hits.Add(1)
	return e
}

// Put returns an *Error to the pool after resetting it.
// Ignores nil errors or if pooling is disabled; preserves stack capacity; thread-safe.
func (ep *ErrorPool) Put(e *Error) {
	if e == nil || currentConfig.disablePooling {
		return
	}

	// Reset the error to a clean state, preserving capacity
	e.Reset()

	// Reset stack length while keeping capacity for reuse
	if e.stack != nil {
		e.stack = e.stack[:0]
	}

	ep.pool.Put(e)
}

// Stats returns the current pool statistics as hits and misses.
// Thread-safe; uses atomic loads to ensure accurate counts.
func (ep *ErrorPool) Stats() (hits, misses int64) {
	return ep.poolStats.hits.Load(), ep.poolStats.misses.Load()
}
