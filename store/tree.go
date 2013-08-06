package store

import (
	"path"
	"sort"
	"strings"
	"time"
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// A file system like tree structure. Each non-leaf node of the tree has a hashmap to
// store its children nodes. Leaf nodes has no hashmap (a nil pointer)
type tree struct {
	Root *treeNode
}

// A treeNode wraps a Node. It has a hashmap to keep records of its children treeNodes.
type treeNode struct {
	InternalNode Node
	Dir          bool
	NodeMap      map[string]*treeNode
}

// TreeNode with its key. We use it when we need to sort the treeNodes.
type tnWithKey struct {
	key string
	tn  *treeNode
}

// Define type and functions to match sort interface
type tnWithKeySlice []tnWithKey

func (s tnWithKeySlice) Len() int           { return len(s) }
func (s tnWithKeySlice) Less(i, j int) bool { return s[i].key < s[j].key }
func (s tnWithKeySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// CONSTANT VARIABLE

// Represent an empty node
var emptyNode = Node{"", PERMANENT, nil}

//------------------------------------------------------------------------------
//
// Methods
//
//------------------------------------------------------------------------------

// Set the key to the given value, return true if success
// If any intermidate path of the key is not a directory type, it will fail
// For example if the /foo = Node(bar) exists, set /foo/foo = Node(barbar)
// will fail.
func (t *tree) set(key string, value Node) bool {

	nodesName := split(key)

	// avoid set value to "/"
	if len(nodesName) == 1 && len(nodesName[0]) == 0 {
		return false
	}

	nodeMap := t.Root.NodeMap

	i := 0
	newDir := false

	// go through all the path
	for i = 0; i < len(nodesName)-1; i++ {

		// if we meet a new directory, all the directory after it must be new
		if newDir {
			tn := &treeNode{emptyNode, true, make(map[string]*treeNode)}
			nodeMap[nodesName[i]] = tn
			nodeMap = tn.NodeMap
			continue
		}

		// get the node from the nodeMap of the current level
		tn, ok := nodeMap[nodesName[i]]

		if !ok {
			// add a new directory and set newDir to true
			newDir = true
			tn := &treeNode{emptyNode, true, make(map[string]*treeNode)}
			nodeMap[nodesName[i]] = tn
			nodeMap = tn.NodeMap

		} else if ok && !tn.Dir {

			// if we meet a non-directory node, we cannot set the key
			return false
		} else {

			// update the nodeMap to next level
			nodeMap = tn.NodeMap
		}

	}

	// Add the last node
	tn, ok := nodeMap[nodesName[i]]

	if !ok {
		// we add a new treeNode
		tn := &treeNode{value, false, nil}
		nodeMap[nodesName[i]] = tn

	} else {
		if tn.Dir {
			return false
		}
		// we change the value of a old Treenode
		tn.InternalNode = value
	}
	return true

}

// Get the tree node of the key
func (t *tree) internalGet(key string) (*treeNode, bool) {
	nodesName := split(key)

	nodeMap := t.Root.NodeMap

	var i int

	for i = 0; i < len(nodesName)-1; i++ {
		node, ok := nodeMap[nodesName[i]]
		if !ok || !node.Dir {
			return nil, false
		}
		nodeMap = node.NodeMap
	}

	tn, ok := nodeMap[nodesName[i]]
	if ok {
		return tn, ok
	} else {
		return nil, ok
	}
}

// get the internalNode of the key
func (t *tree) get(key string) (Node, bool) {
	tn, ok := t.internalGet(key)

	if ok {
		if tn.Dir {
			return emptyNode, false
		}
		return tn.InternalNode, ok
	} else {
		return emptyNode, ok
	}
}

// get the internalNode of the key
func (t *tree) list(directory string) (interface{}, []string, bool) {
	treeNode, ok := t.internalGet(directory)

	if !ok {
		return nil, nil, ok

	} else {
		if !treeNode.Dir {
			return &treeNode.InternalNode, nil, ok
		}
		length := len(treeNode.NodeMap)
		nodes := make([]*Node, length)
		keys := make([]string, length)

		i := 0
		for key, node := range treeNode.NodeMap {
			nodes[i] = &node.InternalNode
			keys[i] = key
			i++
		}

		return nodes, keys, ok
	}
}

// delete the key, return true if success
func (t *tree) delete(key string) bool {
	nodesName := split(key)

	nodeMap := t.Root.NodeMap

	var i int

	for i = 0; i < len(nodesName)-1; i++ {
		node, ok := nodeMap[nodesName[i]]
		if !ok || !node.Dir {
			return false
		}
		nodeMap = node.NodeMap
	}

	node, ok := nodeMap[nodesName[i]]
	if ok && !node.Dir {
		delete(nodeMap, nodesName[i])
		return true
	}
	return false
}

// traverse wrapper
func (t *tree) traverse(f func(string, *Node), sort bool) {
	if sort {
		sortDfs("", t.Root, f)
	} else {
		dfs("", t.Root, f)
	}
}

// clone() will return a deep cloned tree
func (t *tree) clone() *tree {
	newTree := new(tree)
	newTree.Root = &treeNode{
		Node{
			"/",
			time.Unix(0, 0),
			nil,
		},
		true,
		make(map[string]*treeNode),
	}
	recursiveClone(t.Root, newTree.Root)
	return newTree
}

// recursiveClone is a helper function for clone()
func recursiveClone(tnSrc *treeNode, tnDes *treeNode) {
	if !tnSrc.Dir {
		tnDes.InternalNode = tnSrc.InternalNode
		return

	} else {
		tnDes.InternalNode = tnSrc.InternalNode
		tnDes.Dir = true
		tnDes.NodeMap = make(map[string]*treeNode)

		for key, tn := range tnSrc.NodeMap {
			newTn := new(treeNode)
			recursiveClone(tn, newTn)
			tnDes.NodeMap[key] = newTn
		}

	}
}

// deep first search to traverse the tree
// apply the func f to each internal node
func dfs(key string, t *treeNode, f func(string, *Node)) {

	// base case
	if len(t.NodeMap) == 0 {
		f(key, &t.InternalNode)

		// recursion
	} else {
		for tnKey, tn := range t.NodeMap {
			tnKey := key + "/" + tnKey
			dfs(tnKey, tn, f)
		}
	}
}

// sort deep first search to traverse the tree
// apply the func f to each internal node
func sortDfs(key string, t *treeNode, f func(string, *Node)) {
	// base case
	if len(t.NodeMap) == 0 {
		f(key, &t.InternalNode)

		// recursion
	} else {

		s := make(tnWithKeySlice, len(t.NodeMap))
		i := 0

		// copy
		for tnKey, tn := range t.NodeMap {
			tnKey := key + "/" + tnKey
			s[i] = tnWithKey{tnKey, tn}
			i++
		}

		// sort
		sort.Sort(s)

		// traverse
		for i = 0; i < len(t.NodeMap); i++ {
			sortDfs(s[i].key, s[i].tn, f)
		}
	}
}

// split the key by '/', get the intermediate node name
func split(key string) []string {
	key = "/" + key
	key = path.Clean(key)

	// get the intermidate nodes name
	nodesName := strings.Split(key, "/")
	// we do not need the root node, since we start with it
	nodesName = nodesName[1:]
	return nodesName
}
