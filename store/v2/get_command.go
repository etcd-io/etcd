package v2

import (
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

func init() {
	raft.RegisterCommand(&GetCommand{})
}

// The GetCommand gets a key from the Store.
type GetCommand struct {
	Key       string `json:"key"`
	Recursive bool   `json:"recursive"`
	Sorted    bool   `json:sorted`
}

// The name of the get command in the log
func (c *GetCommand) CommandName() string {
	return "etcd:get"
}

// Get the key
func (c *GetCommand) Apply(context raft.Context) (interface{}, error) {
	s, _ := context.Server().StateMachine().(store.Store)
	e, err := s.Get(c.Key, c.Recursive, c.Sorted)

	if err != nil {
		log.Debug(err)
		return nil, err
	}
	return e, nil
}
