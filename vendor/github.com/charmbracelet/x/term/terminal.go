package term

import (
	"io"
)

// File represents a file that has a file descriptor and can be read from,
// written to, and closed.
type File interface {
	io.ReadWriteCloser
	Fd() uintptr
}
