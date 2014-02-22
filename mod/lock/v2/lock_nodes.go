package v2

import (
	"path"
	"sort"
	"strconv"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

// lockNodes is a wrapper for go-etcd's Nodes to allow for sorting by numeric key.
type lockNodes struct {
	etcd.Nodes
}

// Less sorts the nodes by key (numerically).
func (s lockNodes) Less(i, j int) bool {
	a, _ := strconv.Atoi(path.Base(s.Nodes[i].Key))
	b, _ := strconv.Atoi(path.Base(s.Nodes[j].Key))
	return a < b
}

// Retrieves the first node in the set of lock nodes.
func (s lockNodes) First() *etcd.Node {
	sort.Sort(s)
	if len(s.Nodes) > 0 {
		return &s.Nodes[0]
	}
	return nil
}

// Retrieves the first node with a given value.
func (s lockNodes) FindByValue(value string) (*etcd.Node, int) {
	sort.Sort(s)

	for i, node := range s.Nodes {
		if node.Value == value {
			return &node, i
		}
	}
	return nil, 0
}

// Find the node with the largest index in the lockNodes that is smaller than the given index. Also return the lastModified index of that node.
func (s lockNodes) PrevIndex(index int) (int, int) {
	sort.Sort(s)

	// Iterate over each node to find the given index. We keep track of the
	// previous index on each iteration so we can return it when we match the
	// index we're looking for.
	var prevIndex int
	for _, node := range s.Nodes {
		idx, _ := strconv.Atoi(path.Base(node.Key))
		if index == idx {
			return prevIndex, int(node.ModifiedIndex)
		}
		prevIndex = idx
	}
	return 0, 0
}
