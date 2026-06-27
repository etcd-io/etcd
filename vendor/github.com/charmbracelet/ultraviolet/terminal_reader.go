package uv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/cancelreader"
	"github.com/rivo/uniseg"
)

// ErrReaderNotStarted is returned when the reader has not been started yet.
var ErrReaderNotStarted = fmt.Errorf("reader not started")

// DefaultEscTimeout is the default timeout at which the [TerminalReader] will
// process ESC sequences. It is set to 50 milliseconds.
const DefaultEscTimeout = 50 * time.Millisecond

// TerminalReader represents an input event loop that reads input events from
// a reader and parses them into human-readable events. It supports
// reading escape sequences, mouse events, and bracketed paste mode.
type TerminalReader struct {
	EventDecoder

	// MouseMode determines whether mouse events are enabled or not. This is a
	// platform-specific feature and is only available on Windows. When this is
	// true, the reader will be initialized to read mouse events using the
	// Windows Console API.
	MouseMode *MouseMode

	// EscTimeout is the escape character timeout duration. Most escape
	// sequences start with an escape character [ansi.ESC] and are followed by
	// one or more characters. If the next character is not received within
	// this timeout, the reader will assume that the escape sequence is
	// complete and will process the received characters as a complete escape
	// sequence.
	//
	// By default, this is set to [DefaultEscTimeout] (50 milliseconds).
	EscTimeout time.Duration

	r     io.Reader
	table map[string]Key // table is a lookup table for key sequences.

	term string // term is the terminal name $TERM.

	// paste is the bracketed paste mode buffer.
	// When nil, bracketed paste mode is disabled.
	paste []byte

	lookup bool // lookup indicates whether to use the lookup table for key sequences.

	// vtInput indicates whether we're using Windows Console API VT input mode.
	//nolint:unused,nolintlint
	vtInput bool

	// We use these buffers to decode UTF-16 sequences and graphemes from the
	// Windows Console API and Win32-Input-Mode events.
	utf16Half   [2]bool    // 0 key up, 1 key down
	utf16Buf    [2][2]rune // 0 key up, 1 key down
	graphemeBuf [2][]rune  // 0 key up, 1 key down
	//nolint:unused,nolintlint
	lastMouseBtns uint32 // the last mouse button state for the previous event
	//nolint:unused,nolintlint
	lastWinsizeX, lastWinsizeY int16 // the last window size for the previous event to prevent multiple size events from firing

	logger Logger // The logger to use for debugging.
}

// NewTerminalReader returns a new input event reader. The reader streams input
// events from the terminal and parses escape sequences into human-readable
// events. It supports reading Terminfo databases.
//
// Use [TerminalReader.UseTerminfo] to use Terminfo defined key sequences.
// Use [TerminalReader.Legacy] to control legacy key encoding behavior.
//
// Example:
//
//	```go
//	var cr cancelreader.CancelReader
//	var evc chan Event
//	sc := NewTerminalReader(cr, os.Getenv("TERM"))
//	go sc.StreamEvents(ctx, evc)
//	```
func NewTerminalReader(r io.Reader, termType string) *TerminalReader {
	d := &TerminalReader{
		EscTimeout: DefaultEscTimeout,
		r:          r,
		term:       termType,
		lookup:     true, // Use lookup table by default.
	}
	d.r = r
	if d.table == nil {
		d.table = buildKeysTable(d.Legacy, d.term, d.UseTerminfo)
	}
	return d
}

// readBufSize is the size of the read buffer used to read input events at a time.
const readBufSize = 4096

// sendBytes reads data from the reader and sends it to the provided channel.
// It stops when an error occurs or when the context is closed.
func (d *TerminalReader) sendBytes(ctx context.Context, readc chan []byte) error {
	for {
		var readBuf [readBufSize]byte
		n, err := d.r.Read(readBuf[:])
		if err != nil {
			return err //nolint:wrapcheck
		}

		select {
		case <-ctx.Done():
			return nil
		case readc <- readBuf[:n]:
		}
	}
}

// StreamEvents sends events to the provided channel. It stops when the context
// is closed or when an error occurs.
func (d *TerminalReader) StreamEvents(ctx context.Context, eventc chan<- Event) error {
	var buf bytes.Buffer
	errc := make(chan error, 1)
	readc := make(chan []byte)
	timeout := time.NewTimer(d.EscTimeout)
	ttimeout := time.Now().Add(d.EscTimeout)

	go func() {
		if err := d.streamData(ctx, readc); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, cancelreader.ErrCanceled) {
				errc <- nil
				return
			}
			errc <- err
			return
		}
	}()

	for {
		select {
		case <-ctx.Done():
			d.sendEvents(buf.Bytes(), true, eventc)
			return nil
		case err := <-errc:
			d.sendEvents(buf.Bytes(), true, eventc)
			return err // return the first error encountered
		case <-timeout.C:
			d.logf("timeout reached")

			// Timeout reached process the buffer including any incomplete sequences.
			var n int // n is the number of bytes processed
			timedout := time.Now().After(ttimeout)
			if buf.Len() > 0 && timedout {
				d.logf("timeout expired, processing buffer")
				n = d.sendEvents(buf.Bytes(), true, eventc)
			}

			if n > 0 {
				buf.Next(n)
			}

			if buf.Len() > 0 {
				if !timeout.Stop() {
					// drain the channel if it was already running
					select {
					case <-timeout.C:
					default:
					}
				}

				d.logf("resetting timeout for remaining buffer")
				timeout.Reset(d.EscTimeout)
			}

		case read := <-readc:
			d.logf("input: %q", read)
			buf.Write(read)
			ttimeout = time.Now().Add(d.EscTimeout)
			n := d.sendEvents(buf.Bytes(), false, eventc)
			if !timeout.Stop() {
				// drain the channel if it was already running
				select {
				case <-timeout.C:
				default:
				}
			}

			if n > 0 {
				d.logf("processed %d bytes from buffer", n)
				buf.Next(n)
			}

			if buf.Len() > 0 {
				d.logf("resetting timeout for remaining buffer after parse")
				timeout.Reset(d.EscTimeout)
			}
		}
	}
}

// SetLogger sets the logger to use for debugging. If nil, no logging will be
// performed.
func (d *TerminalReader) SetLogger(logger Logger) {
	d.logger = logger
}

func (d *TerminalReader) sendEvents(buf []byte, expired bool, eventc chan<- Event) int {
	n, events := d.scanEvents(buf, expired)
	for _, event := range events {
		eventc <- event
	}
	return n
}

func (d *TerminalReader) scanEvents(buf []byte, expired bool) (total int, events []Event) {
	if len(buf) == 0 {
		return 0, nil
	}

	var dn int
	d.logf("processing buf %d %q", len(buf), buf)
	dn, buf = d.deserializeWin32Input(buf)
	total += dn

	// Lookup table first
	if d.lookup && len(buf) > 2 && buf[0] == ansi.ESC {
		if k, ok := d.table[string(buf)]; ok {
			return len(buf), []Event{KeyPressEvent(k)}
		}
	}

	// total is the total number of bytes processed
	for len(buf) > 0 {
		esc := buf[0] == ansi.ESC
		n, event := d.Decode(buf)

		// Handle bracketed-paste
		if d.paste != nil { //nolint:nestif
			if _, ok := event.(PasteEndEvent); !ok {
				switch event := event.(type) {
				case KeyPressEvent:
					if len(event.Text) > 0 {
						d.paste = append(d.paste, event.Text...)
					} else {
						seq := string(buf[:n])
						isWin32 := strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "_")
						switch {
						case isWin32 && event.Code == KeyEnter && event.Code == event.BaseCode:
							// This handles special cases where
							// win32-input-mode encodes newlines and other keys
							// as keypress events. We need to encode them as
							// their respective values.
							d.paste = append(d.paste, '\n')
						case isWin32 && unicode.IsControl(event.Code) && event.Code == event.BaseCode:
							// This handles other cases such as tabs, escapes, etc.
							d.paste = append(d.paste, string(event.Code)...)
						case !isWin32:
							// We ignore all other non-text win32-input-mode events.
							if esc && n <= 2 && !expired {
								// If the event is an escape sequence and we
								// are not expired, we need to wait for more
								// input.
								return total, events
							}
							d.paste = append(d.paste, seq...)
						}
					}
				case UnknownEvent:
					if !expired {
						// If the event is unknown and we are not expired, we
						// return need to try to decode the buffer again.
						return total, events
					}
				default:
					// Everything else is ignored...
				}
				buf = buf[n:]
				total += n
				continue
			}
		}

		var isUnknown bool
		switch event.(type) {
		case ignoredEvent:
			// ignore this event
			event = nil
		case UnknownEvent:
			isUnknown = true
			// Try to look up the event in the table.
			if !expired {
				return total, events
			}

			if k, ok := d.table[string(buf[:n])]; ok {
				events = append(events, KeyPressEvent(k))
				return total + n, events
			}

			events = append(events, event)
		case PasteStartEvent:
			d.paste = []byte{} // reset the paste buffer
		case PasteEndEvent:
			var paste []rune
			for len(d.paste) > 0 {
				r, w := utf8.DecodeRune(d.paste)
				if r != utf8.RuneError {
					paste = append(paste, r)
				}
				d.paste = d.paste[w:]
			}
			d.paste = nil // reset the paste buffer
			events = append(events, PasteEvent{string(paste)})
		}

		if !isUnknown && event != nil {
			if esc && n <= 2 && !expired {
				// Wait for more input
				return total, events
			}

			if m, ok := event.(MultiEvent); ok {
				// If the event is a MultiEvent, append all events to the queue.
				events = append(events, m...)
			} else {
				// Otherwise, just append the event to the queue.
				events = append(events, event)
			}
		}

		buf = buf[n:]
		total += n
	}

	return total, events
}

func (d *TerminalReader) encodeGraphemeBufs() []byte {
	var b []byte
	for kd := range d.graphemeBuf {
		if len(d.graphemeBuf[kd]) > 0 {
			switch kd {
			case 1:
				b = append(b, string(d.graphemeBuf[kd])...)
			case 0:
				// Encode the release grapheme as Kitty Keyboard to get the release event.
				grs := uniseg.NewGraphemes(string(d.graphemeBuf[kd]))
				for grs.Next() {
					var codepoints string
					gr := grs.Str()
					for i, r := range gr {
						if r == 0 {
							continue
						}
						if i > 0 {
							codepoints += ":"
						}
						codepoints += strconv.FormatInt(int64(r), 10)
					}
					// This is dark :)
					// During serializing/deserializing of win32 input events, the
					// API will split a grapheme into runes and send them as a
					// keydown/keyup sequences. We collect the runes, decoded them
					// from UTF-16 just fine. However, [EventDecoder.Decode] will
					// always decode graphemes as [KeyPressEvent]s. Thus, to
					// workaround that and the existing API, while we are
					// intercepting win32 input events, we encode the grapheme
					// release events as Kitty Keyboard sequences so that
					// [EventDecoder.Decode] can properly decode them as
					// [KeyReleaseEvent]s.
					seq := fmt.Sprintf("\x1b[%d;1:3;%su", d.graphemeBuf[kd][0], codepoints)
					b = append(b, seq...)
				}
			}
			d.graphemeBuf[kd] = d.graphemeBuf[kd][:0] // reset the buffer
		}
	}
	return b
}

func (d *TerminalReader) storeGraphemeRune(kd int, r rune) {
	if d.utf16Half[kd] {
		// We have a half pair that needs to be decoded.
		d.utf16Half[kd] = false
		d.utf16Buf[kd][1] = r
		r := utf16.DecodeRune(d.utf16Buf[kd][0], d.utf16Buf[kd][1])
		d.graphemeBuf[kd] = append(d.graphemeBuf[kd], r)
	} else if utf16.IsSurrogate(r) {
		// This is the first half of a UTF-16 surrogate pair.
		d.utf16Half[kd] = true
		d.utf16Buf[kd][0] = r
	} else {
		// This should be a single UTF-16 that can be converted
		// to UTF-8.
		d.graphemeBuf[kd] = append(d.graphemeBuf[kd], r)
	}
}

// deserializeWin32Input deserializes the Win32 input events converting
// KeyEventRecrods to bytes. Before returning the bytes, it will also try to
// decode any UTF-16 pairs that might be present in the input buffer.
func (d *TerminalReader) deserializeWin32Input(buf []byte) (int, []byte) {
	p := parserPool.Get().(*ansi.Parser)
	defer parserPool.Put(p)

	var processed int
	var state byte
	des := make([]byte, 0, len(buf))

	for len(buf) > 0 {
		seq, width, n, newState := ansi.DecodeSequence(buf, state, p)
		switch width {
		case 0:
			if p.Command() == '_' { // Win32 Input Mode
				vk, _ := p.Param(0, 0)
				if vk == 0 {
					// This is either a serialized KeyEventRecord or a UTF-16
					// pair.
					uc, _ := p.Param(2, 0)
					kd, _ := p.Param(3, 0)
					kd = clamp(kd, 0, 1) // kd is the key down state (0 or 1)
					d.storeGraphemeRune(kd, rune(uc))
					processed += n
					break
				}
			}
			fallthrough
		default:
			des = append(des, d.encodeGraphemeBufs()...)
			des = append(des, seq...)
		}

		state = newState
		buf = buf[n:]
	}

	des = append(des, d.encodeGraphemeBufs()...)

	return processed, des
}

func (d *TerminalReader) logf(format string, v ...interface{}) {
	if d.logger == nil {
		return
	}
	d.logger.Printf(format, v...)
}
