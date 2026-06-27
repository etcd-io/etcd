package lx

import "io"

// Handler defines the interface for processing log entries.
// Implementations (e.g., TextHandler, JSONHandler) format and output log entries to various
// destinations (e.g., stdout, files). The Handle method returns an error if processing fails,
// allowing the logger to handle output failures gracefully.
// Example (simplified handler implementation):
//
//	type MyHandler struct{}
//	func (h *MyHandler) Handle(e *Entry) error {
//	    fmt.Printf("[%s] %s: %s\n", e.Namespace, e.Level.String(), e.Message)
//	    return nil
//	}
type Handler interface {
	Handle(e *Entry) error // Processes a log entry, returning any error
}

// Outputter defines the interface for handlers that support dynamic output
// destination changes. Implementations can switch their output writer at runtime.
//
// Example usage:
//
//	h := &JSONHandler{}
//	h.Output(os.Stderr) // Switch to stderr
//	h.Output(file)      // Switch to file
type Outputter interface {
	Output(w io.Writer)
}

// HandlerOutputter combines the Handler and Outputter interfaces.
// Types implementing this interface can both process log entries and
// dynamically change their output destination at runtime.
//
// This is useful for creating flexible logging handlers that support
// features like log rotation, output redirection, or runtime configuration.
//
// Example usage:
//
//	var ho HandlerOutputter = &TextHandler{}
//	// Handle log entries
//	ho.Handle(&Entry{...})
//	// Switch output destination
//	ho.Output(os.Stderr)
//
// Common implementations include TextHandler and JSONHandler when they
// support output destination changes.
type HandlerOutputter interface {
	Handler   // Can process log entries
	Outputter // Can change output destination (has Output(w io.Writer) method)
}

// Timestamper defines an interface for handlers that support timestamp configuration.
// It includes a method to enable or disable timestamp logging and optionally set the timestamp format.
type Timestamper interface {
	// Timestamped enables or disables timestamp logging and allows specifying an optional format.
	// Parameters:
	//   enable: Boolean to enable or disable timestamp logging
	//   format: Optional string(s) to specify the timestamp format
	Timestamped(enable bool, format ...string)
}

// Wrap is a handler decorator function that transforms a log handler.
// It takes an existing handler as input and returns a new, wrapped handler
// that adds functionality (like filtering, transformation, or routing).
type Wrap func(next Handler) Handler
