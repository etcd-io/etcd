package ansi

import "unsafe"

// Params represents a list of packed parameters.
type Params []Param

// Param returns the parameter at the given index and if it is part of a
// sub-parameters. It falls back to the default value if the parameter is
// missing. If the index is out of bounds, it returns the default value and
// false.
func (p Params) Param(i, def int) (int, bool, bool) {
	if i < 0 || i >= len(p) {
		return def, false, false
	}
	return p[i].Param(def), p[i].HasMore(), true
}

// ForEach iterates over the parameters and calls the given function for each
// parameter. If a parameter is part of a sub-parameter, it will be called with
// hasMore set to true.
// Use def to set a default value for missing parameters.
func (p Params) ForEach(def int, f func(i, param int, hasMore bool)) {
	for i := range p {
		f(i, p[i].Param(def), p[i].HasMore())
	}
}

// ToParams converts a list of integers to a list of parameters.
func ToParams(params []int) Params {
	return unsafe.Slice((*Param)(unsafe.Pointer(&params[0])), len(params))
}

// Handler handles actions performed by the parser.
// It is used to handle ANSI escape sequences, control characters, and runes.
type Handler struct {
	// Print is called when a printable rune is encountered.
	Print func(r rune)
	// Execute is called when a control character is encountered.
	Execute func(b byte)
	// HandleCsi is called when a CSI sequence is encountered.
	HandleCsi func(cmd Cmd, params Params)
	// HandleEsc is called when an ESC sequence is encountered.
	HandleEsc func(cmd Cmd)
	// HandleDcs is called when a DCS sequence is encountered.
	HandleDcs func(cmd Cmd, params Params, data []byte)
	// HandleOsc is called when an OSC sequence is encountered.
	HandleOsc func(cmd int, data []byte)
	// HandlePm is called when a PM sequence is encountered.
	HandlePm func(data []byte)
	// HandleApc is called when an APC sequence is encountered.
	HandleApc func(data []byte)
	// HandleSos is called when a SOS sequence is encountered.
	HandleSos func(data []byte)
}

// SetHandler sets the handler for the parser.
func (p *Parser) SetHandler(h Handler) {
	p.handler = h
}
