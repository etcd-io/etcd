package store

import (
	"path"
	"strings"
	)

type tree struct {
	Root *treeNode
}

type treeNode struct {

	Value Node

	Dir bool //for clearity

	NodeMap map[string]*treeNode

}

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

func (t *tree) traverse(f func(*treeNode)) {
	dfs(t.Root, f)
}

func dfs(t *treeNode, f func(*treeNode)) {
	if len(t.NodeMap) == 0{
		f(t)
	} else {
		for _, _treeNode := range t.NodeMap {
			dfs(_treeNode, f)
		}
	}
}


