package ansi

import (
	"bytes"
	"strconv"
	"strings"
)

// SixelGraphics returns a sequence that encodes the given sixel image payload to
// a DCS sixel sequence.
//
//	DCS p1; p2; p3; q [sixel payload] ST
//
// p1 = pixel aspect ratio, deprecated and replaced by pixel metrics in the payload
//
// p2 = This is supposed to be 0 for transparency, but terminals don't seem to
// to use it properly. Value 0 leaves an unsightly black bar on all terminals
// I've tried and looks correct with value 1.
//
// p3 = Horizontal grid size parameter. Everyone ignores this and uses a fixed grid
// size, as far as I can tell.
//
// See https://shuford.invisible-island.net/all_about_sixels.txt
func SixelGraphics(p1, p2, p3 int, payload []byte) string {
	var buf bytes.Buffer

	buf.WriteString("\x1bP")
	if p1 >= 0 {
		buf.WriteString(strconv.Itoa(p1))
	}
	buf.WriteByte(';')
	if p2 >= 0 {
		buf.WriteString(strconv.Itoa(p2))
	}
	if p3 > 0 {
		buf.WriteByte(';')
		buf.WriteString(strconv.Itoa(p3))
	}
	buf.WriteByte('q')
	buf.Write(payload)
	buf.WriteString("\x1b\\")

	return buf.String()
}

// KittyGraphics returns a sequence that encodes the given image in the Kitty
// graphics protocol.
//
//	APC G [comma separated options] ; [base64 encoded payload] ST
//
// See https://sw.kovidgoyal.net/kitty/graphics-protocol/
func KittyGraphics(payload []byte, opts ...string) string {
	var buf bytes.Buffer
	buf.WriteString("\x1b_G")
	buf.WriteString(strings.Join(opts, ","))
	if len(payload) > 0 {
		buf.WriteString(";")
		buf.Write(payload)
	}
	buf.WriteString("\x1b\\")
	return buf.String()
}
