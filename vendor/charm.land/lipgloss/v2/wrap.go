package lipgloss

import (
	"bytes"
	"io"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Wrap wraps the given string to the given width, preserving ANSI styles and links.
func Wrap(s string, width int, breakpoints string) string {
	var buf bytes.Buffer
	s = ansi.Wrap(s, width, breakpoints)
	w := NewWrapWriter(&buf)
	defer w.Close() //nolint:errcheck
	_, _ = io.WriteString(w, s)
	return buf.String()
}

// WrapWriter is a writer that writes to a buffer and keeps track of the
// current pen style and link state for the purpose of wrapping with newlines.
//
// When it encounters a newline, it resets the style and link, writes the
// newline, and then reapplies the style and link to the next line.
type WrapWriter struct {
	w     io.Writer
	p     *ansi.Parser
	style uv.Style
	link  uv.Link
}

// NewWrapWriter returns a new [WrapWriter].
func NewWrapWriter(w io.Writer) *WrapWriter {
	pw := &WrapWriter{w: w}
	pw.p = ansi.GetParser()
	handleCsi := func(cmd ansi.Cmd, params ansi.Params) {
		if cmd == 'm' {
			uv.ReadStyle(params, &pw.style)
		}
	}
	handleOsc := func(cmd int, data []byte) {
		if cmd == 8 {
			uv.ReadLink(data, &pw.link)
		}
	}
	pw.p.SetHandler(ansi.Handler{
		HandleCsi: handleCsi,
		HandleOsc: handleOsc,
	})
	return pw
}

// Style returns the current pen style.
func (w *WrapWriter) Style() uv.Style {
	return w.style
}

// Link returns the current pen link.
func (w *WrapWriter) Link() uv.Link {
	return w.link
}

// Write writes to the buffer.
func (w *WrapWriter) Write(p []byte) (int, error) {
	for i := range p {
		b := p[i]
		w.p.Advance(b)
		if b == '\n' {
			if !w.style.IsZero() {
				_, _ = w.w.Write([]byte(ansi.ResetStyle))
			}
			if !w.link.IsZero() {
				_, _ = w.w.Write([]byte(ansi.ResetHyperlink()))
			}
		}

		_, _ = w.w.Write([]byte{b})
		if b == '\n' {
			if !w.link.IsZero() {
				_, _ = w.w.Write([]byte(ansi.SetHyperlink(w.link.URL, w.link.Params)))
			}
			if !w.style.IsZero() {
				_, _ = w.w.Write([]byte(w.style.String()))
			}
		}
	}

	return len(p), nil
}

// Close closes the writer, resets the style and link if necessary, and releases
// its parser. Calling it is performance critical, but forgetting it does not
// cause safety issues or leaks.
func (w *WrapWriter) Close() error {
	if !w.style.IsZero() {
		_, _ = w.w.Write([]byte(ansi.ResetStyle))
	}
	if !w.link.IsZero() {
		_, _ = w.w.Write([]byte(ansi.ResetHyperlink()))
	}
	if w.p != nil {
		ansi.PutParser(w.p)
		w.p = nil
	}
	return nil
}
