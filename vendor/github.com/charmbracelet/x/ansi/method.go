package ansi

import (
	"os"
	"strconv"

	"github.com/clipperhouse/displaywidth"
	"github.com/mattn/go-runewidth"
)

var wcOptions = &runewidth.Condition{
	EastAsianWidth:     false,
	StrictEmojiNeutral: true,
}

var dwOptions = &displaywidth.Options{
	EastAsianWidth: false,
}

func init() {
	if ea, err := strconv.ParseBool(os.Getenv("RUNEWIDTH_EASTASIAN")); err == nil && ea {
		wcOptions.EastAsianWidth = true
		dwOptions.EastAsianWidth = true
	}
}

// Method is a type that represents the how the renderer should calculate the
// display width of cells.
type Method uint8

// Display width modes.
const (
	WcWidth Method = iota
	GraphemeWidth
)

// StringWidth returns the width of a string in cells. This is the number of
// cells that the string will occupy when printed in a terminal. ANSI escape
// codes are ignored and wide characters (such as East Asians and emojis) are
// accounted for.
func (m Method) StringWidth(s string) int {
	return stringWidth(m, s)
}

// Truncate truncates a string to a given length, adding a tail to the end if
// the string is longer than the given length. This function is aware of ANSI
// escape codes and will not break them, and accounts for wide-characters (such
// as East-Asian characters and emojis).
func (m Method) Truncate(s string, length int, tail string) string {
	return truncate(m, s, length, tail)
}

// TruncateLeft truncates a string to a given length, adding a prefix to the
// beginning if the string is longer than the given length. This function is
// aware of ANSI escape codes and will not break them, and accounts for
// wide-characters (such as East-Asian characters and emojis).
func (m Method) TruncateLeft(s string, length int, prefix string) string {
	return truncateLeft(m, s, length, prefix)
}

// Cut the string, without adding any prefix or tail strings. This function is
// aware of ANSI escape codes and will not break them, and accounts for
// wide-characters (such as East-Asian characters and emojis). Note that the
// [left] parameter is inclusive, while [right] isn't.
func (m Method) Cut(s string, left, right int) string {
	return cut(m, s, left, right)
}

// Hardwrap wraps a string or a block of text to a given line length, breaking
// word boundaries. This will preserve ANSI escape codes and will account for
// wide-characters in the string.
// When preserveSpace is true, spaces at the beginning of a line will be
// preserved.
// This treats the text as a sequence of graphemes.
func (m Method) Hardwrap(s string, length int, preserveSpace bool) string {
	return hardwrap(m, s, length, preserveSpace)
}

// Wordwrap wraps a string or a block of text to a given line length, not
// breaking word boundaries. This will preserve ANSI escape codes and will
// account for wide-characters in the string.
// The breakpoints string is a list of characters that are considered
// breakpoints for word wrapping. A hyphen (-) is always considered a
// breakpoint.
//
// Note: breakpoints must be a string of 1-cell wide rune characters.
func (m Method) Wordwrap(s string, length int, breakpoints string) string {
	return wordwrap(m, s, length, breakpoints)
}

// Wrap wraps a string or a block of text to a given line length, breaking word
// boundaries if necessary. This will preserve ANSI escape codes and will
// account for wide-characters in the string. The breakpoints string is a list
// of characters that are considered breakpoints for word wrapping. A hyphen
// (-) is always considered a breakpoint.
//
// Note: breakpoints must be a string of 1-cell wide rune characters.
func (m Method) Wrap(s string, length int, breakpoints string) string {
	return wrap(m, s, length, breakpoints)
}

// DecodeSequence decodes the first ANSI escape sequence or a printable
// grapheme from the given data. It returns the sequence slice, the number of
// bytes read, the cell width for each sequence, and the new state.
//
// The cell width will always be 0 for control and escape sequences, 1 for
// ASCII printable characters, and the number of cells other Unicode characters
// occupy. It uses the uniseg package to calculate the width of Unicode
// graphemes and characters. This means it will always do grapheme clustering
// (mode 2027).
//
// Passing a non-nil [*Parser] as the last argument will allow the decoder to
// collect sequence parameters, data, and commands. The parser cmd will have
// the packed command value that contains intermediate and prefix characters.
// In the case of a OSC sequence, the cmd will be the OSC command number. Use
// [Cmd] and [Param] types to unpack command intermediates and prefixes as well
// as parameters.
//
// Zero [Cmd] means the CSI, DCS, or ESC sequence is invalid. Moreover, checking the
// validity of other data sequences, OSC, DCS, etc, will require checking for
// the returned sequence terminator bytes such as ST (ESC \\) and BEL).
//
// We store the command byte in [Cmd] in the most significant byte, the
// prefix byte in the next byte, and the intermediate byte in the least
// significant byte. This is done to avoid using a struct to store the command
// and its intermediates and prefixes. The command byte is always the least
// significant byte i.e. [Cmd & 0xff]. Use the [Cmd] type to unpack the
// command, intermediate, and prefix bytes. Note that we only collect the last
// prefix character and intermediate byte.
//
// The [p.Params] slice will contain the parameters of the sequence. Any
// sub-parameter will have the [parser.HasMoreFlag] set. Use the [Param] type
// to unpack the parameters.
//
// Example:
//
//	var state byte // the initial state is always zero [NormalState]
//	p := NewParser(32, 1024) // create a new parser with a 32 params buffer and 1024 data buffer (optional)
//	input := []byte("\x1b[31mHello, World!\x1b[0m")
//	for len(input) > 0 {
//		seq, width, n, newState := DecodeSequence(input, state, p)
//		log.Printf("seq: %q, width: %d", seq, width)
//		state = newState
//		input = input[n:]
//	}
func (m Method) DecodeSequence(data []byte, state byte, p *Parser) (seq []byte, width, n int, newState byte) {
	return decodeSequence(m, data, state, p)
}

// DecodeSequenceInString decodes the first ANSI escape sequence or a printable
// grapheme from the given data. It returns the sequence slice, the number of
// bytes read, the cell width for each sequence, and the new state.
//
// The cell width will always be 0 for control and escape sequences, 1 for
// ASCII printable characters, and the number of cells other Unicode characters
// occupy. It uses the uniseg package to calculate the width of Unicode
// graphemes and characters. This means it will always do grapheme clustering
// (mode 2027).
//
// Passing a non-nil [*Parser] as the last argument will allow the decoder to
// collect sequence parameters, data, and commands. The parser cmd will have
// the packed command value that contains intermediate and prefix characters.
// In the case of a OSC sequence, the cmd will be the OSC command number. Use
// [Cmd] and [Param] types to unpack command intermediates and prefixes as well
// as parameters.
//
// Zero [Cmd] means the CSI, DCS, or ESC sequence is invalid. Moreover, checking the
// validity of other data sequences, OSC, DCS, etc, will require checking for
// the returned sequence terminator bytes such as ST (ESC \\) and BEL).
//
// We store the command byte in [Cmd] in the most significant byte, the
// prefix byte in the next byte, and the intermediate byte in the least
// significant byte. This is done to avoid using a struct to store the command
// and its intermediates and prefixes. The command byte is always the least
// significant byte i.e. [Cmd & 0xff]. Use the [Cmd] type to unpack the
// command, intermediate, and prefix bytes. Note that we only collect the last
// prefix character and intermediate byte.
//
// The [p.Params] slice will contain the parameters of the sequence. Any
// sub-parameter will have the [parser.HasMoreFlag] set. Use the [Param] type
// to unpack the parameters.
//
// Example:
//
//	var state byte // the initial state is always zero [NormalState]
//	p := NewParser(32, 1024) // create a new parser with a 32 params buffer and 1024 data buffer (optional)
//	input := []byte("\x1b[31mHello, World!\x1b[0m")
//	for len(input) > 0 {
//		seq, width, n, newState := DecodeSequenceInString(input, state, p)
//		log.Printf("seq: %q, width: %d", seq, width)
//		state = newState
//		input = input[n:]
//	}
func (m Method) DecodeSequenceInString(data string, state byte, p *Parser) (seq string, width, n int, newState byte) {
	return decodeSequence(m, data, state, p)
}
