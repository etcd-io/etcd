//go:build go1.26

package pq

import (
	"errors"
	"github.com/lib/pq/pqerror"
	"slices"
)

// As asserts that the given error is [pq.Error] and returns it, returning nil
// if it's not a pq.Error.
//
// It will return nil if the pq.Error is not one of the given error codes. If no
// codes are given it will always return the Error.
//
// This is safe to call with a nil error.
func As(err error, codes ...pqerror.Code) *Error {
	if pqErr, ok := errors.AsType[*Error](err); ok && (len(codes) == 0 || slices.Contains(codes, pqErr.Code)) {
		return pqErr
	}
	return nil
}
