//go:build !windows
// +build !windows

package dotwriter

import (
	"bytes"
	"fmt"
)

// hide cursor
var hide = fmt.Sprintf("%c[?25l", ESC)

// show cursor
var show = fmt.Sprintf("%c[?25h", ESC)

// Flush the buffer, writing all buffered lines to out
func (w *Writer) Flush() error {
	if w.buf.Len() == 0 {
		return nil
	}
	// Hide cursor during write to avoid it moving around the screen
	defer w.hideCursor()()

	// Move up to the top of our last output.
	w.up(w.lineCount)
	lines := bytes.Split(w.buf.Bytes(), []byte{'\n'})
	w.lineCount = len(lines) - 1 // Record how many lines we will write for the next Flush()
	for i, line := range lines {
		// For each line, write the contents and clear everything else on the line
		_, err := w.out.Write(line)
		if err != nil {
			return err
		}
		w.clearRest()
		// Add a newline if this isn't the last line
		if i != len(lines)-1 {
			_, err := w.out.Write([]byte{'\n'})
			if err != nil {
				return err
			}
		}
	}
	w.buf.Reset()
	return nil
}

func (w *Writer) up(count int) {
	if count == 0 {
		return
	}
	_, _ = fmt.Fprintf(w.out, "%c[%dA", ESC, count)
}

func (w *Writer) clearRest() {
	_, _ = fmt.Fprintf(w.out, "%c[0K", ESC)
}

// hideCursor hides the cursor and returns a function to restore the cursor back.
func (w *Writer) hideCursor() func() {
	_, _ = fmt.Fprint(w.out, hide)
	return func() {
		_, _ = fmt.Fprint(w.out, show)
	}
}
