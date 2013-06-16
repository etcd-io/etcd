package main
//------------------------------------------------------------------------------
//
// Commands
//
//------------------------------------------------------------------------------

import (
	"github.com/benbjohnson/go-raft"
	"encoding/json"
	"time"
	)

// A command represents an action to be taken on the replicated state machine.
type Command interface {
	CommandName() string
	Apply(server *raft.Server) ([]byte, error)
	GeneratePath() string // Gererate a path for http request
	Type() string // http request type
	GetValue() string
	GetKey() string
	Sensitive() bool // Sensitive to the stateMachine
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
	res := s.Set(c.Key, c.Value, time.Unix(0, 0))
	return json.Marshal(res)
}

// Get the path for http request
func (c *SetCommand) GeneratePath() string {
	return "set/" + c.Key
}

// Get the type for http request
func (c *SetCommand) Type() string {
	return "POST"
}

func (c *SetCommand) GetValue() string {
	return c.Value
}

func (c *SetCommand) GetKey() string {
	return c.Key
}

func (c *SetCommand) Sensitive() bool {
	return true
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
	return "get/" + c.Key
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

func (c *GetCommand) Sensitive() bool {
	return false
}


// Delete command
type DeleteCommand struct {
	Key string `json:"key"`
}

// The name of the command in the log
func (c *DeleteCommand) CommandName() string {
	return "delete"
}

// Delete the key 
func (c *DeleteCommand) Apply(server *raft.Server) ([]byte, error){
	res := s.Delete(c.Key)
	return json.Marshal(res)
}

func (c *DeleteCommand) GeneratePath() string{
	return "delete/" + c.Key
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

func (c *DeleteCommand) Sensitive() bool {
	return true
}


// Watch command
type WatchCommand struct {
	Key string `json:"key"`
}

//The name of the command in the log
func (c *WatchCommand) CommandName() string {
	return "watch"
}

func (c *WatchCommand) Apply(server *raft.Server) ([]byte, error){
	ch := make(chan Response)

	// add to the watchers list
	w.add(c.Key, ch)	

	// wait for the notification for any changing
	res := <- ch

	return json.Marshal(res)
}

func (c *WatchCommand) GeneratePath() string{
	return "watch/" + c.Key
}

func (c *WatchCommand) Type() string{
	return "GET"
}

func (c *WatchCommand) GetValue() string{
	return ""
}

func (c *WatchCommand) GetKey() string{
	return c.Key
}

func (c *WatchCommand) Sensitive() bool {
	return false
}


// JoinCommand
type JoinCommand struct {
	Name string `json:"name"`
}

func (c *JoinCommand) CommandName() string {
	return "join"
}

func (c *JoinCommand) Apply(server *raft.Server) ([]byte, error) {
	err := server.AddPeer(c.Name)
	// no result will be returned
	return nil, err
}
