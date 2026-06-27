package ansi

// ResetInitialState (RIS) resets the terminal to its initial state.
//
//	ESC c
//
// See: https://vt100.net/docs/vt510-rm/RIS.html
const (
	ResetInitialState = "\x1bc"
	RIS               = ResetInitialState
)
