package store

import (
	"time"
)

// Node is the representation of the internal node with additional fields
// PrevValue is the previous value of the node
// TTL is time to live in second
type Node struct {
	Key           string     `json:"key, omitempty"`
	PrevValue     string     `json:"prevValue,omitempty"`
	Value         string     `json:"value,omitempty"`
	Dir           bool       `json:"dir,omitempty"`
	Expiration    *time.Time `json:"expiration,omitempty"`
	TTL           int64      `json:"ttl,omitempty"`
	Nodes         Nodes      `json:"nodes,omitempty"`
	ModifiedIndex uint64     `json:"modifiedIndex,omitempty"`
	CreatedIndex  uint64     `json:"createdIndex,omitempty"`
}

type Nodes []Node

// interfaces for sorting
func (ns Nodes) Len() int {
	return len(ns)
}

func (ns Nodes) Less(i, j int) bool {
	return ns[i].Key < ns[j].Key
}

func (ns Nodes) Swap(i, j int) {
	ns[i], ns[j] = ns[j], ns[i]
}
