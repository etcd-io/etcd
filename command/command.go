package command

import (
	"github.com/coreos/go-raft"
)

// A command represents an action to be taken on the replicated state machine.
type Command interface {
	CommandName() string
	Apply(server *raft.Server) (interface{}, error)
}

// Registers commands to the Raft library.
func Register() {
	raft.RegisterCommand(&DeleteCommand{})
	raft.RegisterCommand(&TestAndSetCommand{})
	raft.RegisterCommand(&CreateCommand{})
	raft.RegisterCommand(&UpdateCommand{})
}
