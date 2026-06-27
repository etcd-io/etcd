package lh

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/olekukonko/ll/lx"
)

// Buffering holds configuration for the Buffered handler.
type Buffering struct {
	BatchSize     int           // Flush when this many entries are buffered (default: 100)
	FlushInterval time.Duration // Maximum time between flushes (default: 10s)
	MaxBuffer     int           // Maximum buffer size before applying backpressure (default: 1000)
	OnOverflow    func(int)     // Called when buffer reaches MaxBuffer (default: logs warning)
	ErrorOutput   io.Writer     // Destination for internal errors like flush failures (default: os.Stderr)
}

// BufferingOpt configures Buffered handler.
type BufferingOpt func(*Buffering)

// WithBatchSize sets the batch size for flushing.
// It specifies the number of log entries to buffer before flushing to the underlying handler.
// Example:
//
//	handler := NewBuffered(textHandler, WithBatchSize(50)) // Flush every 50 entries
func WithBatchSize(size int) BufferingOpt {
	return func(c *Buffering) {
		c.BatchSize = size
	}
}

// WithFlushInterval sets the maximum time between flushes.
// It defines the interval at which buffered entries are flushed, even if the batch size is not reached.
// Example:
//
//	handler := NewBuffered(textHandler, WithFlushInterval(5*time.Second)) // Flush every 5 seconds
func WithFlushInterval(d time.Duration) BufferingOpt {
	return func(c *Buffering) {
		c.FlushInterval = d
	}
}

// WithMaxBuffer sets the maximum buffer size before backpressure.
// It limits the number of entries that can be queued in the channel, triggering overflow handling if exceeded.
// Example:
//
//	handler := NewBuffered(textHandler, WithMaxBuffer(500)) // Allow up to 500 buffered entries
func WithMaxBuffer(size int) BufferingOpt {
	return func(c *Buffering) {
		c.MaxBuffer = size
	}
}

// WithOverflowHandler sets the overflow callback.
// It specifies a function to call when the buffer reaches MaxBuffer, typically for logging or metrics.
// Example:
//
//	handler := NewBuffered(textHandler, WithOverflowHandler(func(n int) { fmt.Printf("Overflow: %d entries\n", n) }))
func WithOverflowHandler(fn func(int)) BufferingOpt {
	return func(c *Buffering) {
		c.OnOverflow = fn
	}
}

// WithErrorOutput sets the destination for internal errors (e.g., downstream handler failures).
// Defaults to os.Stderr if not set.
// Example:
//
//	// Redirect internal errors to a file or discard them
//	handler := NewBuffered(textHandler, WithErrorOutput(os.Stdout))
func WithErrorOutput(w io.Writer) BufferingOpt {
	return func(c *Buffering) {
		c.ErrorOutput = w
	}
}

// Buffered wraps any Handler to provide buffering capabilities.
// It buffers log entries in a channel and flushes them based on batch size, time interval, or explicit flush.
// The generic type H ensures compatibility with any lx.Handler implementation.
// Thread-safe via channels and sync primitives.
type Buffered[H lx.Handler] struct {
	handler      H              // Underlying handler to process log entries
	config       *Buffering     // Configuration for batching and flushing
	entries      chan *lx.Entry // Channel for buffering log entries
	flushSignal  chan struct{}  // Channel to trigger explicit flushes
	shutdown     chan struct{}  // Channel to signal worker shutdown
	shutdownOnce sync.Once      // Ensures Close is called only once
	wg           sync.WaitGroup // Waits for worker goroutine to finish
}

// NewBuffered creates a new buffered handler that wraps another handler.
// It initializes the handler with default or provided configuration options and starts a worker goroutine.
// Thread-safe via channel operations and finalizer for cleanup.
// Example:
//
//	textHandler := lh.NewTextHandler(os.Stdout)
//	buffered := NewBuffered(textHandler, WithBatchSize(50))
func NewBuffered[H lx.Handler](handler H, opts ...BufferingOpt) *Buffered[H] {
	// Initialize default configuration
	config := &Buffering{
		BatchSize:     100,              // Default: flush every 100 entries
		FlushInterval: 10 * time.Second, // Default: flush every 10 seconds
		MaxBuffer:     1000,             // Default: max 1000 entries in buffer
		ErrorOutput:   os.Stderr,        // Default: report errors to stderr
		OnOverflow: func(count int) { // Default: log overflow to io.Discard (silent by default for overflow)
			fmt.Fprintf(io.Discard, "log buffer overflow: %d entries\n", count)
		},
	}

	// Apply provided options
	for _, opt := range opts {
		opt(config)
	}

	// Ensure sane configuration values
	if config.BatchSize < 1 {
		config.BatchSize = 1 // Minimum batch size is 1
	}
	if config.MaxBuffer < config.BatchSize {
		config.MaxBuffer = config.BatchSize * 10 // Ensure buffer is at least 10x batch size
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 10 * time.Second // Minimum flush interval is 10s
	}
	if config.ErrorOutput == nil {
		config.ErrorOutput = os.Stderr
	}

	// Initialize Buffered handler
	b := &Buffered[H]{
		handler:     handler,                                // Set underlying handler
		config:      config,                                 // Set configuration
		entries:     make(chan *lx.Entry, config.MaxBuffer), // Create buffered channel
		flushSignal: make(chan struct{}, 1),                 // Create single-slot flush signal channel
		shutdown:    make(chan struct{}),                    // Create shutdown signal channel
	}

	// Start worker goroutine
	b.wg.Add(1)
	go b.worker()

	// Set finalizer for cleanup during garbage collection
	runtime.SetFinalizer(b, (*Buffered[H]).Final)
	return b
}

// Handle implements the lx.Handler interface.
// It buffers log entries in the entries channel or triggers a flush on overflow.
// Returns an error if the buffer is full and flush cannot be triggered.
// Thread-safe via non-blocking channel operations.
// Example:
//
//	buffered.Handle(&lx.Entry{Message: "test"}) // Buffers entry or triggers flush
func (b *Buffered[H]) Handle(e *lx.Entry) error {
	select {
	case b.entries <- e: // Buffer entry if channel has space
		return nil
	default: // Handle buffer overflow
		if b.config.OnOverflow != nil {
			b.config.OnOverflow(len(b.entries)) // Call overflow handler
		}
		select {
		case b.flushSignal <- struct{}{}: // Trigger flush if possible
			return fmt.Errorf("log buffer overflow, triggering flush")
		default: // Flush already in progress
			return fmt.Errorf("log buffer overflow and flush already in progress")
		}
	}
}

// Flush triggers an immediate flush of buffered entries.
// It sends a signal to the worker to process all buffered entries.
// If a flush is already pending, it waits briefly and may exit without flushing.
// Thread-safe via non-blocking channel operations.
// Example:
//
//	buffered.Flush() // Flushes all buffered entries
func (b *Buffered[H]) Flush() {
	select {
	case b.flushSignal <- struct{}{}: // Signal worker to flush
	case <-time.After(100 * time.Millisecond): // Timeout if flush is pending
		// Flush already pending
	}
}

// Close flushes any remaining entries and stops the worker.
// It ensures shutdown is performed only once and waits for the worker to finish.
// If the underlying handler implements a Close() error method, it will be called to release resources.
// Thread-safe via sync.Once and WaitGroup.
// Returns any error from the underlying handler's Close, or nil.
// Example:
//
//	buffered.Close() // Flushes entries and stops worker
func (b *Buffered[H]) Close() error {
	var closeErr error
	b.shutdownOnce.Do(func() {
		close(b.shutdown)            // Signal worker to shut down
		b.wg.Wait()                  // Wait for worker to finish
		runtime.SetFinalizer(b, nil) // Remove finalizer

		// Check if underlying handler has a Close method and call it
		if closer, ok := any(b.handler).(interface{ Close() error }); ok {
			closeErr = closer.Close()
		}
	})
	return closeErr
}

// Final ensures remaining entries are flushed during garbage collection.
// It calls Close to flush entries and stop the worker.
// Used as a runtime finalizer to prevent log loss.
// Example (internal usage):
//
//	runtime.SetFinalizer(buffered, (*Buffered[H]).Final)
func (b *Buffered[H]) Final() {
	b.Close()
}

// Config returns the current configuration of the Buffered handler.
// It provides access to BatchSize, FlushInterval, MaxBuffer, and OnOverflow settings.
// Example:
//
//	config := buffered.Config() // Access configuration
func (b *Buffered[H]) Config() *Buffering {
	return b.config
}

// worker processes entries and handles flushing.
// It runs in a goroutine, buffering entries, flushing on batch size, timer, or explicit signal,
// and shutting down cleanly when signaled.
// Thread-safe via channel operations and WaitGroup.
func (b *Buffered[H]) worker() {
	defer b.wg.Done()                                 // Signal completion when worker exits
	batch := make([]*lx.Entry, 0, b.config.BatchSize) // Buffer for batching entries
	ticker := time.NewTicker(b.config.FlushInterval)  // Timer for periodic flushes
	defer ticker.Stop()                               // Clean up ticker
	for {
		select {
		case entry := <-b.entries: // Receive new entry
			batch = append(batch, entry)
			// Flush if batch size is reached
			if len(batch) >= b.config.BatchSize {
				b.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C: // Periodic flush
			if len(batch) > 0 {
				b.flushBatch(batch)
				batch = batch[:0]
			}
		case <-b.flushSignal: // Explicit flush
			if len(batch) > 0 {
				b.flushBatch(batch)
				batch = batch[:0]
			}
			b.drainRemaining() // Drain all entries from the channel
		case <-b.shutdown: // Shutdown signal
			if len(batch) > 0 {
				b.flushBatch(batch)
			}
			b.drainRemaining() // Flush remaining entries
			return
		}
	}
}

// flushBatch processes a batch of entries through the wrapped handler.
// It writes each entry to the underlying handler, logging any errors to the configured ErrorOutput.
// Example (internal usage):
//
//	b.flushBatch([]*lx.Entry{entry1, entry2})
func (b *Buffered[H]) flushBatch(batch []*lx.Entry) {
	for _, entry := range batch {
		// Process each entry through the handler
		if err := b.handler.Handle(entry); err != nil {
			if b.config.ErrorOutput != nil {
				fmt.Fprintf(b.config.ErrorOutput, "log flush error: %v\n", err)
			}
		}
	}
}

// drainRemaining processes any remaining entries in the channel.
// It flushes all entries from the entries channel to the underlying handler,
// logging any errors to the configured ErrorOutput. Used during flush or shutdown.
// Example (internal usage):
//
//	b.drainRemaining() // Flushes all pending entries
func (b *Buffered[H]) drainRemaining() {
	for {
		select {
		case entry := <-b.entries: // Process next entry
			if err := b.handler.Handle(entry); err != nil {
				if b.config.ErrorOutput != nil {
					fmt.Fprintf(b.config.ErrorOutput, "log drain error: %v\n", err)
				}
			}
		default: // Exit when channel is empty
			return
		}
	}
}
