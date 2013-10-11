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
	r, _ := server.Context().(*RaftServer)

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
