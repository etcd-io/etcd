package v2

import (
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&CompareAndSwapCommand{})
}

// The CompareAndSwap performs a conditional update on a key in the store.
type CompareAndSwapCommand struct {
	Key		string		`json:"key"`
	Value		string		`json:"value"`
	ExpireTime	time.Time	`json:"expireTime"`
	PrevValue	string		`json:"prevValue"`
	PrevIndex	uint64		`json:"prevIndex"`
}

// The name of the testAndSet command in the log
func (c *CompareAndSwapCommand) CommandName() string {
	return "etcd:compareAndSwap"
}

// Set the key-value pair if the current value of the key equals to the given prevValue
func (c *CompareAndSwapCommand) Apply(context raft.Context) (interface{}, error) {
	s, _ := context.Server().StateMachine().(store.Store)

	e, err := s.CompareAndSwap(c.Key, c.PrevValue, c.PrevIndex, c.Value, c.ExpireTime)

	if err != nil {
		log.Debug(err)
		return nil, err
	}

	return e, nil
}
