package raft

// Context represents the current state of the server. It is passed into
// a command when the command is being applied since the server methods
// are locked.
type Context interface {
	Server() Server
	CurrentTerm() uint64
	CurrentIndex() uint64
	CommitIndex() uint64
}

// context is the concrete implementation of Context.
type context struct {
	server       Server
	currentIndex uint64
	currentTerm  uint64
	commitIndex  uint64
}

// Server returns a reference to the server.
func (c *context) Server() Server {
	return c.server
}

// CurrentTerm returns current term the server is in.
func (c *context) CurrentTerm() uint64 {
	return c.currentTerm
}

// CurrentIndex returns current index the server is at.
func (c *context) CurrentIndex() uint64 {
	return c.currentIndex
}

// CommitIndex returns last commit index the server is at.
func (c *context) CommitIndex() uint64 {
	return c.commitIndex
}
