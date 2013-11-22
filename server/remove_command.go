package server

import (
	"encoding/binary"
	"os"

	"github.com/coreos/etcd/log"
	"github.com/coreos/raft"
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
func (c *RemoveCommand) Apply(server raft.Server) (interface{}, error) {
	ps, _ := server.Context().(*PeerServer)

	// Remove node from the shared registry.
	err := ps.registry.Unregister(c.Name)

	// Delete from stats
	delete(ps.followersStats.Followers, c.Name)

	if err != nil {
		log.Debugf("Error while unregistering: %s (%v)", c.Name, err)
		return []byte{0}, err
	}

	// Remove peer in raft
	err = server.RemovePeer(c.Name)
	if err != nil {
		log.Debugf("Unable to remove peer: %s (%v)", c.Name, err)
		return []byte{0}, err
	}

	if c.Name == server.Name() {
		// the removed node is this node

		// if the node is not replaying the previous logs
		// and the node has sent out a join request in this
		// start. It is sure that this node received a new remove
		// command and need to be removed
		if server.CommitIndex() > ps.joinIndex && ps.joinIndex != 0 {
			log.Debugf("server [%s] is removed", server.Name())
			os.Exit(0)
		} else {
			// else ignore remove
			log.Debugf("ignore previous remove command.")
		}
	}

	b := make([]byte, 8)
	binary.PutUvarint(b, server.CommitIndex())

	return b, err
}
