package raft

import (
	"io"
)

// NOP command
type NOPCommand struct {
}

// The name of the NOP command in the log
func (c NOPCommand) CommandName() string {
	return "raft:nop"
}

func (c NOPCommand) Apply(server Server) (interface{}, error) {
	return nil, nil
}

func (c NOPCommand) Encode(w io.Writer) error {
	return nil
}

func (c NOPCommand) Decode(r io.Reader) error {
	return nil
}
