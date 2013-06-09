package main
//------------------------------------------------------------------------------
//
// Commands
//
//------------------------------------------------------------------------------

import (
	"github.com/benbjohnson/go-raft"
	"encoding/json"
	)


// A command represents an action to be taken on the replicated state machine.
type Command interface {
	CommandName() string
	Apply(server *raft.Server) ([]byte, error)
	GeneratePath() string
	Type() string
	GetValue() string
	GetKey() string
}

// Set command
type SetCommand struct {
	Key string `json:"key"`
	Value string `json:"value"`
}

// The name of the command in the log
func (c *SetCommand) CommandName() string {
	return "set"
}

// Set the value of key to value
func (c *SetCommand) Apply(server *raft.Server) ([]byte, error) {
	res := s.Set(c.Key, c.Value)
	return json.Marshal(res)
}

func (c *SetCommand) GeneratePath() string{
	return "/set/" + c.Key
}

func (c *SetCommand) Type() string{
	return "POST"
}

func (c *SetCommand) GetValue() string{
	return c.Value
}

func (c *SetCommand) GetKey() string{
	return c.Key
}


// Get command
type GetCommand struct {
	Key string `json:"key"`
}

// The name of the command in the log
func (c *GetCommand) CommandName() string {
	return "get"
}

// Set the value of key to value
func (c *GetCommand) Apply(server *raft.Server) ([]byte, error){
	res := s.Get(c.Key)
	return json.Marshal(res)
}

func (c *GetCommand) GeneratePath() string{
	return "/get/" + c.Key
}

func (c *GetCommand) Type() string{
	return "GET"
}

func (c *GetCommand) GetValue() string{
	return ""
}

func (c *GetCommand) GetKey() string{
	return c.Key
}


// Delete command
type DeleteCommand struct {
	Key string `json:"key"`
}

// The name of the command in the log
func (c *DeleteCommand) CommandName() string {
	return "delete"
}

// Set the value of key to value
func (c *DeleteCommand) Apply(server *raft.Server) ([]byte, error){
	res := s.Delete(c.Key)
	return json.Marshal(res)
}

func (c *DeleteCommand) GeneratePath() string{
	return "/delete/" + c.Key
}

func (c *DeleteCommand) Type() string{
	return "GET"
}

func (c *DeleteCommand) GetValue() string{
	return ""
}

func (c *DeleteCommand) GetKey() string{
	return c.Key
}

// joinCommand
type joinCommand struct {
	Name string `json:"name"`
}

func (c *joinCommand) CommandName() string {
	return "join"
}

func (c *joinCommand) Apply(server *raft.Server) ([]byte, error) {
	err := server.AddPeer(c.Name)
	return nil, err
}
