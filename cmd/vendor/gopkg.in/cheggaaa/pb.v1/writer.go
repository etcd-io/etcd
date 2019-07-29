package pb

import (
	"io"
)

// It's proxy Writer, implement io.Writer
type Writer struct {
	io.Writer
	bar *ProgressBar
}

func (r *Writer) Write(p []byte) (n int, err error) {
	n, err = r.Writer.Write(p)
	r.bar.Add(n)
	return
}

// Close the reader when it implements io.Closer
func (r *Writer) Close() (err error) {
	if closer, ok := r.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return
}
