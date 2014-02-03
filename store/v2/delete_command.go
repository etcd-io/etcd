package v2

import (
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&DeleteCommand{})
}

// The DeleteCommand removes a key from the Store.
type DeleteCommand struct {
	Key		string	`json:"key"`
	Recursive	bool	`json:"recursive"`
	Dir		bool	`json:"dir"`
}

// The name of the delete command in the log
func (c *DeleteCommand) CommandName() string {
	return "etcd:delete"
}

// Delete the key
func (c *DeleteCommand) Apply(context raft.Context) (interface{}, error) {
	s, _ := context.Server().StateMachine().(store.Store)

	if c.Recursive {
		// recursive implies dir
		c.Dir = true
	}

	e, err := s.Delete(c.Key, c.Dir, c.Recursive)

	if err != nil {
		log.Debug(err)
		return nil, err
	}

	return e, nil
}
