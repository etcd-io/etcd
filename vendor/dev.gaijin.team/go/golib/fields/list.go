package fields

import (
	"iter"
	"strings"
)

// List is an ordered collection of Field values, preserving insertion order.
// Such collection do not check for duplicate keys.
type List []Field //nolint:recvcheck //we need Add to be a pointer receiver to modify original value.

// Add one or more fields to the List, modifying it.
//
// Example:
//
//	var l fields.List
//	l.Add(fields.F("foo", "bar"), fields.F("baz", 42))
func (l *List) Add(fields ...Field) {
	*l = append(*l, fields...)
}

// ToDict converts the List to a Dict, overwriting duplicate keys with the last occurrence.
//
// Example:
//
//	l := fields.List{fields.F("foo", 1), fields.F("foo", 2)}
//	d := l.ToDict() // d["foo"] == 2
func (l List) ToDict() Dict {
	d := make(Dict, len(l))

	for i := range l {
		d[l[i].K] = l[i].V
	}

	return d
}

// All returns an iterator over all key-value pairs in the List as iter.Seq2[string, any].
//
// Example:
//
//	for k, v := range l.All() {
//	    fmt.Println(k, v)
//	}
func (l List) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for i := 0; i < len(l); i++ {
			if !yield(l[i].K, l[i].V) {
				return
			}
		}
	}
}

// WriteTo writes the List as a string in the format "(key1=val1, key2=val2)" to the provided builder.
// If the List is empty, nothing is written.
func (l List) WriteTo(b *strings.Builder) {
	WriteTo(b, l.All())
}

// String returns the List as a string in the format "(key1=val1, key2=val2)".
// Returns an empty string if the List is empty.
func (l List) String() string {
	b := strings.Builder{}

	l.WriteTo(&b)

	return b.String()
}
