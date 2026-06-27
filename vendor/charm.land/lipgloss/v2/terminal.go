package lipgloss

import (
	"fmt"
	"image/color"
	"io"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// queryBackgroundColor queries the terminal for the background color.
// If the terminal does not support querying the background color, nil is
// returned.
//
// Note: you will need to set the input to raw mode before calling this
// function.
//
//	state, _ := term.MakeRaw(in.Fd())
//	defer term.Restore(in.Fd(), state)
//
// copied from x/term@v0.1.3.
func queryBackgroundColor(in io.Reader, out io.Writer) (c color.Color, err error) {
	err = queryTerminal(in, out, defaultQueryTimeout,
		func(seq string, pa *ansi.Parser) bool {
			switch {
			case ansi.HasOscPrefix(seq):
				switch pa.Command() {
				case 11: // OSC 11
					parts := strings.Split(string(pa.Data()), ";")
					if len(parts) != 2 {
						break // invalid, but we still need to parse the next sequence
					}
					c = ansi.XParseColor(parts[1])
				}
			case ansi.HasCsiPrefix(seq):
				switch pa.Command() {
				case ansi.Command('?', 0, 'c'): // DA1
					return false
				}
			}
			return true
		}, ansi.RequestBackgroundColor+ansi.RequestPrimaryDeviceAttributes)
	return
}

const defaultQueryTimeout = time.Second * 2

// queryTerminalFilter is a function that filters input events using a type
// switch. If false is returned, the QueryTerminal function will stop reading
// input.
type queryTerminalFilter func(seq string, pa *ansi.Parser) bool

// queryTerminal queries the terminal for support of various features and
// returns a list of response events.
// Most of the time, you will need to set stdin to raw mode before calling this
// function.
// Note: This function will block until the terminal responds or the timeout
// is reached.
// copied from x/term@v0.1.3.
func queryTerminal(
	in io.Reader,
	out io.Writer,
	timeout time.Duration,
	filter queryTerminalFilter,
	query string,
) error {
	// We use [uv.NewCancelReader] because it uses a different Windows
	// implementation than the on in the [cancelreader] library, which uses
	// the Cancel IO API to cancel reads instead of using Overlapped IO.
	rd, err := uv.NewCancelReader(in)
	if err != nil {
		return fmt.Errorf("could not create cancel reader: %w", err)
	}

	defer rd.Close() //nolint: errcheck

	done := make(chan struct{}, 1)
	defer close(done)
	go func() {
		select {
		case <-done:
		case <-time.After(timeout):
			rd.Cancel()
		}
	}()

	if _, err := io.WriteString(out, query); err != nil {
		return fmt.Errorf("could not write query: %w", err)
	}

	pa := ansi.GetParser()
	defer ansi.PutParser(pa)

	var acc []byte    // Accumulate partial responses before filtering
	var buf [256]byte // 256 bytes should be enough for most responses
	var state byte
	for {
		n, err := rd.Read(buf[:])
		if err != nil {
			return fmt.Errorf("could not read from input: %w", err)
		}

		p := buf[:]
		for n > 0 {
			seq, _, read, newState := ansi.DecodeSequence(p[:n], state, pa)
			acc = append(acc, seq...)

			if newState == ansi.NormalState {
				if !filter(string(acc), pa) {
					return nil
				}

				acc = acc[:0]
			}

			state = newState
			n -= read
			p = p[read:]
		}
	}
}
