package colorprofile

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"strconv"

	"github.com/charmbracelet/x/ansi"
)

// NewWriter creates a new color profile writer that downgrades color sequences
// based on the detected color profile.
//
// If environ is nil, it will use os.Environ() to get the environment variables.
//
// It queries the given writer to determine if it supports ANSI escape codes.
// If it does, along with the given environment variables, it will determine
// the appropriate color profile to use for color formatting.
//
// This respects the NO_COLOR, CLICOLOR, and CLICOLOR_FORCE environment variables.
func NewWriter(w io.Writer, environ []string) *Writer {
	return &Writer{
		Forward: w,
		Profile: Detect(w, environ),
	}
}

// Writer represents a color profile writer that writes ANSI sequences to the
// underlying writer.
type Writer struct {
	Forward io.Writer
	Profile Profile
}

// Write writes the given text to the underlying writer.
func (w *Writer) Write(p []byte) (int, error) {
	switch {
	case w.Profile == TrueColor:
		return w.Forward.Write(p) //nolint:wrapcheck
	case w.Profile <= NoTTY:
		_, err := io.WriteString(w.Forward, ansi.Strip(string(p)))
		return len(p), err
	case w.Profile == ASCII, w.Profile == ANSI, w.Profile == ANSI256:
		_, err := w.downsample(p)
		return len(p), err
	default:
		return 0, fmt.Errorf("invalid profile: %v", w.Profile)
	}
}

// downsample downgrades the given text to the appropriate color profile.
func (w *Writer) downsample(p []byte) (int, error) {
	var buf bytes.Buffer
	var state byte

	parser := ansi.GetParser()
	defer ansi.PutParser(parser)

	for len(p) > 0 {
		parser.Reset()
		seq, _, read, newState := ansi.DecodeSequence(p, state, parser)

		switch {
		case ansi.HasCsiPrefix(seq) && parser.Command() == 'm':
			handleSgr(w, parser, &buf)
		default:
			// If we're not a style SGR sequence, just write the bytes.
			if n, err := buf.Write(seq); err != nil {
				return n, err //nolint:wrapcheck
			}
		}

		p = p[read:]
		state = newState
	}

	return w.Forward.Write(buf.Bytes()) //nolint:wrapcheck
}

// WriteString writes the given text to the underlying writer.
func (w *Writer) WriteString(s string) (n int, err error) {
	return w.Write([]byte(s))
}

func handleSgr(w *Writer, p *ansi.Parser, buf *bytes.Buffer) {
	var style ansi.Style
	params := p.Params()
	for i := 0; i < len(params); i++ {
		param := params[i]

		switch param := param.Param(0); param {
		case 0:
			// SGR default parameter is 0. We use an empty string to reduce the
			// number of bytes written to the buffer.
			style = append(style, "")
		case 30, 31, 32, 33, 34, 35, 36, 37: // 8-bit foreground color
			if w.Profile < ANSI {
				continue
			}
			style = style.ForegroundColor(
				w.Profile.Convert(ansi.BasicColor(param - 30))) //nolint:gosec
		case 38: // 16 or 24-bit foreground color
			var c color.Color
			if n := ansi.ReadStyleColor(params[i:], &c); n > 0 {
				i += n - 1
			}
			if w.Profile < ANSI {
				continue
			}
			style = style.ForegroundColor(w.Profile.Convert(c))
		case 39: // default foreground color
			if w.Profile < ANSI {
				continue
			}
			style = style.ForegroundColor(nil)
		case 40, 41, 42, 43, 44, 45, 46, 47: // 8-bit background color
			if w.Profile < ANSI {
				continue
			}
			style = style.BackgroundColor(
				w.Profile.Convert(ansi.BasicColor(param - 40))) //nolint:gosec
		case 48: // 16 or 24-bit background color
			var c color.Color
			if n := ansi.ReadStyleColor(params[i:], &c); n > 0 {
				i += n - 1
			}
			if w.Profile < ANSI {
				continue
			}
			style = style.BackgroundColor(w.Profile.Convert(c))
		case 49: // default background color
			if w.Profile < ANSI {
				continue
			}
			style = style.BackgroundColor(nil)
		case 58: // 16 or 24-bit underline color
			var c color.Color
			if n := ansi.ReadStyleColor(params[i:], &c); n > 0 {
				i += n - 1
			}
			if w.Profile < ANSI {
				continue
			}
			style = style.UnderlineColor(w.Profile.Convert(c))
		case 59: // default underline color
			if w.Profile < ANSI {
				continue
			}
			style = style.UnderlineColor(nil)
		case 90, 91, 92, 93, 94, 95, 96, 97: // 8-bit bright foreground color
			if w.Profile < ANSI {
				continue
			}
			style = style.ForegroundColor(
				w.Profile.Convert(ansi.BasicColor(param - 90 + 8))) //nolint:gosec
		case 100, 101, 102, 103, 104, 105, 106, 107: // 8-bit bright background color
			if w.Profile < ANSI {
				continue
			}
			style = style.BackgroundColor(
				w.Profile.Convert(ansi.BasicColor(param - 100 + 8))) //nolint:gosec
		default:
			// If this is not a color attribute, just append it to the style.
			style = append(style, strconv.Itoa(param))
		}
	}

	_, _ = buf.WriteString(style.String())
}
