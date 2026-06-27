//go:build !darwin && !windows && !linux && !solaris && !freebsd && !netbsd && !openbsd && !dragonfly
// +build !darwin,!windows,!linux,!solaris,!freebsd,!netbsd,!openbsd,!dragonfly

package cancelreader

import "io"

// NewReader returns a fallbackCancelReader that satisfies the CancelReader but
// does not actually support cancellation.
func NewReader(reader io.Reader) (CancelReader, error) {
	return newFallbackCancelReader(reader)
}
