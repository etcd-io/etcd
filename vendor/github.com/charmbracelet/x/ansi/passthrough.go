package ansi

import (
	"bytes"
)

// ScreenPassthrough wraps the given ANSI sequence in a DCS passthrough
// sequence to be sent to the outer terminal. This is used to send raw escape
// sequences to the outer terminal when running inside GNU Screen.
//
//	DCS <data> ST
//
// Note: Screen limits the length of string sequences to 768 bytes (since 2014).
// Use zero to indicate no limit, otherwise, this will chunk the returned
// string into limit sized chunks.
//
// See: https://www.gnu.org/software/screen/manual/screen.html#String-Escapes
// See: https://git.savannah.gnu.org/cgit/screen.git/tree/src/screen.h?id=c184c6ec27683ff1a860c45be5cf520d896fd2ef#n44
func ScreenPassthrough(seq string, limit int) string {
	var b bytes.Buffer
	b.WriteString("\x1bP")
	if limit > 0 {
		for i := 0; i < len(seq); i += limit {
			end := min(i+limit, len(seq))
			b.WriteString(seq[i:end])
			if end < len(seq) {
				b.WriteString("\x1b\\\x1bP")
			}
		}
	} else {
		b.WriteString(seq)
	}
	b.WriteString("\x1b\\")
	return b.String()
}

// TmuxPassthrough wraps the given ANSI sequence in a special DCS passthrough
// sequence to be sent to the outer terminal. This is used to send raw escape
// sequences to the outer terminal when running inside Tmux.
//
//	DCS tmux ; <escaped-data> ST
//
// Where <escaped-data> is the given sequence in which all occurrences of ESC
// (0x1b) are doubled i.e. replaced with ESC ESC (0x1b 0x1b).
//
// Note: this needs the `allow-passthrough` option to be set to `on`.
//
// See: https://github.com/tmux/tmux/wiki/FAQ#what-is-the-passthrough-escape-sequence-and-how-do-i-use-it
func TmuxPassthrough(seq string) string {
	var b bytes.Buffer
	b.WriteString("\x1bPtmux;")
	for i := range len(seq) {
		if seq[i] == ESC {
			b.WriteByte(ESC)
		}
		b.WriteByte(seq[i])
	}
	b.WriteString("\x1b\\")
	return b.String()
}
