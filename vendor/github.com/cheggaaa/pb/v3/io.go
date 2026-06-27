package pb

import (
	"io"
)

// Reader it's a wrapper for given reader, but with progress handle
type Reader struct {
	io.Reader
	bar *ProgressBar
}

// Read reads bytes from wrapped reader and add amount of bytes to progress bar
func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.bar.Add(n)
	return
}

// Close the wrapped reader when it implements io.Closer
func (r *Reader) Close() (err error) {
	r.bar.Finish()
	if closer, ok := r.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return
}

// Writer it's a wrapper for given writer, but with progress handle
type Writer struct {
	io.Writer
	bar *ProgressBar
}

// Write writes bytes to wrapped writer and add amount of bytes to progress bar
func (r *Writer) Write(p []byte) (n int, err error) {
	n, err = r.Writer.Write(p)
	r.bar.Add(n)
	return
}

// Close the wrapped reader when it implements io.Closer
func (r *Writer) Close() (err error) {
	r.bar.Finish()
	if closer, ok := r.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return
}
