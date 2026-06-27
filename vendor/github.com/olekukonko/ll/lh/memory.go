package lh

import (
	"fmt"
	"io"
	"sync"

	"github.com/olekukonko/ll/lx"
)

// MemoryHandler is an lx.Handler that stores log entries in memory.
// Useful for testing or buffering logs for later inspection.
// It maintains a thread-safe slice of log entries, protected by a read-write mutex.
type MemoryHandler struct {
	mu         sync.RWMutex // Protects concurrent access to entries
	entries    []*lx.Entry  // Slice of stored log entries
	showTime   bool         // Whether to show timestamps when dumping
	timeFormat string       // Time format for dumping
}

// NewMemoryHandler creates a new MemoryHandler.
// It initializes an empty slice for storing log entries, ready for use in logging or testing.
// Example:
//
//	handler := NewMemoryHandler()
//	logger := ll.New("app").Enable().Handler(handler)
//	logger.Info("Test") // Stores entry in memory
func NewMemoryHandler() *MemoryHandler {
	return &MemoryHandler{
		entries: make([]*lx.Entry, 0), // Initialize empty slice for entries
	}
}

// Timestamped enables/disables timestamp display when dumping and optionally sets a time format.
// Consistent with TextHandler and ColorizedHandler signature.
// Example:
//
//	handler.Timestamped(true) // Enable with default format
//	handler.Timestamped(true, time.StampMilli) // Enable with custom format
//	handler.Timestamped(false) // Disable
func (h *MemoryHandler) Timestamped(enable bool, format ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.showTime = enable
	if len(format) > 0 && format[0] != "" {
		h.timeFormat = format[0]
	}
}

// Handle stores the log entry in memory.
// It appends the provided entry to the entries slice, ensuring thread-safety with a write lock.
// Always returns nil, as it does not perform I/O operations.
// Example:
//
//	handler.Handle(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Stores entry
func (h *MemoryHandler) Handle(entry *lx.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, entry) // Append entry to slice
	return nil
}

// Entries returns a copy of the stored log entries.
// It creates a new slice with copies of all entries, ensuring thread-safety with a read lock.
// The returned slice is safe for external use without affecting the handler's internal state.
// Example:
//
//	entries := handler.Entries() // Returns copy of stored entries
func (h *MemoryHandler) Entries() []*lx.Entry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	entries := make([]*lx.Entry, len(h.entries)) // Create new slice for copy
	copy(entries, h.entries)                     // Copy entries to new slice
	return entries
}

// Reset clears all stored entries.
// It truncates the entries slice to zero length, preserving capacity, using a write lock for thread-safety.
// Example:
//
//	handler.Reset() // Clears all stored entries
func (h *MemoryHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = h.entries[:0] // Truncate slice to zero length
}

// Dump writes all stored log entries to the provided io.Writer in text format.
// Entries are formatted as they would be by a TextHandler, including namespace, level,
// message, and fields. Thread-safe with read lock.
// Returns an error if writing fails.
// Example:
//
//	logger := ll.New("test", ll.WithHandler(NewMemoryHandler())).Enable()
//	logger.Info("Test message")
//	handler := logger.handler.(*MemoryHandler)
//	handler.Dump(os.Stdout) // Output: [test] INFO: Test message
func (h *MemoryHandler) Dump(w io.Writer) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Create a temporary TextHandler to format entries
	tempHandler := NewTextHandler(w)
	tempHandler.Timestamped(h.showTime, h.timeFormat)

	// Process each entry through the TextHandler
	for _, entry := range h.entries {
		if err := tempHandler.Handle(entry); err != nil {
			return fmt.Errorf("failed to dump entry: %writer", err) // Wrap and return write errors
		}
	}
	return nil
}
