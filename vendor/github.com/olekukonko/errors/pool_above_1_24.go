//go:build go1.24
// +build go1.24

package errors

import "runtime"

// setupCleanup configures a cleanup function for an *Error to auto-return it to the pool.
// Only active for Go 1.24+; uses runtime.AddCleanup when autoFree is set and pooling is enabled.
func (ep *ErrorPool) setupCleanup(e *Error) {
	if currentConfig.autoFree {
		runtime.AddCleanup(e, func(_ *struct{}) {
			if !currentConfig.disablePooling {
				ep.Put(e) // Return to pool when cleaned up
			}
		}, nil) // No additional context needed
	}
}

// clearCleanup is a no-op for Go 1.24 and above.
// Cleanup is managed by runtime.AddCleanup; no explicit removal is required.
func (ep *ErrorPool) clearCleanup(e *Error) {
	// No-op for Go 1.24+
}
