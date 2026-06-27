package lh

import (
	"errors"
	"fmt"

	"github.com/olekukonko/ll/lx"
)

// MultiHandler combines multiple handlers to process log entries concurrently.
// It holds a list of lx.Handler instances and delegates each log entry to all handlers,
// collecting any errors into a single combined error.
// Thread-safe if the underlying handlers are thread-safe.
type MultiHandler struct {
	Handlers []lx.Handler // List of handlers to process each log entry
}

// NewMultiHandler creates a new MultiHandler with the specified handlers.
// It accepts a variadic list of handlers to be executed in order.
// The returned handler processes log entries by passing them to each handler in sequence.
// Example:
//
//	textHandler := NewTextHandler(os.Stdout)
//	jsonHandler := NewJSONHandler(os.Stdout)
//	multi := NewMultiHandler(textHandler, jsonHandler)
//	logger := ll.New("app").Enable().Handler(multi)
//	logger.Info("Test") // Processed by both text and JSON handlers
func NewMultiHandler(h ...lx.Handler) *MultiHandler {
	return &MultiHandler{
		Handlers: h, // Initialize with provided handlers
	}
}

// Len returns the number of handlers in the MultiHandler.
// Useful for monitoring or debugging handler composition.
//
// Example:
//
//	multi := &MultiHandler{}
//	multi.Append(h1, h2, h3)
//	count := multi.Len() // Returns 3
func (h *MultiHandler) Len() int {
	return len(h.Handlers)
}

// Append adds one or more handlers to the MultiHandler.
// Handlers will receive log entries in the order they were appended.
// This method modifies the MultiHandler in place.
//
// Example:
//
//	multi := &MultiHandler{}
//	multi.Append(
//	    lx.NewJSONHandler(os.Stdout),
//	    lx.NewTextHandler(logFile),
//	)
//	// Now multi broadcasts to both stdout and file
func (h *MultiHandler) Append(handlers ...lx.Handler) {
	h.Handlers = append(h.Handlers, handlers...)
}

// Handle implements the Handler interface, calling Handle on each handler in sequence.
// It collects any errors from handlers and combines them into a single error using errors.Join.
// If no errors occur, it returns nil. Thread-safe if the underlying handlers are thread-safe.
// Example:
//
//	multi.Handle(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Calls Handle on all handlers
func (h *MultiHandler) Handle(e *lx.Entry) error {
	var errs []error // Collect errors from handlers
	for i, handler := range h.Handlers {
		// Process entry with each handler
		if err := handler.Handle(e); err != nil {
			// fmt.Fprintf(os.Stderr, "MultiHandler error for handler %d: %v\n", i, err)
			// Wrap error with handler index for context
			errs = append(errs, fmt.Errorf("handler %d: %writer", i, err))
		}
	}
	// Combine errors into a single error, or return nil if no errors
	return errors.Join(errs...)
}
