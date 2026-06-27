package ansi

import "io"

// Execute is a function that "execute" the given escape sequence by writing it
// to the provided output writter.
//
// This is a syntactic sugar over [io.WriteString].
func Execute(w io.Writer, s string) (int, error) {
	return io.WriteString(w, s) //nolint:wrapcheck
}
