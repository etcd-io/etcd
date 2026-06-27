// Package fields provides types and functions for working with key-value fields.
package fields

import (
	"fmt"
	"iter"
	"strings"
)

// Field represents a key-value pair, where the key is a string and the value can be any type.
type Field struct {
	// Key of the field
	K string
	// Value of the field
	V any
}

// F creates a new Field with the given key and value.
//
// Example:
//
//	f := fields.F("user", "alice")
func F(key string, value any) Field {
	return Field{K: key, V: value}
}

// writeKVTo writes a key-value pair to the given builder in the format "key=value".
func writeKVTo(b *strings.Builder, key string, value any) {
	b.WriteString(key)
	b.WriteRune('=')

	switch val := value.(type) {
	case string:
		b.WriteString(val)

	case fmt.Stringer:
		b.WriteString(val.String())

	case error:
		b.WriteString(val.Error())

	default:
		_, _ = fmt.Fprintf(b, "%v", value)
	}
}

// WriteTo writes the Field as a string in the format "key=value" to the provided builder.
func (f Field) WriteTo(b *strings.Builder) {
	writeKVTo(b, f.K, f.V)
}

// String returns the Field as a string in the format "key=value".
func (f Field) String() string {
	b := &strings.Builder{}
	f.WriteTo(b)

	return b.String()
}

// WriteTo writes key-value pairs from an iter.Seq2[string, any] to the builder in the format "(key1=val1, key2=val2)".
// If no fields are present, nothing is written.
func WriteTo(b *strings.Builder, seq iter.Seq2[string, any]) {
	first := true

	for k, v := range seq {
		if first {
			first = false

			b.WriteString("(")
		} else {
			b.WriteString(", ")
		}

		writeKVTo(b, k, v)
	}

	// means that we've written at least one field, and, therefore,
	// can close the parenthesis
	if !first {
		b.WriteString(")")
	}
}
