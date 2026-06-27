package ansi

import (
	"strconv"
	"strings"
)

const (
	// ResizeWindowWinOp is a window operation that resizes the terminal
	// window.
	//
	// Deprecated: Use constant number directly with [WindowOp].
	ResizeWindowWinOp = 4

	// RequestWindowSizeWinOp is a window operation that requests a report of
	// the size of the terminal window in pixels. The response is in the form:
	//  CSI 4 ; height ; width t
	//
	// Deprecated: Use constant number directly with [WindowOp].
	RequestWindowSizeWinOp = 14

	// RequestCellSizeWinOp is a window operation that requests a report of
	// the size of the terminal cell size in pixels. The response is in the form:
	//  CSI 6 ; height ; width t
	//
	// Deprecated: Use constant number directly with [WindowOp].
	RequestCellSizeWinOp = 16
)

// WindowOp (XTWINOPS) is a sequence that manipulates the terminal window.
//
//	CSI Ps ; Ps ; Ps t
//
// Ps is a semicolon-separated list of parameters.
// See https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h4-Functions-using-CSI-_-ordered-by-the-final-character-lparen-s-rparen:CSI-Ps;Ps;Ps-t.1EB0
func WindowOp(p int, ps ...int) string {
	if p <= 0 {
		return ""
	}

	if len(ps) == 0 {
		return "\x1b[" + strconv.Itoa(p) + "t"
	}

	params := make([]string, 0, len(ps)+1)
	params = append(params, strconv.Itoa(p))
	for _, p := range ps {
		if p >= 0 {
			params = append(params, strconv.Itoa(p))
		}
	}

	return "\x1b[" + strings.Join(params, ";") + "t"
}

// XTWINOPS is an alias for [WindowOp].
func XTWINOPS(p int, ps ...int) string {
	return WindowOp(p, ps...)
}
