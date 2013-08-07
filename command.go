package main

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"path"
	"time"
)

// A command represents an action to be taken on the replicated state machine.
type Command interface {
	CommandName() string
	Apply(server *raft.Server) (interface{}, error)
}

// Set command
type SetCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
}

// The name of the set command in the log
func (c *SetCommand) CommandName() string {
	return "etcd:set"
}

// Set the key-value pair
func (c *SetCommand) Apply(server *raft.Server) (interface{}, error) {
	return etcdStore.Set(c.Key, c.Value, c.ExpireTime, server.CommitIndex())
}

// TestAndSet command
type TestAndSetCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	PrevValue  string    `json: prevValue`
	ExpireTime time.Time `json:"expireTime"`
}

// The name of the testAndSet command in the log
func (c *TestAndSetCommand) CommandName() string {
	return "testAndSet"
}

// Set the key-value pair if the current value of the key equals to the given prevValue
func (c *TestAndSetCommand) Apply(server *raft.Server) (interface{}, error) {
	return etcdStore.TestAndSet(c.Key, c.PrevValue, c.Value, c.ExpireTime, server.CommitIndex())
}

// Get command
type GetCommand struct {
	Key string `json:"key"`
}

// The name of the get command in the log
func (c *GetCommand) CommandName() string {
	return "etcd:get"
}

// Get the value of key
func (c *GetCommand) Apply(server *raft.Server) (interface{}, error) {
	return etcdStore.Get(c.Key)
}

// Delete command
type DeleteCommand struct {
	Key string `json:"key"`
}

// The name of the delete command in the log
func (c *DeleteCommand) CommandName() string {
	return "etcd:delete"
}

// Delete the key
func (c *DeleteCommand) Apply(server *raft.Server) (interface{}, error) {
	return etcdStore.Delete(c.Key, server.CommitIndex())
}

// Watch command
type WatchCommand struct {
	Key        string `json:"key"`
	SinceIndex uint64 `json:"sinceIndex"`
}

// The name of the watch command in the log
func (c *WatchCommand) CommandName() string {
	return "etcd:watch"
}

func (c *WatchCommand) Apply(server *raft.Server) (interface{}, error) {
	// create a new watcher
	watcher := store.NewWatcher()

	// add to the watchers list
	etcdStore.AddWatcher(c.Key, watcher, c.SinceIndex)

	// wait for the notification for any changing
	res := <-watcher.C

	if res == nil {
		return nil, fmt.Errorf("Clearing watch")
	}

	return json.Marshal(res)
}

// JoinCommand
type JoinCommand struct {
	Name       string `json:"name"`
	Hostname   string `json:"hostName"`
	RaftPort   int    `json:"raftPort"`
	ClientPort int    `json:"clientPort"`
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return "etcd:join"
}

// Join a server to the cluster
func (c *JoinCommand) Apply(raftServer *raft.Server) (interface{}, error) {

	// check if the join command is from a previous machine, who lost all its previous log.
	response, _ := etcdStore.RawGet(path.Join("_etcd/machines", c.Name))

	if response != nil {
		return []byte("join success"), nil
	}

	// check machine number in the cluster
	num := machineNum()
	if num == maxClusterSize {
		return []byte("join fail"), fmt.Errorf(errors[103])
	}

	// add peer in raft
	err := raftServer.AddPeer(c.Name)

	// add machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)
	value := fmt.Sprintf("%s,%d,%d", c.Hostname, c.RaftPort, c.ClientPort)
	etcdStore.Set(key, value, time.Unix(0, 0), raftServer.CommitIndex())

	return []byte("join success"), err
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}
