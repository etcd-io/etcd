package store

import (
	"path"
	"sort"
	"sync"
	"time"

	etcdErr "github.com/coreos/etcd/error"
)

var (
	Permanent time.Time
)

const (
	normal = iota
	removed
)

// Node is the basic element in the store system.
// A key-value pair will have a string value
// A directory will have a children map
type Node struct {
	Path string

	CreateIndex   uint64
	CreateTerm    uint64
	ModifiedIndex uint64
	ModifiedTerm  uint64

	Parent *Node `json:"-"` // should not encode this field! avoid cyclical dependency.

	ExpireTime time.Time
	ACL        string
	Value      string           // for key-value pair
	Children   map[string]*Node // for directory

	// a ttl node will have an expire routine associated with it.
	// we need a channel to stop that routine when the expiration changes.
	stopExpire chan bool

	// ensure we only delete the node once
	// expire and remove may try to delete a node twice
	once sync.Once
}

// newKV creates a Key-Value pair
func newKV(nodePath string, value string, createIndex uint64,
	createTerm uint64, parent *Node, ACL string, expireTime time.Time) *Node {

	return &Node{
		Path:          nodePath,
		CreateIndex:   createIndex,
		CreateTerm:    createTerm,
		ModifiedIndex: createIndex,
		ModifiedTerm:  createTerm,
		Parent:        parent,
		ACL:           ACL,
		stopExpire:    make(chan bool, 1),
		ExpireTime:    expireTime,
		Value:         value,
	}
}

// newDir creates a directory
func newDir(nodePath string, createIndex uint64, createTerm uint64,
	parent *Node, ACL string, expireTime time.Time) *Node {

	return &Node{
		Path:        nodePath,
		CreateIndex: createIndex,
		CreateTerm:  createTerm,
		Parent:      parent,
		ACL:         ACL,
		stopExpire:  make(chan bool, 1),
		ExpireTime:  expireTime,
		Children:    make(map[string]*Node),
	}
}

// IsHidden function checks if the node is a hidden node. A hidden node
// will begin with '_'
// A hidden node will not be shown via get command under a directory
// For example if we have /foo/_hidden and /foo/notHidden, get "/foo"
// will only return /foo/notHidden
func (n *Node) IsHidden() bool {
	_, name := path.Split(n.Path)

	return name[0] == '_'
}

// IsPermanent function checks if the node is a permanent one.
func (n *Node) IsPermanent() bool {
	return n.ExpireTime.Sub(Permanent) == 0
}

// IsExpired function checks if the node has been expired.
func (n *Node) IsExpired() (bool, time.Duration) {
	if n.IsPermanent() {
		return false, 0
	}

	duration := n.ExpireTime.Sub(time.Now())
	if duration <= 0 {
		return true, 0
	}

	return false, duration
}

// IsDir function checks whether the node is a directory.
// If the node is a directory, the function will return true.
// Otherwise the function will return false.
func (n *Node) IsDir() bool {
	return !(n.Children == nil)
}

// Read function gets the value of the node.
// If the receiver node is not a key-value pair, a "Not A File" error will be returned.
func (n *Node) Read() (string, *etcdErr.Error) {
	if n.IsDir() {
		return "", etcdErr.NewError(etcdErr.EcodeNotFile, "", UndefIndex, UndefTerm)
	}

	return n.Value, nil
}

// Write function set the value of the node to the given value.
// If the receiver node is a directory, a "Not A File" error will be returned.
func (n *Node) Write(value string, index uint64, term uint64) *etcdErr.Error {
	if n.IsDir() {
		return etcdErr.NewError(etcdErr.EcodeNotFile, "", UndefIndex, UndefTerm)
	}

	n.Value = value
	n.ModifiedIndex = index
	n.ModifiedTerm = term

	return nil
}

func (n *Node) ExpirationAndTTL() (*time.Time, int64) {
	if n.ExpireTime.Sub(Permanent) != 0 {
		return &n.ExpireTime, int64(n.ExpireTime.Sub(time.Now())/time.Second) + 1
	}
	return nil, 0
}

// List function return a slice of nodes under the receiver node.
// If the receiver node is not a directory, a "Not A Directory" error will be returned.
func (n *Node) List() ([]*Node, *etcdErr.Error) {
	if !n.IsDir() {
		return nil, etcdErr.NewError(etcdErr.EcodeNotDir, "", UndefIndex, UndefTerm)
	}

	nodes := make([]*Node, len(n.Children))

	i := 0
	for _, node := range n.Children {
		nodes[i] = node
		i++
	}

	return nodes, nil
}

// GetChild function returns the child node under the directory node.
// On success, it returns the file node
func (n *Node) GetChild(name string) (*Node, *etcdErr.Error) {
	if !n.IsDir() {
		return nil, etcdErr.NewError(etcdErr.EcodeNotDir, n.Path, UndefIndex, UndefTerm)
	}

	child, ok := n.Children[name]

	if ok {
		return child, nil
	}

	return nil, nil
}

// Add function adds a node to the receiver node.
// If the receiver is not a directory, a "Not A Directory" error will be returned.
// If there is a existing node with the same name under the directory, a "Already Exist"
// error will be returned
func (n *Node) Add(child *Node) *etcdErr.Error {
	if !n.IsDir() {
		return etcdErr.NewError(etcdErr.EcodeNotDir, "", UndefIndex, UndefTerm)
	}

	_, name := path.Split(child.Path)

	_, ok := n.Children[name]

	if ok {
		return etcdErr.NewError(etcdErr.EcodeNodeExist, "", UndefIndex, UndefTerm)
	}

	n.Children[name] = child

	return nil
}

// Remove function remove the node.
func (n *Node) Remove(recursive bool, callback func(path string)) *etcdErr.Error {

	if n.IsDir() && !recursive {
		// cannot delete a directory without set recursive to true
		return etcdErr.NewError(etcdErr.EcodeNotFile, "", UndefIndex, UndefTerm)
	}

	onceBody := func() {
		n.internalRemove(recursive, callback)
	}

	// this function might be entered multiple times by expire and delete
	// every node will only be deleted once.
	n.once.Do(onceBody)

	return nil
}

// internalRemove function will be called by remove()
func (n *Node) internalRemove(recursive bool, callback func(path string)) {
	if !n.IsDir() { // key-value pair
		_, name := path.Split(n.Path)

		// find its parent and remove the node from the map
		if n.Parent != nil && n.Parent.Children[name] == n {
			delete(n.Parent.Children, name)
		}

		if callback != nil {
			callback(n.Path)
		}

		// the stop channel has a buffer. just send to it!
		n.stopExpire <- true
		return
	}

	for _, child := range n.Children { // delete all children
		child.Remove(true, callback)
	}

	// delete self
	_, name := path.Split(n.Path)
	if n.Parent != nil && n.Parent.Children[name] == n {
		delete(n.Parent.Children, name)

		if callback != nil {
			callback(n.Path)
		}

		n.stopExpire <- true
	}
}

// Expire function will test if the node is expired.
// if the node is already expired, delete the node and return.
// if the node is permanent (this shouldn't happen), return at once.
// else wait for a period time, then remove the node. and notify the watchhub.
func (n *Node) Expire(s *Store) {
	expired, duration := n.IsExpired()

	if expired { // has been expired
		// since the parent function of Expire() runs serially,
		// there is no need for lock here
		e := newEvent(Expire, n.Path, UndefIndex, UndefTerm)
		s.WatcherHub.notify(e)

		n.Remove(true, nil)
		s.Stats.Inc(ExpireCount)

		return
	}

	if duration == 0 { // Permanent Node
		return
	}

	go func() { // do monitoring
		select {
		// if timeout, delete the node
		case <-time.After(duration):

			// before expire get the lock, the expiration time
			// of the node may be updated.
			// we have to check again when get the lock
			s.worldLock.Lock()
			defer s.worldLock.Unlock()

			expired, _ := n.IsExpired()

			if expired {
				e := newEvent(Expire, n.Path, UndefIndex, UndefTerm)
				s.WatcherHub.notify(e)

				n.Remove(true, nil)
				s.Stats.Inc(ExpireCount)
			}

			return

		// if stopped, return
		case <-n.stopExpire:
			return
		}
	}()
}

func (n *Node) Pair(recurisive, sorted bool) KeyValuePair {
	if n.IsDir() {
		pair := KeyValuePair{
			Key: n.Path,
			Dir: true,
		}

		if !recurisive {
			return pair
		}

		children, _ := n.List()
		pair.KVPairs = make([]KeyValuePair, len(children))

		// we do not use the index in the children slice directly
		// we need to skip the hidden one
		i := 0

		for _, child := range children {

			if child.IsHidden() { // get will not list hidden node
				continue
			}

			pair.KVPairs[i] = child.Pair(recurisive, sorted)

			i++
		}

		// eliminate hidden nodes
		pair.KVPairs = pair.KVPairs[:i]
		if sorted {
			sort.Sort(pair.KVPairs)
		}

		return pair
	}

	return KeyValuePair{
		Key:   n.Path,
		Value: n.Value,
	}
}

func (n *Node) UpdateTTL(expireTime time.Time, s *Store) {
	if !n.IsPermanent() {
		// check if the node has been expired
		// if the node is not expired, we need to stop the go routine associated with
		// that node.
		expired, _ := n.IsExpired()

		if !expired {
			n.stopExpire <- true // suspend it to modify the expiration
		}
	}

	if expireTime.Sub(Permanent) != 0 {
		n.ExpireTime = expireTime
		n.Expire(s)
	}
}

// Clone function clone the node recursively and return the new node.
// If the node is a directory, it will clone all the content under this directory.
// If the node is a key-value pair, it will clone the pair.
func (n *Node) Clone() *Node {
	if !n.IsDir() {
		return newKV(n.Path, n.Value, n.CreateIndex, n.CreateTerm, n.Parent, n.ACL, n.ExpireTime)
	}

	clone := newDir(n.Path, n.CreateIndex, n.CreateTerm, n.Parent, n.ACL, n.ExpireTime)

	for key, child := range n.Children {
		clone.Children[key] = child.Clone()
	}

	return clone
}

// recoverAndclean function help to do recovery.
// Two things need to be done: 1. recovery structure; 2. delete expired nodes

// If the node is a directory, it will help recover children's parent pointer and recursively
// call this function on its children.
// We check the expire last since we need to recover the whole structure first and add all the
// notifications into the event history.
func (n *Node) recoverAndclean(s *Store) {
	if n.IsDir() {
		for _, child := range n.Children {
			child.Parent = n
			child.recoverAndclean(s)
		}
	}

	n.stopExpire = make(chan bool, 1)

	n.Expire(s)
}
