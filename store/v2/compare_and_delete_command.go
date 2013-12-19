package v2

import (
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&CompareAndDeleteCommand{})
}

// The CompareAndDelete performs a conditional delete on a key in the store.
type CompareAndDeleteCommand struct {
	Key       string `json:"key"`
	PrevValue string `json:"prevValue"`
	PrevIndex uint64 `json:"prevIndex"`
	Recursive bool   `json:"recursive"`
}

// The name of the compareAndDelete command in the log
func (c *CompareAndDeleteCommand) CommandName() string {
	return "etcd:compareAndDelete"
}

// Set the key-value pair if the current value of the key equals to the given prevValue
func (c *CompareAndDeleteCommand) Apply(server raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(store.Store)

	e, err := s.CompareAndDelete(c.Key, c.Recursive, c.PrevValue, c.PrevIndex)

	if err != nil {
		log.Debug(err)
		return nil, err
	}

	return e, nil
}
