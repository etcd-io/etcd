package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

const commandPrefix = "etcd:"

func commandName(name string) string {
	return commandPrefix + name
}

// A command represents an action to be taken on the replicated state machine.
type Command interface {
	CommandName() string
	Apply(server *raft.Server) (interface{}, error)
}

// Create command
type CreateCommand struct {
	Key               string    `json:"key"`
	Value             string    `json:"value"`
	ExpireTime        time.Time `json:"expireTime"`
	IncrementalSuffix bool      `json:"incrementalSuffix"`
	Force             bool      `json:"force"`
}

// The name of the create command in the log
func (c *CreateCommand) CommandName() string {
	return commandName("create")
}

// Create node
func (c *CreateCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)

	e, err := s.Create(c.Key, c.Value, c.IncrementalSuffix, c.Force, c.ExpireTime, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return e, nil
}

// Update command
type UpdateCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
}

// The name of the update command in the log
func (c *UpdateCommand) CommandName() string {
	return commandName("update")
}

// Update node
func (c *UpdateCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)

	e, err := s.Update(c.Key, c.Value, c.ExpireTime, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return e, nil
}

// TestAndSet command
type TestAndSetCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
	PrevValue  string    `json: prevValue`
	PrevIndex  uint64    `json: prevValue`
}

// The name of the testAndSet command in the log
func (c *TestAndSetCommand) CommandName() string {
	return commandName("testAndSet")
}

// Set the key-value pair if the current value of the key equals to the given prevValue
func (c *TestAndSetCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)

	e, err := s.TestAndSet(c.Key, c.PrevValue, c.PrevIndex,
		c.Value, c.ExpireTime, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return e, nil
}

// Get command
type GetCommand struct {
	Key       string `json:"key"`
	Recursive bool   `json:"recursive"`
	Sorted    bool   `json:"sorted"`
}

// The name of the get command in the log
func (c *GetCommand) CommandName() string {
	return commandName("get")
}

// Get the value of key
func (c *GetCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)

	e, err := s.Get(c.Key, c.Recursive, c.Sorted, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return e, nil
}

// Delete command
type DeleteCommand struct {
	Key       string `json:"key"`
	Recursive bool   `json:"recursive"`
}

// The name of the delete command in the log
func (c *DeleteCommand) CommandName() string {
	return commandName("delete")
}

// Delete the key
func (c *DeleteCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)

	e, err := s.Delete(c.Key, c.Recursive, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return e, nil
}

// Watch command
type WatchCommand struct {
	Key        string `json:"key"`
	SinceIndex uint64 `json:"sinceIndex"`
	Recursive  bool   `json:"recursive"`
}

// The name of the watch command in the log
func (c *WatchCommand) CommandName() string {
	return commandName("watch")
}

func (c *WatchCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)

	eventChan, err := s.Watch(c.Key, c.Recursive, c.SinceIndex, server.CommitIndex(), server.Term())

	if err != nil {
		return nil, err
	}

	e := <-eventChan

	return e, nil
}

// JoinCommand
type JoinCommand struct {
	RaftVersion string `json:"raftVersion"`
	Name        string `json:"name"`
	RaftURL     string `json:"raftURL"`
	EtcdURL     string `json:"etcdURL"`
}

func newJoinCommand(version, name, raftUrl, etcdUrl string) *JoinCommand {
	return &JoinCommand{
		RaftVersion: version,
		Name:        name,
		RaftURL:     raftUrl,
		EtcdURL:     etcdUrl,
	}
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return commandName("join")
}

// Join a server to the cluster
func (c *JoinCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)
	r, _ := server.Context().(*raftServer)

	// check if the join command is from a previous machine, who lost all its previous log.
	e, _ := s.Get(path.Join("/_etcd/machines", c.Name), false, false, server.CommitIndex(), server.Term())

	b := make([]byte, 8)
	binary.PutUvarint(b, server.CommitIndex())

	if e != nil {
		return b, nil
	}

	// check machine number in the cluster
	num := machineNum()
	if num == maxClusterSize {
		debug("Reject join request from ", c.Name)
		return []byte{0}, etcdErr.NewError(etcdErr.EcodeNoMoreMachine, "", server.CommitIndex(), server.Term())
	}

	addNameToURL(c.Name, c.RaftVersion, c.RaftURL, c.EtcdURL)

	// add peer in raft
	err := server.AddPeer(c.Name, "")

	// add machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)
	value := fmt.Sprintf("raft=%s&etcd=%s&raftVersion=%s", c.RaftURL, c.EtcdURL, c.RaftVersion)
	s.Create(key, value, false, false, store.Permanent, server.CommitIndex(), server.Term())

	// add peer stats
	if c.Name != r.Name() {
		r.followersStats.Followers[c.Name] = &raftFollowerStats{}
		r.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	return b, err
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}

// RemoveCommand
type RemoveCommand struct {
	Name string `json:"name"`
}

// The name of the remove command in the log
func (c *RemoveCommand) CommandName() string {
	return commandName("remove")
}

// Remove a server from the cluster
func (c *RemoveCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)
	r, _ := server.Context().(*raftServer)

	// remove machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)

	_, err := s.Delete(key, false, server.CommitIndex(), server.Term())
	// delete from stats
	delete(r.followersStats.Followers, c.Name)

	if err != nil {
		return []byte{0}, err
	}

	// remove peer in raft
	err = server.RemovePeer(c.Name)

	if err != nil {
		return []byte{0}, err
	}

	if c.Name == server.Name() {
		// the removed node is this node

		// if the node is not replaying the previous logs
		// and the node has sent out a join request in this
		// start. It is sure that this node received a new remove
		// command and need to be removed
		if server.CommitIndex() > r.joinIndex && r.joinIndex != 0 {
			debugf("server [%s] is removed", server.Name())
			os.Exit(0)
		} else {
			// else ignore remove
			debugf("ignore previous remove command.")
		}
	}

	b := make([]byte, 8)
	binary.PutUvarint(b, server.CommitIndex())

	return b, err
}
