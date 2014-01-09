package v2

import (
	"time"

	"github.com/coreos/etcd/store"
	"github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&SyncCommand{})
}

type SyncCommand struct {
	Time time.Time `json:"time"`
}

// The name of the Sync command in the log
func (c SyncCommand) CommandName() string {
	return "etcd:sync"
}

func (c SyncCommand) Apply(context raft.Context) (interface{}, error) {
	s, _ := context.Server().StateMachine().(store.Store)
	s.DeleteExpiredKeys(c.Time)

	return nil, nil
}
