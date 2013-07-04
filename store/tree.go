package store

import (
	"path"
	"strings"
	"sort"
	)

type tree struct {
	Root *treeNode
}

type treeNode struct {

	Value Node

	Dir bool //for clearity

	NodeMap map[string]*treeNode

}

type tnWithKey struct{
	key string
	tn  *treeNode
}

type tnWithKeySlice []tnWithKey

func (s tnWithKeySlice) Len() int           { return len(s) }
func (s tnWithKeySlice) Less(i, j int) bool { return s[i].key < s[j].key }
func (s tnWithKeySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }



var emptyNode = Node{".", PERMANENT, nil}

// set the key to value, return the old value if the key exists 
func (s *tree) set(key string, value Node) bool {
	key = "/" + key
	key = path.Clean(key)

	nodes := strings.Split(key, "/")
	nodes = nodes[1:]

	//fmt.Println("TreeStore: Nodes ", nodes, " length: ", len(nodes))

	nodeMap := s.Root.NodeMap

	i := 0
	newDir := false

	for i = 0; i < len(nodes) - 1; i++ {

		if newDir {
			node := &treeNode{emptyNode, true, make(map[string]*treeNode)}
			nodeMap[nodes[i]] = node
			nodeMap = node.NodeMap
			continue
		}

		node, ok := nodeMap[nodes[i]]
		// add new dir
		if !ok {
			//fmt.Println("TreeStore: Add a dir ", nodes[i])
			newDir = true
			node := &treeNode{emptyNode, true, make(map[string]*treeNode)}
			nodeMap[nodes[i]] = node
			nodeMap = node.NodeMap

		} else if ok && !node.Dir {

			return false
		} else {

			//fmt.Println("TreeStore: found dir ", nodes[i])
			nodeMap = node.NodeMap
		}

	}

	// add the last node and value
	node, ok := nodeMap[nodes[i]]

	if !ok {
		node := &treeNode{value, false, nil}
		nodeMap[nodes[i]] = node
		//fmt.Println("TreeStore: Add a new Node ", key, "=", value)
	} else {
		node.Value = value
		//fmt.Println("TreeStore: Update a Node ", key, "=", value, "[", oldValue, "]")
	}
	return true

}

// get the node of the key
func (s *tree) get(key string) (Node, bool) {
	key = "/" + key
	key = path.Clean(key)

	nodes := strings.Split(key, "/")
	nodes = nodes[1:]

	//fmt.Println("TreeStore: Nodes ", nodes, " length: ", len(nodes))

	nodeMap := s.Root.NodeMap
		
	var i int

	for i = 0; i < len(nodes) - 1; i++ {
		node, ok := nodeMap[nodes[i]]
		if !ok || !node.Dir {
			return emptyNode, false
		}
		nodeMap = node.NodeMap
	}

	treeNode, ok := nodeMap[nodes[i]]
	if ok {
		return treeNode.Value, ok
	} else {
		return emptyNode, ok
	}

}

// delete the key, return the old value if the key exists
func (s *tree) delete(key string) bool {
	key = "/" + key
	key = path.Clean(key)

	nodes := strings.Split(key, "/")
	nodes = nodes[1:]

	//fmt.Println("TreeStore: Nodes ", nodes, " length: ", len(nodes))

	nodeMap := s.Root.NodeMap
		
	var i int

	for i = 0; i < len(nodes) - 1; i++ {
		node, ok := nodeMap[nodes[i]]
		if !ok || !node.Dir {
			return false
		}
		nodeMap = node.NodeMap
	}

	node, ok := nodeMap[nodes[i]]
	if ok && !node.Dir{
		delete(nodeMap, nodes[i])
		return true
	}
	return false
}

func (t *tree) traverse(f func(string, *Node), sort bool) {
	if sort {
		sortDfs("", t.Root, f)
	} else {
		dfs("", t.Root, f)	
	}
}

func dfs(key string, t *treeNode, f func(string, *Node)) {
	// base case
	if len(t.NodeMap) == 0{
		f(key, &t.Value)

	// recursion
	} else {
		for nodeKey, _treeNode := range t.NodeMap {
			newKey := key + "/" + nodeKey
			dfs(newKey, _treeNode, f)
		}
	}
}

func sortDfs(key string, t *treeNode, f func(string, *Node)) {
	// base case
	if len(t.NodeMap) == 0{
		f(key, &t.Value)

	// recursion
	} else {

		s := make(tnWithKeySlice, len(t.NodeMap))
		i := 0

		// copy
		for nodeKey, _treeNode := range t.NodeMap {
			newKey := key + "/" + nodeKey
			s[i] = tnWithKey{newKey, _treeNode}
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


