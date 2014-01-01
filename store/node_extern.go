package store

import (
	"time"
)

// NodeExtern is the external representation of the
// internal node with additional fields
// PrevValue is the previous value of the node
// TTL is time to live in second
type NodeExtern struct {
	Key           string      `json:"key, omitempty"`
	Value         string      `json:"value,omitempty"`
	Dir           bool        `json:"dir,omitempty"`
	Expiration    *time.Time  `json:"expiration,omitempty"`
	TTL           int64       `json:"ttl,omitempty"`
	Nodes         NodeExterns `json:"nodes,omitempty"`
	ModifiedIndex uint64      `json:"modifiedIndex,omitempty"`
	CreatedIndex  uint64      `json:"createdIndex,omitempty"`
}

type NodeExterns []*NodeExtern

// interfaces for sorting
func (ns NodeExterns) Len() int {
	return len(ns)
}

func (ns NodeExterns) Less(i, j int) bool {
	return ns[i].Key < ns[j].Key
}

func (ns NodeExterns) Swap(i, j int) {
	ns[i], ns[j] = ns[j], ns[i]
}
