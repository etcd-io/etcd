package lh

import (
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/ll/lx"
)

// Pipe chains multiple handler wrappers together, applying them from left to right.
// The wrappers are composed such that the first wrapper in the list becomes
// the innermost layer, and the last wrapper becomes the outermost layer.
//
// Usage pattern: Pipe(baseHandler, wrapper1, wrapper2, wrapper3)
// Result: wrapper3(wrapper2(wrapper1(baseHandler)))
//
// This enables clean, declarative construction of handler middleware chains.
//
// Example - building a processing pipeline:
//
//	base := lx.NewJSONHandler(os.Stdout)
//	handler := lh.Pipe(base,
//	    lh.NewDedup(2*time.Second),    // 1. Deduplicate first
//	    lh.NewRateLimit(10, time.Second), // 2. Then rate limit
//	)
//	logger := lx.NewLogger(handler)
//
// In this example, logs flow: Dedup → RateLimit → AddTimestamp → JSONHandler
func Pipe(h lx.Handler, wraps ...lx.Wrap) lx.Handler {
	for _, w := range wraps {
		if w != nil {
			h = w(h)
		}
	}
	return h
}

// PipeDedup returns a wrapper that applies deduplication to the handler.
func PipeDedup(ttl time.Duration, opts ...DedupOpt[lx.Handler]) lx.Wrap {
	return func(next lx.Handler) lx.Handler {
		return NewDedup(next, ttl, opts...)
	}
}

// PipeBuffer returns a wrapper that applies buffering to the handler.
func PipeBuffer(opts ...BufferingOpt) lx.Wrap {
	return func(next lx.Handler) lx.Handler {
		return NewBuffered(next, opts...)
	}
}

// PipeRotate returns a wrapper that applies log rotation.
// Ideally, the 'next' handler should be one that writes to a file (like TextHandler or JSONHandler).
//
// If the underlying handler does not implement lx.HandlerOutputter (cannot change output destination),
// or if rotation initialization fails, this will log a warning to stderr and return the
// original handler unmodified to prevent application crashes.
func PipeRotate(maxSizeBytes int64, src RotateSource) lx.Wrap {
	return func(next lx.Handler) lx.Handler {
		// Attempt to cast to HandlerOutputter (Handler + Outputter interface)
		h, ok := next.(lx.HandlerOutputter)
		if !ok {
			fmt.Fprintf(os.Stderr, "ll/lh: PipeRotate skipped - handler does not implement SetOutput(io.Writer)\n")
			return next
		}

		// Initialize the rotating handler
		r, err := NewRotating(h, maxSizeBytes, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ll/lh: PipeRotate initialization failed: %v\n", err)
			return next
		}
		return r
	}
}
