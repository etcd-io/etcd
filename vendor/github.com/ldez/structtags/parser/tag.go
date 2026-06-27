// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"fmt"
	"strconv"
)

// Filler is the interface implemented by types that can fill elements from a struct tag.
type Filler[T any] interface {
	// Data returns the [T] filled by the struct tag content.
	Data() T

	// Fill fills the data from a struct tag.
	Fill(key, value string) error
}

// Tag parses a struct tag.
//
// Based on https://github.com/golang/go/blob/411c250d64304033181c46413a6e9381e8fe9b82/src/reflect/type.go#L1030-L1108
//
//nolint:gocyclo
func Tag[T any](tag string, filler Filler[T]) (T, error) {
	base := tag

	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}

		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}

		switch {
		case i == 0:
			var zero T

			return zero, fmt.Errorf("invalid struct tag syntax `%s`", base)

		case i+1 > len(tag):
			var zero T

			return zero, fmt.Errorf("invalid struct tag syntax `%s`: missing `:`", base)

		case i+1 == len(tag):
			var zero T

			return zero, fmt.Errorf("invalid struct tag value `%s`", base)

		case tag[i] != ':':
			var zero T

			return zero, fmt.Errorf("invalid struct tag syntax `%s`: missing `:`", base)

		case tag[i+1] != '"':
			var zero T

			return zero, fmt.Errorf("invalid struct tag value `%s`: missing opening quote", base)
		}

		name := tag[:i]
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}

			i++
		}

		if i >= len(tag) {
			var zero T

			return zero, fmt.Errorf("invalid struct tag value `%s`: missing closing quote", base)
		}

		qvalue := tag[:i+1]
		tag = tag[i+1:]

		value, err := strconv.Unquote(qvalue)
		if err != nil {
			var zero T

			return zero, fmt.Errorf("invalid struct tag value `%s`: %w", base, err)
		}

		err = filler.Fill(name, value)
		if err != nil {
			var zero T

			return zero, err
		}
	}

	return filler.Data(), nil
}
