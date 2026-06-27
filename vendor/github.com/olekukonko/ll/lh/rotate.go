package lh

import (
	"io"
	"sync"

	"github.com/olekukonko/ll/lx"
)

// RotateSource defines the callbacks needed to implement log rotation.
// It abstracts the destination lifecycle: opening, sizing, and rotating.
//
// Example for file rotation:
//
//	src := lh.RotateSource{
//		Open: func() (io.WriteCloser, error) {
//			return os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
//		},
//		Size: func() (int64, error) {
//			if fi, err := os.Stat("app.log"); err == nil {
//				return fi.Size(), nil
//			}
//			return 0, nil // File doesn't exist yet
//		},
//		Rotate: func() error {
//			// Rename current log before creating new one
//			return os.Rename("app.log", "app.log."+time.Now().Format("20060102-150405"))
//		},
//	}
type RotateSource struct {
	// Open returns a fresh destination for log output.
	// Called on initialization and after rotation.
	Open func() (io.WriteCloser, error)

	// Size returns the current size in bytes of the active destination.
	// Return an error if size cannot be determined (rotation will be skipped).
	Size func() (int64, error)

	// Rotate performs cleanup/rotation actions before opening a new destination.
	// For files: rename or move the current log. Optional for other destinations.
	Rotate func() error
}

// Rotating wraps a handler to rotate its output when maxSize is exceeded.
// The wrapped handler must implement both Handler and Outputter interfaces.
// Rotation is triggered on each Handle call if the current size >= maxSize.
//
// Example:
//
//	handler := lx.NewJSONHandler(os.Stdout)
//	src := lh.RotateSource{...} // see RotateSource example
//	rotator, err := lh.NewRotating(handler, 10*1024*1024, src) // 10 MB
//	logger := lx.NewLogger(rotator)
//	logger.Info("This log may trigger rotation when file reaches 10MB")
type Rotating[H interface {
	lx.Handler
	lx.Outputter
}] struct {
	mu      sync.Mutex
	maxSize int64
	src     RotateSource

	out     io.WriteCloser
	handler H
}

// NewRotating creates a rotating wrapper around handler.
// Handler's output will be replaced with destinations from src.Open.
// If maxSizeBytes <= 0, rotation is disabled.
// src.Rotate may be nil if no pre-open actions are needed.
//
// Example:
//
//	// Create a JSON handler that rotates at 5MB
//	handler := lx.NewJSONHandler(os.Stdout)
//	rotator, err := lh.NewRotating(handler, 5*1024*1024, src)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use rotator as your logger's handler
//	logger := lx.NewLogger(rotator)
func NewRotating[H interface {
	lx.Handler
	lx.Outputter
}](handler H, maxSizeBytes int64, src RotateSource) (*Rotating[H], error) {
	r := &Rotating[H]{
		maxSize: maxSizeBytes,
		src:     src,
		handler: handler,
	}
	if err := r.reopenLocked(); err != nil {
		return nil, err
	}
	return r, nil
}

// Handle processes a log entry, rotating output if necessary.
// Thread-safe: can be called concurrently.
//
// Example:
//
//	rotator.Handle(&lx.Entry{
//	    Level:     lx.InfoLevel,
//	    Message:   "Processing request",
//	    Namespace: "api",
//	})
func (r *Rotating[H]) Handle(e *lx.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.rotateIfNeededLocked(); err != nil {
		return err
	}
	return r.handler.Handle(e)
}

// Close releases resources (closes the current output).
// Safe to call multiple times.
//
// Example:
//
//	defer rotator.Close()
func (r *Rotating[H]) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.out != nil {
		return r.out.Close()
	}
	return nil
}

// rotateIfNeededLocked checks current size and rotates if maxSize exceeded.
// Called with mu already held.
func (r *Rotating[H]) rotateIfNeededLocked() error {
	if r.maxSize <= 0 || r.src.Size == nil || r.src.Open == nil {
		return nil
	}

	size, err := r.src.Size()
	if err != nil {
		// Size unknown - skip rotation
		return nil
	}
	if size < r.maxSize {
		return nil
	}

	// Close current output
	if r.out != nil {
		_ = r.out.Close()
		r.out = nil
	}

	// Run rotation hook (rename/move/commit)
	if r.src.Rotate != nil {
		if err := r.src.Rotate(); err != nil {
			return err
		}
	}

	// Open fresh output
	return r.reopenLocked()
}

// reopenLocked opens a new destination and sets it on the handler.
// Called with mu already held.
func (r *Rotating[H]) reopenLocked() error {
	out, err := r.src.Open()
	if err != nil {
		return err
	}
	r.out = out
	r.handler.Output(out)
	return nil
}
