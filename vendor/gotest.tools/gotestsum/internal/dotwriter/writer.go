/*
Package dotwriter implements a buffered Writer for updating progress on the
terminal.
*/
package dotwriter

import (
	"bytes"
	"io"
)

// ESC is the ASCII code for escape character
const ESC = 27

// Writer buffers writes until Flush is called. Flush clears previously written
// lines before writing new lines from the buffer.
// The main logic is platform specific, see the related files.
type Writer struct {
	out       io.Writer
	buf       bytes.Buffer
	lineCount int
}

// New returns a new Writer
func New(out io.Writer) *Writer {
	return &Writer{out: out}
}

// Write saves buf to a buffer
func (w *Writer) Write(buf []byte) (int, error) {
	return w.buf.Write(buf)
}
