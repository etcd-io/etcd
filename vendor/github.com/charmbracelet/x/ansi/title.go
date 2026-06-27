package ansi

// SetIconNameWindowTitle returns a sequence for setting the icon name and
// window title.
//
//	OSC 0 ; title ST
//	OSC 0 ; title BEL
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Operating-System-Commands
func SetIconNameWindowTitle(s string) string {
	return "\x1b]0;" + s + "\x07"
}

// SetIconName returns a sequence for setting the icon name.
//
//	OSC 1 ; title ST
//	OSC 1 ; title BEL
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Operating-System-Commands
func SetIconName(s string) string {
	return "\x1b]1;" + s + "\x07"
}

// SetWindowTitle returns a sequence for setting the window title.
//
//	OSC 2 ; title ST
//	OSC 2 ; title BEL
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Operating-System-Commands
func SetWindowTitle(s string) string {
	return "\x1b]2;" + s + "\x07"
}

// DECSWT is a sequence for setting the window title.
//
// This is an alias for [SetWindowTitle]("1;<name>").
// See: EK-VT520-RM 5–156 https://vt100.net/dec/ek-vt520-rm.pdf
func DECSWT(name string) string {
	return SetWindowTitle("1;" + name)
}

// DECSIN is a sequence for setting the icon name.
//
// This is an alias for [SetWindowTitle]("L;<name>").
// See: EK-VT520-RM 5–134 https://vt100.net/dec/ek-vt520-rm.pdf
func DECSIN(name string) string {
	return SetWindowTitle("L;" + name)
}
