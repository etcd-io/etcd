package server

import (
	"encoding/binary"
	"path"

	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

func init() {
	raft.RegisterCommand(&RemoveCommand{})
}

// The RemoveCommand removes a server from the cluster.
type RemoveCommand struct {
	Name string `json:"name"`
}

// The name of the remove command in the log
func (c *RemoveCommand) CommandName() string {
	return "etcd:remove"
}

// Remove a server from the cluster
func (c *RemoveCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)
	ps, _ := server.Context().(*PeerServer)

	// Remove node from the shared registry.
	err := ps.registry.Unregister(c.Name, server.CommitIndex(), server.Term())

	// Delete from stats
	delete(ps.followersStats.Followers, c.Name)

	if err != nil {
		return []byte{0}, err
	}

	// Remove peer in raft
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
		if server.CommitIndex() > ps.joinIndex && ps.joinIndex != 0 {
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
