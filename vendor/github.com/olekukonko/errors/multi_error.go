package errors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
)

// MultiError represents a thread-safe collection of errors with enhanced features.
// Supports limits, sampling, and custom formatting for error aggregation.
type MultiError struct {
	errors []error
	mu     sync.RWMutex

	// Configuration fields
	limit      int            // Maximum number of errors to store (0 = unlimited)
	formatter  ErrorFormatter // Custom formatting function for error string
	sampling   bool           // Whether sampling is enabled to limit error collection
	sampleRate uint32         // Sampling percentage (1-100) when sampling is enabled
	rand       *rand.Rand     // Random source for sampling (nil defaults to fastRand)
}

// ErrorFormatter defines a function for custom error message formatting.
// Takes a slice of errors and returns a single formatted string.
type ErrorFormatter func([]error) string

// MultiErrorOption configures MultiError behavior during creation.
type MultiErrorOption func(*MultiError)

// NewMultiError creates a new MultiError instance with optional configuration.
// Initial capacity is set to 4; applies options in the order provided.
func NewMultiError(opts ...MultiErrorOption) *MultiError {
	m := &MultiError{
		errors: make([]error, 0, 4),
		limit:  0, // Unlimited by default
	}

	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Add appends an error to the collection with optional sampling, limit checks, and duplicate prevention.
// Ignores nil errors and duplicates based on string equality; thread-safe.
func (m *MultiError) Add(errs ...error) {
	if len(errs) == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, err := range errs {
		if err == nil {
			continue
		}

		// Check for duplicates by comparing error messages
		duplicate := false
		for _, e := range m.errors {
			if e.Error() == err.Error() {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		// Apply sampling if enabled and collection isn’t empty
		if m.sampling && len(m.errors) > 0 {
			var r uint32
			if m.rand != nil {
				r = uint32(m.rand.Int31n(100))
			} else {
				r = fastRand() % 100
			}
			if r > m.sampleRate { // Accept if random value is within sample rate
				continue
			}
		}

		// Respect limit if set
		if m.limit > 0 && len(m.errors) >= m.limit {
			continue
		}

		m.errors = append(m.errors, err)
	}
}

// Addf formats and adds a new error to the collection.
func (m *MultiError) Addf(format string, args ...interface{}) {
	m.Add(Newf(format, args...))
}

// Clear removes all errors from the collection.
// Thread-safe; resets the slice while preserving capacity.
func (m *MultiError) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = m.errors[:0]
}

// Count returns the number of errors in the collection.
// Thread-safe.
func (m *MultiError) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors)
}

// Error returns a formatted string representation of the errors.
// Returns empty string if no errors, single error message if one exists,
// or a formatted list using custom formatter or default if multiple; thread-safe.
func (m *MultiError) Error() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch len(m.errors) {
	case 0:
		return ""
	case 1:
		return m.errors[0].Error()
	default:
		if m.formatter != nil {
			return m.formatter(m.errors)
		}
		return defaultFormat(m.errors)
	}
}

// Errors returns a copy of the contained errors.
// Thread-safe; returns nil if no errors exist.
func (m *MultiError) Errors() []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.errors) == 0 {
		return nil
	}
	errs := make([]error, len(m.errors))
	copy(errs, m.errors)
	return errs
}

// Filter returns a new MultiError containing only errors that match the predicate.
// Thread-safe; preserves original configuration including limit, formatter, and sampling.
func (m *MultiError) Filter(fn func(error) bool) *MultiError {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var opts []MultiErrorOption
	opts = append(opts, WithLimit(m.limit))
	if m.formatter != nil {
		opts = append(opts, WithFormatter(m.formatter))
	}
	if m.sampling {
		opts = append(opts, WithSampling(m.sampleRate))
	}

	filtered := NewMultiError(opts...)
	for _, err := range m.errors {
		if fn(err) {
			filtered.Add(err)
		}
	}
	return filtered
}

// First returns the first error in the collection, if any.
// Thread-safe; returns nil if the collection is empty.
func (m *MultiError) First() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.errors) > 0 {
		return m.errors[0]
	}
	return nil
}

// Has reports whether the collection contains any errors.
// Thread-safe.
func (m *MultiError) Has() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors) > 0
}

// Last returns the most recently added error in the collection, if any.
// Thread-safe; returns nil if the collection is empty.
func (m *MultiError) Last() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.errors) > 0 {
		return m.errors[len(m.errors)-1]
	}
	return nil
}

// Merge combines another MultiError's errors into this one.
// Thread-safe; respects this instance’s limit and sampling settings; no-op if other is nil or empty.
func (m *MultiError) Merge(other *MultiError) {
	if other == nil || !other.Has() {
		return
	}

	other.mu.RLock()
	defer other.mu.RUnlock()

	for _, err := range other.errors {
		m.Add(err)
	}
}

// IsNull checks if the MultiError is empty or contains only null errors.
// Returns true if empty or all errors are null (via IsNull() or empty message); thread-safe.
func (m *MultiError) IsNull() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Fast path for empty MultiError
	if len(m.errors) == 0 {
		return true
	}

	// Check each error for null status
	allNull := true
	for _, err := range m.errors {
		switch e := err.(type) {
		case interface{ IsNull() bool }:
			if !e.IsNull() {
				allNull = false
				break
			}
		case nil:
			continue
		default:
			if e.Error() != "" {
				allNull = false
				break
			}
		}
	}
	return allNull
}

// Single returns nil if the collection is empty, the single error if only one exists,
// or the MultiError itself if multiple errors are present.
// Thread-safe; useful for unwrapping to a single error when possible.
func (m *MultiError) Single() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch len(m.errors) {
	case 0:
		return nil
	case 1:
		return m.errors[0]
	default:
		return m
	}
}

// String implements the Stringer interface for a concise string representation.
// Thread-safe; delegates to Error() for formatting.
func (m *MultiError) String() string {
	return m.Error()
}

// Unwrap returns a copy of the contained errors for multi-error unwrapping.
// Implements the errors.Unwrap interface; thread-safe; returns nil if empty.
func (m *MultiError) Unwrap() []error {
	return m.Errors()
}

// WithFormatter sets a custom error formatting function.
// Returns a MultiErrorOption for use with NewMultiError; overrides default formatting.
func WithFormatter(f ErrorFormatter) MultiErrorOption {
	return func(m *MultiError) {
		m.formatter = f
	}
}

// WithLimit sets the maximum number of errors to store.
// Returns a MultiErrorOption for use with NewMultiError; 0 means unlimited, negative values are ignored.
func WithLimit(n int) MultiErrorOption {
	return func(m *MultiError) {
		if n < 0 {
			n = 0 // Ensure non-negative limit
		}
		m.limit = n
	}
}

// WithSampling enables error sampling with a specified rate (1-100).
// Returns a MultiErrorOption for use with NewMultiError; caps rate at 100 for validity.
func WithSampling(rate uint32) MultiErrorOption {
	return func(m *MultiError) {
		if rate > 100 {
			rate = 100
		}
		m.sampling = true
		m.sampleRate = rate
	}
}

// WithRand sets a custom random source for sampling, useful for testing.
// Returns a MultiErrorOption for use with NewMultiError; defaults to fastRand if nil.
func WithRand(r *rand.Rand) MultiErrorOption {
	return func(m *MultiError) {
		m.rand = r
	}
}

// MarshalJSON serializes the MultiError to JSON, including all contained errors and configuration metadata.
// Thread-safe; errors are serialized using their MarshalJSON method if available, otherwise as strings.
func (m *MultiError) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get buffer from pool for efficiency
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	defer jsonBufferPool.Put(buf)
	buf.Reset()

	// Create encoder
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	// Define JSON structure
	type jsonError struct {
		Error interface{} `json:"error"` // Holds either JSON-marshaled error or string
	}

	je := struct {
		Count      int         `json:"count"`                 // Number of errors
		Limit      int         `json:"limit,omitempty"`       // Maximum error limit (omitted if 0)
		Sampling   bool        `json:"sampling,omitempty"`    // Whether sampling is enabled
		SampleRate uint32      `json:"sample_rate,omitempty"` // Sampling rate (1-100, omitted if not sampling)
		Errors     []jsonError `json:"errors"`                // List of errors
	}{
		Count:      len(m.errors),
		Limit:      m.limit,
		Sampling:   m.sampling,
		SampleRate: m.sampleRate,
	}

	// Serialize each error
	je.Errors = make([]jsonError, len(m.errors))
	for i, err := range m.errors {
		if err == nil {
			je.Errors[i] = jsonError{Error: nil}
			continue
		}
		// Check if the error implements json.Marshaler
		if marshaler, ok := err.(json.Marshaler); ok {
			marshaled, err := marshaler.MarshalJSON()
			if err != nil {
				// Fallback to string if marshaling fails
				je.Errors[i] = jsonError{Error: err.Error()}
			} else {
				var raw json.RawMessage = marshaled
				je.Errors[i] = jsonError{Error: raw}
			}
		} else {
			// Use error string for non-marshaler errors
			je.Errors[i] = jsonError{Error: err.Error()}
		}
	}

	// Encode JSON
	if err := enc.Encode(je); err != nil {
		return nil, fmt.Errorf("failed to marshal MultiError: %v", err)
	}

	// Remove trailing newline
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// defaultFormat provides the default formatting for multiple errors.
// Returns a semicolon-separated list prefixed with the error count (e.g., "errors(3): err1; err2; err3").
func defaultFormat(errs []error) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("errors(%d): ", len(errs)))
	for i, err := range errs {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// fastRand generates a quick pseudo-random number for sampling.
// Uses a simple xorshift algorithm based on the current time; not cryptographically secure.
var fastRandState uint32 = 1 // Must be non-zero

func fastRand() uint32 {
	for {
		// Atomically load the current state
		old := atomic.LoadUint32(&fastRandState)
		// Xorshift computation
		x := old
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		// Attempt to store the new state atomically
		if atomic.CompareAndSwapUint32(&fastRandState, old, x) {
			return x
		}
		// Otherwise retry
	}
}
