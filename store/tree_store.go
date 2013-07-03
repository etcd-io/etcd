package store

import (
	"path"
	"strings"
	"errors"
	//"fmt"
	)

type treeStore struct {
	Root *treeNode
}

type treeNode struct {
	Value string

	Dir bool //for clearity

	NodeMap map[string]*treeNode

	// if the node is a permanent one the ExprieTime will be Unix(0,0)
	// Otherwise after the expireTime, the node will be deleted
	ExpireTime time.Time `json:"expireTime"`

	// a channel to update the expireTime of the node
	update chan time.Time `json:"-"`
}

// set the key to value, return the old value if the key exists 
func (s *treeStore) set(key string, value string, expireTime time.Time, index uint64) (string, error) {
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
			node := &treeNode{".", true, make(map[string]*treeNode)}
			nodeMap[nodes[i]] = node
			nodeMap = node.NodeMap
			continue
		}

		node, ok := nodeMap[nodes[i]]
		// add new dir
		if !ok {
			//fmt.Println("TreeStore: Add a dir ", nodes[i])
			newDir = true
			node := &treeNode{".", true, make(map[string]*treeNode)}
			nodeMap[nodes[i]] = node
			nodeMap = node.NodeMap

		} else if ok && !node.Dir {

			return "", errors.New("Try to add a key under a file")
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
		return "", nil
	} else {
		oldValue := node.Value
		node.Value = value
		//fmt.Println("TreeStore: Update a Node ", key, "=", value, "[", oldValue, "]")
		return oldValue ,nil
	}

}

// get the node of the key
func (s *treeStore) get(key string) *treeNode {
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
			return nil
		}
		nodeMap = node.NodeMap
	}

	node, ok := nodeMap[nodes[i]]
	if ok {
		return node
	}
	return nil

}

// delete the key, return the old value if the key exists
func (s *treeStore) delete(key string) string {
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
			return ""
		}
		nodeMap = node.NodeMap
	}

	node, ok := nodeMap[nodes[i]]
	if ok && !node.Dir{
		oldValue := node.Value
		delete(nodeMap, nodes[i])
		return oldValue
	}
	return ""
}

