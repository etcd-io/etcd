package lh

import (
	"context"
	"log/slog"

	"github.com/olekukonko/ll/lx"
)

// SlogHandler adapts a slog.Handler to implement lx.Handler.
// It converts lx.Entry objects to slog.Record objects and delegates to an underlying
// slog.Handler for processing, enabling compatibility with Go's standard slog package.
// Thread-safe if the underlying slog.Handler is thread-safe.
type SlogHandler struct {
	slogHandler slog.Handler // Underlying slog.Handler for processing log records
}

// NewSlogHandler creates a new SlogHandler wrapping the provided slog.Handler.
// It initializes the handler with the given slog.Handler, allowing lx.Entry logs to be
// processed by slog's logging infrastructure.
// Example:
//
//	slogText := slog.NewTextHandler(os.Stdout, nil)
//	handler := NewSlogHandler(slogText)
//	logger := ll.New("app").Enable().Handler(handler)
//	logger.Info("Test") // Output: level=INFO msg=Test namespace=app class=Text
func NewSlogHandler(h slog.Handler) *SlogHandler {
	return &SlogHandler{slogHandler: h}
}

// Handle converts an lx.Entry to slog.Record and delegates to the slog.Handler.
// It maps the entry's fields, level, namespace, class, and stack trace to slog attributes,
// passing the resulting record to the underlying slog.Handler.
// Returns an error if the slog.Handler fails to process the record.
// Thread-safe if the underlying slog.Handler is thread-safe.
// Example:
//
//	handler.Handle(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Processes as slog record
//
// Handle converts an lx.Entry to slog.Record and delegates to the slog.Handler.
// It maps the entry's fields, level, namespace, class, and stack trace to slog attributes,
// passing the resulting record to the underlying slog.Handler.
// Returns an error if the slog.Handler fails to process the record.
// Thread-safe if the underlying slog.Handler is thread-safe.
// Example:
//
//	handler.Handle(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Processes as slog record
func (h *SlogHandler) Handle(e *lx.Entry) error {
	// Convert lx.LevelType to slog.Level
	level := toSlogLevel(e.Level)

	// Create a slog.Record with the entry's data
	record := slog.NewRecord(
		e.Timestamp, // time.Time for log timestamp
		level,       // slog.Level for log severity
		e.Message,   // string for log message
		0,           // pc (program counter, optional, not used)
	)

	// Add standard fields as attributes
	record.AddAttrs(
		slog.String("namespace", e.Namespace),  // Add namespace as string attribute
		slog.String("class", e.Class.String()), // Add class as string attribute
	)

	// Add stack trace if present
	if len(e.Stack) > 0 {
		record.AddAttrs(slog.String("stack", string(e.Stack))) // Add stack trace as string
	}

	// Add custom fields in order (preserving insertion order)
	for _, pair := range e.Fields {
		record.AddAttrs(slog.Any(pair.Key, pair.Value)) // Add each field as a key-value attribute
	}

	// Handle the record with the underlying slog.Handler
	return h.slogHandler.Handle(context.Background(), record)
}

// toSlogLevel converts lx.LevelType to slog.Level.
// It maps the logging levels used by the lx package to those used by slog,
// defaulting to slog.LevelInfo for unknown levels.
// Example (internal usage):
//
//	level := toSlogLevel(lx.LevelDebug) // Returns slog.LevelDebug
func toSlogLevel(level lx.LevelType) slog.Level {
	switch level {
	case lx.LevelDebug:
		return slog.LevelDebug
	case lx.LevelInfo:
		return slog.LevelInfo
	case lx.LevelWarn:
		return slog.LevelWarn
	case lx.LevelError, lx.LevelFatal:
		return slog.LevelError
	default:
		return slog.LevelInfo // Default for unknown levels
	}
}
