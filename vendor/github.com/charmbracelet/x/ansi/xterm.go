package ansi

import "strconv"

// KeyModifierOptions (XTMODKEYS) sets/resets xterm key modifier options.
//
// Default is 0.
//
//	CSI > Pp m
//	CSI > Pp ; Pv m
//
// If Pv is omitted, the resource is reset to its initial value.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
func KeyModifierOptions(p int, vs ...int) string {
	var pp, pv string
	if p > 0 {
		pp = strconv.Itoa(p)
	}

	if len(vs) == 0 {
		return "\x1b[>" + strconv.Itoa(p) + "m"
	}

	v := vs[0]
	if v > 0 {
		pv = strconv.Itoa(v)
		return "\x1b[>" + pp + ";" + pv + "m"
	}

	return "\x1b[>" + pp + "m"
}

// XTMODKEYS is an alias for [KeyModifierOptions].
func XTMODKEYS(p int, vs ...int) string {
	return KeyModifierOptions(p, vs...)
}

// SetKeyModifierOptions sets xterm key modifier options.
// This is an alias for [KeyModifierOptions].
func SetKeyModifierOptions(pp int, pv int) string {
	return KeyModifierOptions(pp, pv)
}

// ResetKeyModifierOptions resets xterm key modifier options.
// This is an alias for [KeyModifierOptions].
func ResetKeyModifierOptions(pp int) string {
	return KeyModifierOptions(pp)
}

// QueryKeyModifierOptions (XTQMODKEYS) requests xterm key modifier options.
//
// Default is 0.
//
//	CSI ? Pp m
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
func QueryKeyModifierOptions(pp int) string {
	var p string
	if pp > 0 {
		p = strconv.Itoa(pp)
	}
	return "\x1b[?" + p + "m"
}

// XTQMODKEYS is an alias for [QueryKeyModifierOptions].
func XTQMODKEYS(pp int) string {
	return QueryKeyModifierOptions(pp)
}

// Modify Other Keys (modifyOtherKeys) is an xterm feature that allows the
// terminal to modify the behavior of certain keys to send different escape
// sequences when pressed.
//
// See: https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyOtherKeys
const (
	SetModifyOtherKeys1  = "\x1b[>4;1m"
	SetModifyOtherKeys2  = "\x1b[>4;2m"
	ResetModifyOtherKeys = "\x1b[>4m"
	QueryModifyOtherKeys = "\x1b[?4m"
)

// ModifyOtherKeys returns a sequence that sets XTerm modifyOtherKeys mode.
// The mode argument specifies the mode to set.
//
//	0: Disable modifyOtherKeys mode.
//	1: Enable modifyOtherKeys mode 1.
//	2: Enable modifyOtherKeys mode 2.
//
//	CSI > 4 ; mode m
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
// See: https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyOtherKeys
//
// Deprecated: use [SetModifyOtherKeys1] or [SetModifyOtherKeys2] instead.
func ModifyOtherKeys(mode int) string {
	return "\x1b[>4;" + strconv.Itoa(mode) + "m"
}

// DisableModifyOtherKeys disables the modifyOtherKeys mode.
//
//	CSI > 4 ; 0 m
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
// See: https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyOtherKeys
//
// Deprecated: use [ResetModifyOtherKeys] instead.
const DisableModifyOtherKeys = "\x1b[>4;0m"

// EnableModifyOtherKeys1 enables the modifyOtherKeys mode 1.
//
//	CSI > 4 ; 1 m
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
// See: https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyOtherKeys
//
// Deprecated: use [SetModifyOtherKeys1] instead.
const EnableModifyOtherKeys1 = "\x1b[>4;1m"

// EnableModifyOtherKeys2 enables the modifyOtherKeys mode 2.
//
//	CSI > 4 ; 2 m
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
// See: https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyOtherKeys
//
// Deprecated: use [SetModifyOtherKeys2] instead.
const EnableModifyOtherKeys2 = "\x1b[>4;2m"

// RequestModifyOtherKeys requests the modifyOtherKeys mode.
//
//	CSI ? 4  m
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
// See: https://invisible-island.net/xterm/manpage/xterm.html#VT100-Widget-Resources:modifyOtherKeys
//
// Deprecated: use [QueryModifyOtherKeys] instead.
const RequestModifyOtherKeys = "\x1b[?4m"
