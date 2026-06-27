package ansi

import (
	"fmt"
	"strings"
)

// Notify sends a desktop notification using iTerm's OSC 9.
//
//	OSC 9 ; Mc ST
//	OSC 9 ; Mc BEL
//
// Where Mc is the notification body.
//
// See: https://iterm2.com/documentation-escape-codes.html
func Notify(s string) string {
	return "\x1b]9;" + s + "\x07"
}

// DesktopNotification sends a desktop notification based on the extensible OSC
// 99 escape code.
//
//	OSC 99 ; <metadata> ; <payload> ST
//	OSC 99 ; <metadata> ; <payload> BEL
//
// Where <metadata> is a colon-separated list of key-value pairs, and
// <payload> is the notification body.
//
// See: https://sw.kovidgoyal.net/kitty/desktop-notifications/
func DesktopNotification(payload string, metadata ...string) string {
	return fmt.Sprintf("\x1b]99;%s;%s\x07", strings.Join(metadata, ":"), payload)
}
