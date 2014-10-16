package store

import (
	"sort"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/jonboulle/clockwork"
)

// NodeExtern is the external representation of the
// internal node with additional fields
// PrevValue is the previous value of the node
// TTL is time to live in second
type NodeExtern struct {
	Key           string      `json:"key,omitempty"`
	Value         *string     `json:"value,omitempty"`
	Dir           bool        `json:"dir,omitempty"`
	Expiration    *time.Time  `json:"expiration,omitempty"`
	TTL           int64       `json:"ttl,omitempty"`
	Nodes         NodeExterns `json:"nodes,omitempty"`
	ModifiedIndex uint64      `json:"modifiedIndex,omitempty"`
	CreatedIndex  uint64      `json:"createdIndex,omitempty"`
}

func (eNode *NodeExtern) loadInternalNode(n *node, recursive, sorted bool, clock clockwork.Clock) {
	if n.IsDir() { // node is a directory
		eNode.Dir = true

		children, _ := n.List()
		eNode.Nodes = make(NodeExterns, len(children))

		// we do not use the index in the children slice directly
		// we need to skip the hidden one
		i := 0

		for _, child := range children {
			if child.IsHidden() { // get will not return hidden nodes
				continue
			}

			eNode.Nodes[i] = child.Repr(recursive, sorted, clock)
			i++
		}

		// eliminate hidden nodes
		eNode.Nodes = eNode.Nodes[:i]

		if sorted {
			sort.Sort(eNode.Nodes)
		}

	} else { // node is a file
		value, _ := n.Read()
		eNode.Value = &value
	}

	eNode.Expiration, eNode.TTL = n.expirationAndTTL(clock)
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
