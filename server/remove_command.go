package server

import (
	"encoding/binary"
	"encoding/json"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

func init() {
	raft.RegisterCommand(&RemoveCommandV1{})
	raft.RegisterCommand(&RemoveCommandV2{})
}

// The RemoveCommandV1 removes a server from the cluster.
type RemoveCommandV1 struct {
	Name string `json:"name"`
}

// The name of the remove command in the log
func (c *RemoveCommandV1) CommandName() string {
	return "etcd:remove"
}

// Remove a server from the cluster
func (c *RemoveCommandV1) Apply(context raft.Context) (interface{}, error) {
	c2 := &RemoveCommandV2{Name: c.Name}
	resp, err := applyRemove(c2, context)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 8)
	binary.PutUvarint(b, resp.CommitIndex)
	return b, nil
}

// RemoveCommandV2 represents a command to remove a machine from the server.
type RemoveCommandV2 struct {
	Name string `json:"name"`
}

// CommandName returns the name of the command.
func (c *RemoveCommandV2) CommandName() string {
	return "etcd:v2:remove"
}

// Apply removes the given machine from the cluster.
func (c *RemoveCommandV2) Apply(context raft.Context) (interface{}, error) {
	resp, err := applyRemove(c, context)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(resp)
	return b, nil
}

type removeResponseV2 struct {
	CommitIndex uint64 `json:"commitIndex"`
}

// applyRemove removes the given machine from the cluster.
func applyRemove(c *RemoveCommandV2, context raft.Context) (*removeResponseV2, error) {
	ps, _ := context.Server().Context().(*PeerServer)
	msg := &removeResponseV2{CommitIndex: context.CommitIndex()}

	// Remove node from the shared registry.
	err := ps.registry.Unregister(c.Name)

	// Delete from stats
	delete(ps.followersStats.Followers, c.Name)

	if err != nil {
		log.Debugf("Error while unregistering: %s (%v)", c.Name, err)
		return nil, err
	}

	// Remove peer in raft
	if err := context.Server().RemovePeer(c.Name); err != nil {
		log.Debugf("Unable to remove peer: %s (%v)", c.Name, err)
		return nil, err
	}

	if c.Name == context.Server().Name() {
		// the removed node is this node

		// if the node is not replaying the previous logs
		// and the node has sent out a join request in this
		// start. It is sure that this node received a new remove
		// command and need to be removed
		if context.CommitIndex() > ps.joinIndex && ps.joinIndex != 0 {
			log.Debugf("server [%s] is removed", context.Server().Name())
			ps.AsyncRemove()
		} else {
			// else ignore remove
			log.Debugf("ignore previous remove command.")
		}
	}
	return msg, nil
}
