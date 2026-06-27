//go:build !go1.26

package pq

import (
	"errors"
	"slices"
)

// As asserts that the given error is [pq.Error] and returns it, returning nil
// if it's not a pq.Error.
//
// It will return nil if the pq.Error is not one of the given error codes. If no
// codes are given it will always return the Error.
//
// This is safe to call with a nil error.
func As(err error, codes ...ErrorCode) *Error {
	if err == nil { // Not strictly needed, but prevents alloc for nil errors.
		return nil
	}
	pqErr := new(Error)
	if errors.As(err, &pqErr) && (len(codes) == 0 || slices.Contains(codes, pqErr.Code)) {
		return pqErr
	}
	return nil
}
