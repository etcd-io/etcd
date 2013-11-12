package v2

import (
	"time"

	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
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

func (c SyncCommand) Apply(server raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(store.Store)
	s.DeleteExpiredKeys(c.Time)

	return nil, nil
}
