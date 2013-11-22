package v2

import (
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&DeleteCommand{})
}

// The DeleteCommand removes a key from the Store.
type DeleteCommand struct {
	Key       string `json:"key"`
	Recursive bool   `json:"recursive"`
}

// The name of the delete command in the log
func (c *DeleteCommand) CommandName() string {
	return "etcd:delete"
}

// Delete the key
func (c *DeleteCommand) Apply(server raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(store.Store)

	e, err := s.Delete(c.Key, c.Recursive)

	if err != nil {
		log.Debug(err)
		return nil, err
	}

	return e, nil
}
