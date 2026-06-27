//go:build !windows
// +build !windows

package uv

import (
	"io"

	"github.com/muesli/cancelreader"
)

// NewCancelReader creates a new [cancelreader.CancelReader] that provides a
// cancelable reader interface that can be used to cancel reads.
func NewCancelReader(r io.Reader) (cancelreader.CancelReader, error) {
	return cancelreader.NewReader(r) //nolint:wrapcheck
}
