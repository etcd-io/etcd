package ansi

import "strings"

// SetHyperlink returns a sequence for starting a hyperlink.
//
//	OSC 8 ; Params ; Uri ST
//	OSC 8 ; Params ; Uri BEL
//
// To reset the hyperlink, omit the URI.
//
// See: https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
func SetHyperlink(uri string, params ...string) string {
	var p string
	if len(params) > 0 {
		p = strings.Join(params, ":")
	}
	return "\x1b]8;" + p + ";" + uri + "\x07"
}

// ResetHyperlink returns a sequence for resetting the hyperlink.
//
// This is equivalent to SetHyperlink("", params...).
//
// See: https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
func ResetHyperlink(params ...string) string {
	return SetHyperlink("", params...)
}
