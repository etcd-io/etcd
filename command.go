package main

import (
	"encoding/binary"
	"encoding/json"
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
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
}

// The name of the create command in the log
func (c *CreateCommand) CommandName() string {
	return commandName("create")
}

// Create node
func (c *CreateCommand) Apply(server *raft.Server) (interface{}, error) {
	e, err := etcdStore.Create(c.Key, c.Value, c.ExpireTime, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return json.Marshal(e)
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
	e, err := etcdStore.Update(c.Key, c.Value, c.ExpireTime, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return json.Marshal(e)
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
	e, err := etcdStore.TestAndSet(c.Key, c.PrevValue, c.PrevIndex,
		c.Value, c.ExpireTime, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return json.Marshal(e)
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
	e, err := etcdStore.Get(c.Key, c.Recursive, c.Sorted, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return json.Marshal(e)
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
	e, err := etcdStore.Delete(c.Key, c.Recursive, server.CommitIndex(), server.Term())

	if err != nil {
		debug(err)
		return nil, err
	}

	return json.Marshal(e)
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
	eventChan, err := etcdStore.Watch(c.Key, c.Recursive, c.SinceIndex, server.CommitIndex(), server.Term())

	if err != nil {
		return nil, err
	}

	e := <-eventChan

	return json.Marshal(e)
}

// JoinCommand
type JoinCommand struct {
	RaftVersion string `json:"raftVersion"`
	Name        string `json:"name"`
	RaftURL     string `json:"raftURL"`
	EtcdURL     string `json:"etcdURL"`
}

func newJoinCommand() *JoinCommand {
	return &JoinCommand{
		RaftVersion: r.version,
		Name:        r.name,
		RaftURL:     r.url,
		EtcdURL:     e.url,
	}
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return commandName("join")
}

// Join a server to the cluster
func (c *JoinCommand) Apply(raftServer *raft.Server) (interface{}, error) {

	// check if the join command is from a previous machine, who lost all its previous log.
	e, _ := etcdStore.Get(path.Join("/_etcd/machines", c.Name), false, false, raftServer.CommitIndex(), raftServer.Term())

	b := make([]byte, 8)
	binary.PutUvarint(b, raftServer.CommitIndex())

	if e != nil {
		return b, nil
	}

	// check machine number in the cluster
	num := machineNum()
	if num == maxClusterSize {
		debug("Reject join request from ", c.Name)
		return []byte{0}, etcdErr.NewError(etcdErr.EcodeNoMoreMachine, "")
	}

	addNameToURL(c.Name, c.RaftVersion, c.RaftURL, c.EtcdURL)

	// add peer in raft
	err := raftServer.AddPeer(c.Name, "")

	// add machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)
	value := fmt.Sprintf("raft=%s&etcd=%s&raftVersion=%s", c.RaftURL, c.EtcdURL, c.RaftVersion)
	etcdStore.Create(key, value, store.Permanent, raftServer.CommitIndex(), raftServer.Term())

	if c.Name != r.Name() { // do not add self to the peer list
		r.peersStats[c.Name] = &raftPeerStats{MinLatency: 1 << 63}
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
func (c *RemoveCommand) Apply(raftServer *raft.Server) (interface{}, error) {

	// remove machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)

	_, err := etcdStore.Delete(key, false, raftServer.CommitIndex(), raftServer.Term())
	delete(r.peersStats, c.Name)

	if err != nil {
		return []byte{0}, err
	}

	// remove peer in raft
	err = raftServer.RemovePeer(c.Name)

	if err != nil {
		return []byte{0}, err
	}

	if c.Name == raftServer.Name() {
		// the removed node is this node

		// if the node is not replaying the previous logs
		// and the node has sent out a join request in this
		// start. It is sure that this node received a new remove
		// command and need to be removed
		if raftServer.CommitIndex() > r.joinIndex && r.joinIndex != 0 {
			debugf("server [%s] is removed", raftServer.Name())
			os.Exit(0)
		} else {
			// else ignore remove
			debugf("ignore previous remove command.")
		}
	}

	b := make([]byte, 8)
	binary.PutUvarint(b, raftServer.CommitIndex())

	return b, err
}
