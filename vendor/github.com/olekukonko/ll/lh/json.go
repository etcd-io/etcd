package lh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/ll/lx"
)

var jsonBufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// JSONHandler is a handler that outputs log entries as JSON objects.
// It formats log entries with timestamp, level, message, namespace, fields, and optional
// stack traces or dump segments, writing the result to the provided writer.
// Thread-safe with a mutex to protect concurrent writes.
type JSONHandler struct {
	writer  io.Writer // Destination for JSON output
	timeFmt string    // Format for timestamp (default: RFC3339Nano)
	pretty  bool      // Enable pretty printing with indentation if true
	//fieldMap map[string]string // Optional mapping for field names (not used in provided code)
	mu sync.Mutex // Protects concurrent access to writer
}

// JsonOutput represents the JSON structure for a log entry.
// It includes all relevant log data, such as timestamp, level, message, and optional
// stack trace or dump segments, serialized as a JSON object.
type JsonOutput struct {
	Time      string                 `json:"ts"`     // Timestamp in specified format
	Level     string                 `json:"lvl"`    // Log level (e.g., "INFO")
	Class     string                 `json:"class"`  // Entry class (e.g., "Text", "Dump")
	Msg       string                 `json:"msg"`    // Log message
	Namespace string                 `json:"ns"`     // Namespace path
	Stack     []byte                 `json:"stack"`  // Stack trace (if present)
	Dump      []dumpSegment          `json:"dump"`   // Hex/ASCII dump segments (for ClassDump)
	Fields    map[string]interface{} `json:"fields"` // Custom fields
}

// dumpSegment represents a single segment of a hex/ASCII dump.
// Used for ClassDump entries to structure position, hex values, and ASCII representation.
type dumpSegment struct {
	Offset int      `json:"offset"` // Starting byte offset of the segment
	Hex    []string `json:"hex"`    // Hexadecimal values of bytes
	ASCII  string   `json:"ascii"`  // ASCII representation of bytes
}

// NewJSONHandler creates a new JSONHandler writing to the specified writer.
// It initializes the handler with a default timestamp format (RFC3339Nano) and optional
// configuration functions to customize settings like pretty printing.
// Example:
//
//	handler := NewJSONHandler(os.Stdout)
//	logger := ll.New("app").Enable().Handler(handler)
//	logger.Info("Test") // Output: {"ts":"...","lvl":"INFO","class":"Text","msg":"Test","ns":"app","stack":null,"dump":null,"fields":null}
func NewJSONHandler(w io.Writer, opts ...func(*JSONHandler)) *JSONHandler {
	h := &JSONHandler{
		writer:  w,                // Set output writer
		timeFmt: time.RFC3339Nano, // Default timestamp format
	}
	// Apply configuration options
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Handle processes a log entry and writes it as JSON.
// It delegates to specialized methods based on the entry's class (Dump or regular),
// ensuring thread-safety with a mutex.
// Returns an error if JSON encoding or writing fails.
// Example:
//
//	handler.Handle(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Writes JSON object
func (h *JSONHandler) Handle(e *lx.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Handle dump entries separately
	if e.Class == lx.ClassDump {
		return h.handleDump(e)
	}
	// Handle standard log entries
	return h.handleRegular(e)
}

// Output sets the Writer destination for JSONHandler's output, ensuring thread safety with a mutex lock.
func (h *JSONHandler) Output(w io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.writer = w
}

// handleRegular handles standard log entries (non-dump).
// It converts the entry to a JsonOutput struct and encodes it as JSON,
// applying pretty printing if enabled. Logs encoding errors to stderr for debugging.
// Returns an error if encoding or writing fails.
// Example (internal usage):
//
//	h.handleRegular(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Writes JSON object
func (h *JSONHandler) handleRegular(e *lx.Entry) error {
	// Convert ordered fields to map for JSON output
	fieldsMap := make(map[string]interface{}, len(e.Fields))
	for _, pair := range e.Fields {
		fieldsMap[pair.Key] = pair.Value
	}

	// Create JSON output structure
	entry := JsonOutput{
		Time:      e.Timestamp.Format(h.timeFmt), // Format timestamp
		Level:     e.Level.String(),              // Convert level to string
		Class:     e.Class.String(),              // Convert class to string
		Msg:       e.Message,                     // Set message
		Namespace: e.Namespace,                   // Set namespace
		Dump:      nil,                           // No dump for regular entries
		Fields:    fieldsMap,                     // Copy fields as map
		Stack:     e.Stack,                       // Include stack trace if present
	}

	// Acquire buffer from pool to avoid allocation and reduce syscalls
	buf := jsonBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonBufPool.Put(buf)

	// Create JSON encoder writing to buffer
	enc := json.NewEncoder(buf)
	if h.pretty {
		// Enable indentation for pretty printing
		enc.SetIndent("", "  ")
	}

	// Encode JSON to buffer
	err := enc.Encode(entry)
	if err != nil {
		// Log encoding error for debugging
		fmt.Fprintf(os.Stderr, "JSON encode error: %v\n", err)
		return err
	}

	// Write buffer to underlying writer in one go
	_, err = h.writer.Write(buf.Bytes())
	return err
}

// handleDump processes ClassDump entries, converting hex dump output to JSON segments.
// It parses the dump message into structured segments with offset, hex, and ASCII data,
// encoding them as a JsonOutput struct.
// Returns an error if parsing or encoding fails.
// Example (internal usage):
//
//	h.handleDump(&lx.Entry{Class: lx.ClassDump, Message: "pos 00 hex: 61 62 'ab'"}) // Writes JSON with dump segments
func (h *JSONHandler) handleDump(e *lx.Entry) error {
	var segments []dumpSegment
	lines := strings.Split(e.Message, "\n")

	// Parse each line of the dump message
	for _, line := range lines {
		if !strings.HasPrefix(line, "pos") {
			continue // Skip non-dump lines
		}
		parts := strings.SplitN(line, "hex:", 2)
		if len(parts) != 2 {
			continue // Skip invalid lines
		}
		// Parse position
		var offset int
		fmt.Sscanf(parts[0], "pos %d", &offset)

		// Parse hex and ASCII
		hexAscii := strings.SplitN(parts[1], "'", 2)
		hexStr := strings.Fields(strings.TrimSpace(hexAscii[0]))

		// Create dump segment
		segments = append(segments, dumpSegment{
			Offset: offset,                         // Set byte offset
			Hex:    hexStr,                         // Set hex values
			ASCII:  strings.Trim(hexAscii[1], "'"), // Set ASCII representation
		})
	}

	// Convert ordered fields to map for JSON output
	fieldsMap := make(map[string]interface{}, len(e.Fields))
	for _, pair := range e.Fields {
		fieldsMap[pair.Key] = pair.Value
	}

	// Acquire buffer from pool
	buf := jsonBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonBufPool.Put(buf)

	// Encode JSON output with dump segments to buffer
	enc := json.NewEncoder(buf)
	if h.pretty {
		enc.SetIndent("", "  ")
	}

	err := enc.Encode(JsonOutput{
		Time:      e.Timestamp.Format(h.timeFmt), // Format timestamp
		Level:     e.Level.String(),              // Convert level to string
		Class:     e.Class.String(),              // Convert class to string
		Msg:       "dumping segments",            // Fixed message for dumps
		Namespace: e.Namespace,                   // Set namespace
		Dump:      segments,                      // Include parsed segments
		Fields:    fieldsMap,                     // Copy fields as map
		Stack:     e.Stack,                       // Include stack trace if present
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON dump encode error: %v\n", err)
		return err
	}

	// Write buffer to underlying writer
	_, err = h.writer.Write(buf.Bytes())
	return err
}
