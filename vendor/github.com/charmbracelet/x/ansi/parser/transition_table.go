package parser

// Table values are generated like this:
//
//	index:  currentState << IndexStateShift | charCode
//	value:  action << TransitionActionShift | nextState
const (
	TransitionActionShift = 4
	TransitionStateMask   = 15
	IndexStateShift       = 8

	// DefaultTableSize is the default size of the transition table.
	DefaultTableSize = 4096
)

// Table is a DEC ANSI transition table.
var Table = GenerateTransitionTable()

// TransitionTable is a DEC ANSI transition table.
// https://vt100.net/emu/dec_ansi_parser
type TransitionTable []byte

// NewTransitionTable returns a new DEC ANSI transition table.
func NewTransitionTable(size int) TransitionTable {
	if size <= 0 {
		size = DefaultTableSize
	}
	return TransitionTable(make([]byte, size))
}

// SetDefault sets default transition.
func (t TransitionTable) SetDefault(action Action, state State) {
	for i := range t {
		t[i] = action<<TransitionActionShift | state
	}
}

// AddOne adds a transition.
func (t TransitionTable) AddOne(code byte, state State, action Action, next State) {
	idx := int(state)<<IndexStateShift | int(code)
	value := action<<TransitionActionShift | next
	t[idx] = value
}

// AddMany adds many transitions.
func (t TransitionTable) AddMany(codes []byte, state State, action Action, next State) {
	for _, code := range codes {
		t.AddOne(code, state, action, next)
	}
}

// AddRange adds a range of transitions.
func (t TransitionTable) AddRange(start, end byte, state State, action Action, next State) {
	for i := int(start); i <= int(end); i++ {
		t.AddOne(byte(i), state, action, next)
	}
}

// Transition returns the next state and action for the given state and byte.
func (t TransitionTable) Transition(state State, code byte) (State, Action) {
	index := int(state)<<IndexStateShift | int(code)
	value := t[index]
	return value & TransitionStateMask, value >> TransitionActionShift
}

// byte range macro.
func r(start, end byte) []byte {
	var a []byte
	for i := int(start); i <= int(end); i++ {
		a = append(a, byte(i))
	}
	return a
}

// GenerateTransitionTable generates a DEC ANSI transition table compatible
// with the VT500-series of terminals. This implementation includes a few
// modifications that include:
//   - A new Utf8State is introduced to handle UTF8 sequences.
//   - Osc and Dcs data accept UTF8 sequences by extending the printable range
//     to 0xFF and 0xFE respectively.
//   - We don't ignore 0x3A (':') when building Csi and Dcs parameters and
//     instead use it to denote sub-parameters.
//   - Support dispatching SosPmApc sequences.
//   - The DEL (0x7F) character is executed in the Ground state.
//   - The DEL (0x7F) character is collected in the DcsPassthrough string state.
//   - The ST C1 control character (0x9C) is executed and not ignored.
func GenerateTransitionTable() TransitionTable {
	table := NewTransitionTable(DefaultTableSize)
	table.SetDefault(NoneAction, GroundState)

	// Anywhere
	for _, state := range r(GroundState, Utf8State) {
		// Anywhere -> Ground
		table.AddMany([]byte{0x18, 0x1a, 0x99, 0x9a}, state, ExecuteAction, GroundState)
		table.AddRange(0x80, 0x8F, state, ExecuteAction, GroundState)
		table.AddRange(0x90, 0x97, state, ExecuteAction, GroundState)
		table.AddOne(0x9C, state, ExecuteAction, GroundState)
		// Anywhere -> Escape
		table.AddOne(0x1B, state, ClearAction, EscapeState)
		// Anywhere -> SosStringState
		table.AddOne(0x98, state, StartAction, SosStringState)
		// Anywhere -> PmStringState
		table.AddOne(0x9E, state, StartAction, PmStringState)
		// Anywhere -> ApcStringState
		table.AddOne(0x9F, state, StartAction, ApcStringState)
		// Anywhere -> CsiEntry
		table.AddOne(0x9B, state, ClearAction, CsiEntryState)
		// Anywhere -> DcsEntry
		table.AddOne(0x90, state, ClearAction, DcsEntryState)
		// Anywhere -> OscString
		table.AddOne(0x9D, state, StartAction, OscStringState)
		// Anywhere -> Utf8
		table.AddRange(0xC2, 0xDF, state, CollectAction, Utf8State) // UTF8 2 byte sequence
		table.AddRange(0xE0, 0xEF, state, CollectAction, Utf8State) // UTF8 3 byte sequence
		table.AddRange(0xF0, 0xF4, state, CollectAction, Utf8State) // UTF8 4 byte sequence
	}

	// Ground
	table.AddRange(0x00, 0x17, GroundState, ExecuteAction, GroundState)
	table.AddOne(0x19, GroundState, ExecuteAction, GroundState)
	table.AddRange(0x1C, 0x1F, GroundState, ExecuteAction, GroundState)
	table.AddRange(0x20, 0x7E, GroundState, PrintAction, GroundState)
	table.AddOne(0x7F, GroundState, ExecuteAction, GroundState)

	// EscapeIntermediate
	table.AddRange(0x00, 0x17, EscapeIntermediateState, ExecuteAction, EscapeIntermediateState)
	table.AddOne(0x19, EscapeIntermediateState, ExecuteAction, EscapeIntermediateState)
	table.AddRange(0x1C, 0x1F, EscapeIntermediateState, ExecuteAction, EscapeIntermediateState)
	table.AddRange(0x20, 0x2F, EscapeIntermediateState, CollectAction, EscapeIntermediateState)
	table.AddOne(0x7F, EscapeIntermediateState, IgnoreAction, EscapeIntermediateState)
	// EscapeIntermediate -> Ground
	table.AddRange(0x30, 0x7E, EscapeIntermediateState, DispatchAction, GroundState)

	// Escape
	table.AddRange(0x00, 0x17, EscapeState, ExecuteAction, EscapeState)
	table.AddOne(0x19, EscapeState, ExecuteAction, EscapeState)
	table.AddRange(0x1C, 0x1F, EscapeState, ExecuteAction, EscapeState)
	table.AddOne(0x7F, EscapeState, IgnoreAction, EscapeState)
	// Escape -> Ground
	table.AddRange(0x30, 0x4F, EscapeState, DispatchAction, GroundState)
	table.AddRange(0x51, 0x57, EscapeState, DispatchAction, GroundState)
	table.AddOne(0x59, EscapeState, DispatchAction, GroundState)
	table.AddOne(0x5A, EscapeState, DispatchAction, GroundState)
	table.AddOne(0x5C, EscapeState, DispatchAction, GroundState)
	table.AddRange(0x60, 0x7E, EscapeState, DispatchAction, GroundState)
	// Escape -> Escape_intermediate
	table.AddRange(0x20, 0x2F, EscapeState, CollectAction, EscapeIntermediateState)
	// Escape -> Sos_pm_apc_string
	table.AddOne('X', EscapeState, StartAction, SosStringState) // SOS
	table.AddOne('^', EscapeState, StartAction, PmStringState)  // PM
	table.AddOne('_', EscapeState, StartAction, ApcStringState) // APC
	// Escape -> Dcs_entry
	table.AddOne('P', EscapeState, ClearAction, DcsEntryState)
	// Escape -> Csi_entry
	table.AddOne('[', EscapeState, ClearAction, CsiEntryState)
	// Escape -> Osc_string
	table.AddOne(']', EscapeState, StartAction, OscStringState)

	// Sos_pm_apc_string
	for _, state := range r(SosStringState, ApcStringState) {
		table.AddRange(0x00, 0x17, state, PutAction, state)
		table.AddOne(0x19, state, PutAction, state)
		table.AddRange(0x1C, 0x1F, state, PutAction, state)
		table.AddRange(0x20, 0x7F, state, PutAction, state)
		// ESC, ST, CAN, and SUB terminate the sequence
		table.AddOne(0x1B, state, DispatchAction, EscapeState)
		table.AddOne(0x9C, state, DispatchAction, GroundState)
		table.AddMany([]byte{0x18, 0x1A}, state, IgnoreAction, GroundState)
	}

	// Dcs_entry
	table.AddRange(0x00, 0x07, DcsEntryState, IgnoreAction, DcsEntryState)
	table.AddRange(0x0E, 0x17, DcsEntryState, IgnoreAction, DcsEntryState)
	table.AddOne(0x19, DcsEntryState, IgnoreAction, DcsEntryState)
	table.AddRange(0x1C, 0x1F, DcsEntryState, IgnoreAction, DcsEntryState)
	table.AddOne(0x7F, DcsEntryState, IgnoreAction, DcsEntryState)
	// Dcs_entry -> Dcs_intermediate
	table.AddRange(0x20, 0x2F, DcsEntryState, CollectAction, DcsIntermediateState)
	// Dcs_entry -> Dcs_param
	table.AddRange(0x30, 0x3B, DcsEntryState, ParamAction, DcsParamState)
	table.AddRange(0x3C, 0x3F, DcsEntryState, PrefixAction, DcsParamState)
	// Dcs_entry -> Dcs_passthrough
	table.AddRange(0x08, 0x0D, DcsEntryState, PutAction, DcsStringState) // Follows ECMA-48 § 8.3.27
	// XXX: allows passing ESC (not a ECMA-48 standard) this to allow for
	// passthrough of ANSI sequences like in Screen or Tmux passthrough mode.
	table.AddOne(0x1B, DcsEntryState, PutAction, DcsStringState)
	table.AddRange(0x40, 0x7E, DcsEntryState, StartAction, DcsStringState)

	// Dcs_intermediate
	table.AddRange(0x00, 0x17, DcsIntermediateState, IgnoreAction, DcsIntermediateState)
	table.AddOne(0x19, DcsIntermediateState, IgnoreAction, DcsIntermediateState)
	table.AddRange(0x1C, 0x1F, DcsIntermediateState, IgnoreAction, DcsIntermediateState)
	table.AddRange(0x20, 0x2F, DcsIntermediateState, CollectAction, DcsIntermediateState)
	table.AddOne(0x7F, DcsIntermediateState, IgnoreAction, DcsIntermediateState)
	// Dcs_intermediate -> Dcs_passthrough
	table.AddRange(0x30, 0x3F, DcsIntermediateState, StartAction, DcsStringState)
	table.AddRange(0x40, 0x7E, DcsIntermediateState, StartAction, DcsStringState)

	// Dcs_param
	table.AddRange(0x00, 0x17, DcsParamState, IgnoreAction, DcsParamState)
	table.AddOne(0x19, DcsParamState, IgnoreAction, DcsParamState)
	table.AddRange(0x1C, 0x1F, DcsParamState, IgnoreAction, DcsParamState)
	table.AddRange(0x30, 0x3B, DcsParamState, ParamAction, DcsParamState)
	table.AddOne(0x7F, DcsParamState, IgnoreAction, DcsParamState)
	table.AddRange(0x3C, 0x3F, DcsParamState, IgnoreAction, DcsParamState)
	// Dcs_param -> Dcs_intermediate
	table.AddRange(0x20, 0x2F, DcsParamState, CollectAction, DcsIntermediateState)
	// Dcs_param -> Dcs_passthrough
	table.AddRange(0x40, 0x7E, DcsParamState, StartAction, DcsStringState)

	// Dcs_passthrough
	table.AddRange(0x00, 0x17, DcsStringState, PutAction, DcsStringState)
	table.AddOne(0x19, DcsStringState, PutAction, DcsStringState)
	table.AddRange(0x1C, 0x1F, DcsStringState, PutAction, DcsStringState)
	table.AddRange(0x20, 0x7E, DcsStringState, PutAction, DcsStringState)
	table.AddOne(0x7F, DcsStringState, PutAction, DcsStringState)
	table.AddRange(0x80, 0xFF, DcsStringState, PutAction, DcsStringState) // Allow Utf8 characters by extending the printable range from 0x7F to 0xFF
	// ST, CAN, SUB, and ESC terminate the sequence
	table.AddOne(0x1B, DcsStringState, DispatchAction, EscapeState)
	table.AddOne(0x9C, DcsStringState, DispatchAction, GroundState)
	table.AddMany([]byte{0x18, 0x1A}, DcsStringState, IgnoreAction, GroundState)

	// Csi_param
	table.AddRange(0x00, 0x17, CsiParamState, ExecuteAction, CsiParamState)
	table.AddOne(0x19, CsiParamState, ExecuteAction, CsiParamState)
	table.AddRange(0x1C, 0x1F, CsiParamState, ExecuteAction, CsiParamState)
	table.AddRange(0x30, 0x3B, CsiParamState, ParamAction, CsiParamState)
	table.AddOne(0x7F, CsiParamState, IgnoreAction, CsiParamState)
	table.AddRange(0x3C, 0x3F, CsiParamState, IgnoreAction, CsiParamState)
	// Csi_param -> Ground
	table.AddRange(0x40, 0x7E, CsiParamState, DispatchAction, GroundState)
	// Csi_param -> Csi_intermediate
	table.AddRange(0x20, 0x2F, CsiParamState, CollectAction, CsiIntermediateState)

	// Csi_intermediate
	table.AddRange(0x00, 0x17, CsiIntermediateState, ExecuteAction, CsiIntermediateState)
	table.AddOne(0x19, CsiIntermediateState, ExecuteAction, CsiIntermediateState)
	table.AddRange(0x1C, 0x1F, CsiIntermediateState, ExecuteAction, CsiIntermediateState)
	table.AddRange(0x20, 0x2F, CsiIntermediateState, CollectAction, CsiIntermediateState)
	table.AddOne(0x7F, CsiIntermediateState, IgnoreAction, CsiIntermediateState)
	// Csi_intermediate -> Ground
	table.AddRange(0x40, 0x7E, CsiIntermediateState, DispatchAction, GroundState)
	// Csi_intermediate -> Csi_ignore
	table.AddRange(0x30, 0x3F, CsiIntermediateState, IgnoreAction, GroundState)

	// Csi_entry
	table.AddRange(0x00, 0x17, CsiEntryState, ExecuteAction, CsiEntryState)
	table.AddOne(0x19, CsiEntryState, ExecuteAction, CsiEntryState)
	table.AddRange(0x1C, 0x1F, CsiEntryState, ExecuteAction, CsiEntryState)
	table.AddOne(0x7F, CsiEntryState, IgnoreAction, CsiEntryState)
	// Csi_entry -> Ground
	table.AddRange(0x40, 0x7E, CsiEntryState, DispatchAction, GroundState)
	// Csi_entry -> Csi_intermediate
	table.AddRange(0x20, 0x2F, CsiEntryState, CollectAction, CsiIntermediateState)
	// Csi_entry -> Csi_param
	table.AddRange(0x30, 0x3B, CsiEntryState, ParamAction, CsiParamState)
	table.AddRange(0x3C, 0x3F, CsiEntryState, PrefixAction, CsiParamState)

	// Osc_string
	table.AddRange(0x00, 0x06, OscStringState, IgnoreAction, OscStringState)
	table.AddRange(0x08, 0x17, OscStringState, IgnoreAction, OscStringState)
	table.AddOne(0x19, OscStringState, IgnoreAction, OscStringState)
	table.AddRange(0x1C, 0x1F, OscStringState, IgnoreAction, OscStringState)
	table.AddRange(0x20, 0xFF, OscStringState, PutAction, OscStringState) // Allow Utf8 characters by extending the printable range from 0x7F to 0xFF

	// ST, CAN, SUB, ESC, and BEL terminate the sequence
	table.AddOne(0x1B, OscStringState, DispatchAction, EscapeState)
	table.AddOne(0x07, OscStringState, DispatchAction, GroundState)
	table.AddOne(0x9C, OscStringState, DispatchAction, GroundState)
	table.AddMany([]byte{0x18, 0x1A}, OscStringState, IgnoreAction, GroundState)

	return table
}
