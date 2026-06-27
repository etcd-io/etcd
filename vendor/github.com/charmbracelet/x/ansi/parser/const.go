// Package parser provides ANSI escape sequence parsing functionality.
package parser

// Action is a DEC ANSI parser action.
type Action = byte

// These are the actions that the parser can take.
const (
	NoneAction Action = iota
	ClearAction
	CollectAction
	PrefixAction
	DispatchAction
	ExecuteAction
	StartAction // Start of a data string
	PutAction   // Put into the data string
	ParamAction
	PrintAction

	IgnoreAction = NoneAction
)

// ActionNames provides string names for parser actions.
var ActionNames = []string{
	"NoneAction",
	"ClearAction",
	"CollectAction",
	"PrefixAction",
	"DispatchAction",
	"ExecuteAction",
	"StartAction",
	"PutAction",
	"ParamAction",
	"PrintAction",
}

// State is a DEC ANSI parser state.
type State = byte

// These are the states that the parser can be in.
const (
	GroundState State = iota
	CsiEntryState
	CsiIntermediateState
	CsiParamState
	DcsEntryState
	DcsIntermediateState
	DcsParamState
	DcsStringState
	EscapeState
	EscapeIntermediateState
	OscStringState
	SosStringState
	PmStringState
	ApcStringState

	// Utf8State is not part of the DEC ANSI standard. It is used to handle
	// UTF-8 sequences.
	Utf8State
)

// StateNames provides string names for parser states.
var StateNames = []string{
	"GroundState",
	"CsiEntryState",
	"CsiIntermediateState",
	"CsiParamState",
	"DcsEntryState",
	"DcsIntermediateState",
	"DcsParamState",
	"DcsStringState",
	"EscapeState",
	"EscapeIntermediateState",
	"OscStringState",
	"SosStringState",
	"PmStringState",
	"ApcStringState",
	"Utf8State",
}
