package fields

import (
	"iter"
	"strings"
)

const CollectionSep = ", "

// Dict is a map-based collection of unique fields, keyed by string.
// It provides efficient lookup and overwrites duplicate keys.
type Dict map[string]any

// Add inserts or updates fields in the Dict, overwriting existing keys if present.
//
// Example:
//
//	d := fields.Dict{"foo": "bar"}
//	d.Add(fields.F("baz", 42), fields.F("foo", "qux")) // d["foo"] == "qux"
func (d Dict) Add(fields ...Field) {
	for _, f := range fields {
		d[f.K] = f.V
	}
}

// ToList converts the Dict to a List, with order unspecified.
// Each key-value pair becomes a Field in the resulting List.
func (d Dict) ToList() List {
	s := make(List, 0, len(d))

	for k, v := range d {
		s = append(s, Field{k, v})
	}

	return s
}

// All returns an iterator over all key-value pairs in the Dict as iter.Seq2[string, any].
//
// Example:
//
//	for k, v := range d.All() {
//	    fmt.Println(k, v)
//	}
func (d Dict) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for k, v := range d {
			if !yield(k, v) {
				return
			}
		}
	}
}

// WriteTo writes the Dict as a string in the format "(key1=val1, key2=val2)" to the provided builder.
// If the Dict is empty, nothing is written. The order of fields is unspecified.
func (d Dict) WriteTo(b *strings.Builder) {
	WriteTo(b, d.All())
}

// String returns the Dict as a string in the format "(key1=val1, key2=val2)".
// Returns an empty string if the Dict is empty. The order of fields is unspecified.
func (d Dict) String() string {
	b := strings.Builder{}

	d.WriteTo(&b)

	return b.String()
}
