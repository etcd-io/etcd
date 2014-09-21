package client

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnavailable = errors.New("client: no available etcd endpoints")
	ErrNoLeader    = errors.New("client: no leader")
	ErrKeyNoExist  = errors.New("client: key does not exist")
	ErrKeyExists   = errors.New("client: key already exists")
)

type Client interface {
	Create(key, value string, ttl time.Duration) (*Response, error)
	Get(key string) (*Response, error)
	Watch(key string) Watcher
	RecursiveWatch(key string) Watcher
}

type Watcher interface {
	Next() (*Response, error)
}

type Response struct {
	Action   string `json:"action"`
	Node     *Node  `json:"node"`
	PrevNode *Node  `json:"prevNode"`
}

type Nodes []Node
type Node struct {
	Key           string `json:"key"`
	Value         string `json:"value"`
	Nodes         Nodes  `json:"nodes"`
	ModifiedIndex uint64 `json:"modifiedIndex"`
	CreatedIndex  uint64 `json:"createdIndex"`
}

func (n *Node) String() string {
	return fmt.Sprintf("{Key: %s, CreatedIndex: %d, ModifiedIndex: %d}", n.Key, n.CreatedIndex, n.ModifiedIndex)
}
