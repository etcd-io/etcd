//go:build solaris
// +build solaris

package cancelreader

import (
	"io"
)

// NewReader returns a reader and a cancel function. If the input reader is a
// File, the cancel function can be used to interrupt a blocking read call.
// In this case, the cancel function returns true if the call was canceled
// successfully. If the input reader is not a File or the file descriptor
// is 1024 or larger, the cancel function does nothing and always returns false.
// The generic unix implementation is based on the posix select syscall.
func NewReader(reader io.Reader) (CancelReader, error) {
	return newSelectCancelReader(reader)
}
