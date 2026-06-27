package ansi

import (
	"unicode/utf8"
	"unsafe"

	"github.com/charmbracelet/x/ansi/parser"
)

// Parser represents a DEC ANSI compatible sequence parser.
//
// It uses a state machine to parse ANSI escape sequences and control
// characters. The parser is designed to be used with a terminal emulator or
// similar application that needs to parse ANSI escape sequences and control
// characters.
// See package [parser] for more information.
//
//go:generate go run ./gen.go
type Parser struct {
	handler Handler

	// params contains the raw parameters of the sequence.
	// These parameters used when constructing CSI and DCS sequences.
	params []int

	// data contains the raw data of the sequence.
	// These data used when constructing OSC, DCS, SOS, PM, and APC sequences.
	data []byte

	// dataLen keeps track of the length of the data buffer.
	// If dataLen is -1, the data buffer is unlimited and will grow as needed.
	// Otherwise, dataLen is limited by the size of the data buffer.
	dataLen int

	// paramsLen keeps track of the number of parameters.
	// This is limited by the size of the params buffer.
	//
	// This is also used when collecting UTF-8 runes to keep track of the
	// number of rune bytes collected.
	paramsLen int

	// cmd contains the raw command along with the private prefix and
	// intermediate bytes of the sequence.
	// The first lower byte contains the command byte, the next byte contains
	// the private prefix, and the next byte contains the intermediate byte.
	//
	// This is also used when collecting UTF-8 runes treating it as a slice of
	// 4 bytes.
	cmd int

	// state is the current state of the parser.
	state byte
}

// NewParser returns a new parser with the default settings.
// The [Parser] uses a default size of 32 for the parameters and 64KB for the
// data buffer. Use [Parser.SetParamsSize] and [Parser.SetDataSize] to set the
// size of the parameters and data buffer respectively.
func NewParser() *Parser {
	p := new(Parser)
	p.SetParamsSize(parser.MaxParamsSize)
	p.SetDataSize(1024 * 64) // 64KB data buffer
	return p
}

// SetParamsSize sets the size of the parameters buffer.
// This is used when constructing CSI and DCS sequences.
func (p *Parser) SetParamsSize(size int) {
	p.params = make([]int, size)
}

// SetDataSize sets the size of the data buffer.
// This is used when constructing OSC, DCS, SOS, PM, and APC sequences.
// If size is less than or equal to 0, the data buffer is unlimited and will
// grow as needed.
func (p *Parser) SetDataSize(size int) {
	if size <= 0 {
		size = 0
		p.dataLen = -1
	}
	p.data = make([]byte, size)
}

// Params returns the list of parsed packed parameters.
func (p *Parser) Params() Params {
	return unsafe.Slice((*Param)(unsafe.Pointer(&p.params[0])), p.paramsLen)
}

// Param returns the parameter at the given index and falls back to the default
// value if the parameter is missing. If the index is out of bounds, it returns
// the default value and false.
func (p *Parser) Param(i, def int) (int, bool) {
	if i < 0 || i >= p.paramsLen {
		return def, false
	}
	return Param(p.params[i]).Param(def), true
}

// Command returns the packed command of the last dispatched sequence. Use
// [Cmd] to unpack the command.
func (p *Parser) Command() int {
	return p.cmd
}

// Rune returns the last dispatched sequence as a rune.
func (p *Parser) Rune() rune {
	rw := utf8ByteLen(byte(p.cmd & 0xff))
	if rw == -1 {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRune((*[utf8.UTFMax]byte)(unsafe.Pointer(&p.cmd))[:rw])
	return r
}

// Control returns the last dispatched sequence as a control code.
func (p *Parser) Control() byte {
	return byte(p.cmd & 0xff)
}

// Data returns the raw data of the last dispatched sequence.
func (p *Parser) Data() []byte {
	return p.data[:p.dataLen]
}

// Reset resets the parser to its initial state.
func (p *Parser) Reset() {
	p.clear()
	p.state = parser.GroundState
}

// clear clears the parser parameters and command.
func (p *Parser) clear() {
	if len(p.params) > 0 {
		p.params[0] = parser.MissingParam
	}
	p.paramsLen = 0
	p.cmd = 0
}

// State returns the current state of the parser.
func (p *Parser) State() parser.State {
	return p.state
}

// StateName returns the name of the current state.
func (p *Parser) StateName() string {
	return parser.StateNames[p.state]
}

// Parse parses the given dispatcher and byte buffer.
// Deprecated: Loop over the buffer and call [Parser.Advance] instead.
func (p *Parser) Parse(b []byte) {
	for i := range b {
		p.Advance(b[i])
	}
}

// Advance advances the parser using the given byte. It	returns the action
// performed by the parser.
func (p *Parser) Advance(b byte) parser.Action {
	switch p.state {
	case parser.Utf8State:
		// We handle UTF-8 here.
		return p.advanceUtf8(b)
	default:
		return p.advance(b)
	}
}

func (p *Parser) collectRune(b byte) {
	if p.paramsLen >= utf8.UTFMax {
		return
	}

	shift := p.paramsLen * 8
	p.cmd &^= 0xff << shift
	p.cmd |= int(b) << shift
	p.paramsLen++
}

func (p *Parser) advanceUtf8(b byte) parser.Action {
	// Collect UTF-8 rune bytes.
	p.collectRune(b)
	rw := utf8ByteLen(byte(p.cmd & 0xff))
	if rw == -1 {
		// We panic here because the first byte comes from the state machine,
		// if this panics, it means there is a bug in the state machine!
		panic("invalid rune") // unreachable
	}

	if p.paramsLen < rw {
		return parser.CollectAction
	}

	// We have enough bytes to decode the rune using unsafe
	if p.handler.Print != nil {
		p.handler.Print(p.Rune())
	}

	p.state = parser.GroundState
	p.paramsLen = 0

	return parser.PrintAction
}

func (p *Parser) advance(b byte) parser.Action {
	state, action := parser.Table.Transition(p.state, b)

	// We need to clear the parser state if the state changes from EscapeState.
	// This is because when we enter the EscapeState, we don't get a chance to
	// clear the parser state. For example, when a sequence terminates with a
	// ST (\x1b\\ or \x9c), we dispatch the current sequence and transition to
	// EscapeState. However, the parser state is not cleared in this case and
	// we need to clear it here before dispatching the esc sequence.
	if p.state != state {
		if p.state == parser.EscapeState {
			p.performAction(parser.ClearAction, state, b)
		}
		if action == parser.PutAction &&
			p.state == parser.DcsEntryState && state == parser.DcsStringState {
			// XXX: This is a special case where we need to start collecting
			// non-string parameterized data i.e. doesn't follow the ECMA-48 §
			// 5.4.1 string parameters format.
			p.performAction(parser.StartAction, state, 0)
		}
	}

	// Handle special cases
	switch {
	case b == ESC && p.state == parser.EscapeState:
		// Two ESCs in a row
		p.performAction(parser.ExecuteAction, state, b)
	default:
		p.performAction(action, state, b)
	}

	p.state = state

	return action
}

func (p *Parser) parseStringCmd() {
	// Try to parse the command
	datalen := len(p.data)
	if p.dataLen >= 0 {
		datalen = p.dataLen
	}
	for i := range datalen {
		d := p.data[i]
		if d < '0' || d > '9' {
			break
		}
		if p.cmd == parser.MissingCommand {
			p.cmd = 0
		}
		p.cmd *= 10
		p.cmd += int(d - '0')
	}
}

func (p *Parser) performAction(action parser.Action, state parser.State, b byte) {
	switch action {
	case parser.IgnoreAction:
		break

	case parser.ClearAction:
		p.clear()

	case parser.PrintAction:
		p.cmd = int(b)
		if p.handler.Print != nil {
			p.handler.Print(rune(b))
		}

	case parser.ExecuteAction:
		p.cmd = int(b)
		if p.handler.Execute != nil {
			p.handler.Execute(b)
		}

	case parser.PrefixAction:
		// Collect private prefix
		// we only store the last prefix
		p.cmd &^= 0xff << parser.PrefixShift
		p.cmd |= int(b) << parser.PrefixShift

	case parser.CollectAction:
		if state == parser.Utf8State {
			// Reset the UTF-8 counter
			p.paramsLen = 0
			p.collectRune(b)
		} else {
			// Collect intermediate bytes
			// we only store the last intermediate byte
			p.cmd &^= 0xff << parser.IntermedShift
			p.cmd |= int(b) << parser.IntermedShift
		}

	case parser.ParamAction:
		// Collect parameters
		if p.paramsLen >= len(p.params) {
			break
		}

		if b >= '0' && b <= '9' {
			if p.params[p.paramsLen] == parser.MissingParam {
				p.params[p.paramsLen] = 0
			}

			p.params[p.paramsLen] *= 10
			p.params[p.paramsLen] += int(b - '0')
		}

		if b == ':' {
			p.params[p.paramsLen] |= parser.HasMoreFlag
		}

		if b == ';' || b == ':' {
			p.paramsLen++
			if p.paramsLen < len(p.params) {
				p.params[p.paramsLen] = parser.MissingParam
			}
		}

	case parser.StartAction:
		if p.dataLen < 0 && p.data != nil {
			p.data = p.data[:0]
		} else {
			p.dataLen = 0
		}
		if p.state >= parser.DcsEntryState && p.state <= parser.DcsStringState {
			// Collect the command byte for DCS
			p.cmd |= int(b)
		} else {
			p.cmd = parser.MissingCommand
		}

	case parser.PutAction:
		switch p.state {
		case parser.OscStringState:
			if b == ';' && p.cmd == parser.MissingCommand {
				p.parseStringCmd()
			}
		}

		if p.dataLen < 0 {
			p.data = append(p.data, b)
		} else {
			if p.dataLen < len(p.data) {
				p.data[p.dataLen] = b
				p.dataLen++
			}
		}

	case parser.DispatchAction:
		// Increment the last parameter
		if p.paramsLen > 0 && p.paramsLen < len(p.params)-1 ||
			p.paramsLen == 0 && len(p.params) > 0 && p.params[0] != parser.MissingParam {
			p.paramsLen++
		}

		if p.state == parser.OscStringState && p.cmd == parser.MissingCommand {
			// Ensure we have a command for OSC
			p.parseStringCmd()
		}

		data := p.data
		if p.dataLen >= 0 {
			data = data[:p.dataLen]
		}
		switch p.state {
		case parser.CsiEntryState, parser.CsiParamState, parser.CsiIntermediateState:
			p.cmd |= int(b)
			if p.handler.HandleCsi != nil {
				p.handler.HandleCsi(Cmd(p.cmd), p.Params())
			}
		case parser.EscapeState, parser.EscapeIntermediateState:
			p.cmd |= int(b)
			if p.handler.HandleEsc != nil {
				p.handler.HandleEsc(Cmd(p.cmd))
			}
		case parser.DcsEntryState, parser.DcsParamState, parser.DcsIntermediateState, parser.DcsStringState:
			if p.handler.HandleDcs != nil {
				p.handler.HandleDcs(Cmd(p.cmd), p.Params(), data)
			}
		case parser.OscStringState:
			if p.handler.HandleOsc != nil {
				p.handler.HandleOsc(p.cmd, data)
			}
		case parser.SosStringState:
			if p.handler.HandleSos != nil {
				p.handler.HandleSos(data)
			}
		case parser.PmStringState:
			if p.handler.HandlePm != nil {
				p.handler.HandlePm(data)
			}
		case parser.ApcStringState:
			if p.handler.HandleApc != nil {
				p.handler.HandleApc(data)
			}
		}
	}
}

func utf8ByteLen(b byte) int {
	if b <= 0b0111_1111 { // 0x00-0x7F
		return 1
	} else if b >= 0b1100_0000 && b <= 0b1101_1111 { // 0xC0-0xDF
		return 2
	} else if b >= 0b1110_0000 && b <= 0b1110_1111 { // 0xE0-0xEF
		return 3
	} else if b >= 0b1111_0000 && b <= 0b1111_0111 { // 0xF0-0xF7
		return 4
	}
	return -1
}
