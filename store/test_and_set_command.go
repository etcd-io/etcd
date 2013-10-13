package store

import (
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/go-raft"
)

func init() {
	raft.RegisterCommand(&TestAndSetCommand{})
}

// The TestAndSetCommand performs a conditional update on a key in the store.
type TestAndSetCommand struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	ExpireTime time.Time `json:"expireTime"`
	PrevValue  string    `json: prevValue`
	PrevIndex  uint64    `json: prevIndex`
}

// The name of the testAndSet command in the log
func (c *TestAndSetCommand) CommandName() string {
	return "etcd:testAndSet"
}

// Set the key-value pair if the current value of the key equals to the given prevValue
func (c *TestAndSetCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*Store)

	e, err := s.TestAndSet(c.Key, c.PrevValue, c.PrevIndex,
		c.Value, c.ExpireTime, server.CommitIndex(), server.Term())

	if err != nil {
		log.Debug(err)
		return nil, err
	}

	return e, nil
}
