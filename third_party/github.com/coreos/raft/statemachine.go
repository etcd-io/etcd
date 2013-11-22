package raft

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// StateMachine is the interface for allowing the host application to save and
// recovery the state machine
type StateMachine interface {
	Save() ([]byte, error)
	Recovery([]byte) error
}
