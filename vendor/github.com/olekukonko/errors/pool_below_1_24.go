//go:build !go1.24
// +build !go1.24

package errors

import "runtime"

// setupCleanup configures a finalizer for an *Error to auto-return it to the pool.
// Only active for Go versions < 1.24; enables automatic cleanup when autoFree is set and pooling is enabled.
func (ep *ErrorPool) setupCleanup(e *Error) {
	if currentConfig.autoFree {
		runtime.SetFinalizer(e, func(e *Error) {
			if !currentConfig.disablePooling {
				ep.Put(e) // Return to pool when garbage collected
			}
		})
	}
}

// clearCleanup removes any finalizer set on an *Error.
// Only active for Go versions < 1.24; ensures no cleanup action occurs on garbage collection.
func (ep *ErrorPool) clearCleanup(e *Error) {
	runtime.SetFinalizer(e, nil) // Disable finalizer
}
