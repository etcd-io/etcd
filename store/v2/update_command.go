package v2

import (
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&UpdateCommand{})
}

// Update command
type UpdateCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
}

// The name of the update command in the log
func (c *UpdateCommand) CommandName() string {
	return "etcd:update"
}

// Create node
func (c *UpdateCommand) Apply(cxt raft.Context) (interface{}, error) {
	server := cxt.Server()

	s, _ := server.StateMachine().(store.Store)

	e, err := s.Update(c.Key, c.Value, c.ExpireTime)

	if err != nil {
		log.Debug(err)
		return nil, err
	}

	return e, nil
}
