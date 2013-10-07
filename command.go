/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// Set command
type SetCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
}

// The name of the set command in the log
func (c *SetCommand) CommandName() string {
	return commandName("set")
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
	return commandName("testAndSet")
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
	return commandName("get")
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
	return commandName("delete")
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
	return commandName("watch")
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
	response, _ := etcdStore.RawGet(path.Join("_etcd/machines", c.Name))

	b := make([]byte, 8)
	binary.PutUvarint(b, raftServer.CommitIndex())

	if response != nil {
		return b, nil
	}

	// check machine number in the cluster
	num := machineNum()
	if num == maxClusterSize {
		debug("Reject join request from ", c.Name)
		return []byte{0}, etcdErr.NewError(103, "")
	}

	addNameToURL(c.Name, c.RaftVersion, c.RaftURL, c.EtcdURL)

	// add peer in raft
	err := raftServer.AddPeer(c.Name, "")

	// add machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)
	value := fmt.Sprintf("raft=%s&etcd=%s&raftVersion=%s", c.RaftURL, c.EtcdURL, c.RaftVersion)
	etcdStore.Set(key, value, time.Unix(0, 0), raftServer.CommitIndex())

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
func (c *RemoveCommand) Apply(raftServer *raft.Server) (interface{}, error) {

	// remove machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)

	_, err := etcdStore.Delete(key, raftServer.CommitIndex())

	// delete from stats
	delete(r.followersStats.Followers, c.Name)

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
