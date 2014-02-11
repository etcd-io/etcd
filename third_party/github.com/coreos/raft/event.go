package raft

const (
	StateChangeEventType  = "stateChange"
	LeaderChangeEventType = "leaderChange"
	TermChangeEventType   = "termChange"
	CommitEventType   = "commit"
	AddPeerEventType      = "addPeer"
	RemovePeerEventType   = "removePeer"

	HeartbeatIntervalEventType        = "heartbeatInterval"
	ElectionTimeoutThresholdEventType = "electionTimeoutThreshold"

	HeartbeatEventType = "heartbeat"
)

// Event represents an action that occurred within the Raft library.
// Listeners can subscribe to event types by using the Server.AddEventListener() function.
type Event interface {
	Type() string
	Source() interface{}
	Value() interface{}
	PrevValue() interface{}
}

// event is the concrete implementation of the Event interface.
type event struct {
	typ       string
	source    interface{}
	value     interface{}
	prevValue interface{}
}

// newEvent creates a new event.
func newEvent(typ string, value interface{}, prevValue interface{}) *event {
	return &event{
		typ:       typ,
		value:     value,
		prevValue: prevValue,
	}
}

// Type returns the type of event that occurred.
func (e *event) Type() string {
	return e.typ
}

// Source returns the object that dispatched the event.
func (e *event) Source() interface{} {
	return e.source
}

// Value returns the current value associated with the event, if applicable.
func (e *event) Value() interface{} {
	return e.value
}

// PrevValue returns the previous value associated with the event, if applicable.
func (e *event) PrevValue() interface{} {
	return e.prevValue
}
