package store

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	etcdErr "github.com/coreos/etcd/error"
)

type Store struct {
	Root       *Node
	WatcherHub *watcherHub
	Index      uint64
	Term       uint64
	Stats      *Stats
	worldLock  sync.RWMutex // stop the world lock. Used to do snapshot
}

func New() *Store {
	s := new(Store)
	s.Root = newDir("/", 0, 0, nil, "", Permanent)
	s.Stats = newStats()
	s.WatcherHub = newWatchHub(1000)

	return s
}

func (s *Store) Get(nodePath string, recursive, sorted bool, index uint64, term uint64) (*Event, error) {
	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	nodePath = path.Clean(path.Join("/", nodePath))

	n, err := s.internalGet(nodePath, index, term)

	if err != nil {
		s.Stats.Inc(GetFail)
		return nil, err
	}

	e := newEvent(Get, nodePath, index, term)

	if n.IsDir() { // node is dir
		e.Dir = true

		children, _ := n.List()
		e.KVPairs = make([]KeyValuePair, len(children))

		// we do not use the index in the children slice directly
		// we need to skip the hidden one
		i := 0

		for _, child := range children {

			if child.IsHidden() { // get will not list hidden node
				continue
			}

			e.KVPairs[i] = child.Pair(recursive, sorted)

			i++
		}

		// eliminate hidden nodes
		e.KVPairs = e.KVPairs[:i]

		rootPairs := KeyValuePair{
			KVPairs: e.KVPairs,
		}

		if sorted {
			sort.Sort(rootPairs)
		}

	} else { // node is file
		e.Value = n.Value
	}

	if n.ExpireTime.Sub(Permanent) != 0 {
		e.Expiration = &n.ExpireTime
		e.TTL = int64(n.ExpireTime.Sub(time.Now())/time.Second) + 1
	}

	s.Stats.Inc(GetSuccess)

	return e, nil
}

// Create function creates the Node at nodePath. Create will help to create intermediate directories with no ttl.
// If the node has already existed, create will fail.
// If any node on the path is a file, create will fail.
func (s *Store) Create(nodePath string, value string, expireTime time.Time, index uint64, term uint64) (*Event, error) {
	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	nodePath = path.Clean(path.Join("/", nodePath))

	// make sure we can create the node
	_, err := s.internalGet(nodePath, index, term)

	if err == nil { // key already exists
		s.Stats.Inc(SetFail)
		return nil, etcdErr.NewError(etcdErr.EcodeNodeExist, nodePath)
	}

	etcdError, _ := err.(etcdErr.Error)

	if etcdError.ErrorCode == 104 { // we cannot create the key due to meet a file while walking
		s.Stats.Inc(SetFail)
		return nil, err
	}

	dir, _ := path.Split(nodePath)

	// walk through the nodePath, create dirs and get the last directory node
	d, err := s.walk(dir, s.checkDir)

	if err != nil {
		s.Stats.Inc(SetFail)
		return nil, err
	}

	e := newEvent(Create, nodePath, s.Index, s.Term)

	var n *Node

	if len(value) != 0 { // create file
		e.Value = value

		n = newFile(nodePath, value, s.Index, s.Term, d, "", expireTime)

	} else { // create directory
		e.Dir = true

		n = newDir(nodePath, s.Index, s.Term, d, "", expireTime)

	}

	err = d.Add(n)

	if err != nil {
		s.Stats.Inc(SetFail)
		return nil, err
	}

	// Node with TTL
	if expireTime.Sub(Permanent) != 0 {
		n.Expire(s)
		e.Expiration = &n.ExpireTime
		e.TTL = int64(expireTime.Sub(time.Now())/time.Second) + 1
	}

	s.WatcherHub.notify(e)
	s.Stats.Inc(SetSuccess)
	return e, nil
}

// Update function updates the value/ttl of the node.
// If the node is a file, the value and the ttl can be updated.
// If the node is a directory, only the ttl can be updated.
func (s *Store) Update(nodePath string, value string, expireTime time.Time, index uint64, term uint64) (*Event, error) {
	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	n, err := s.internalGet(nodePath, index, term)

	if err != nil { // if the node does not exist, return error
		s.Stats.Inc(UpdateFail)

		return nil, err
	}

	e := newEvent(Update, nodePath, s.Index, s.Term)

	if n.IsDir() { // if the node is a directory, we can only update ttl
		if len(value) != 0 {
			s.Stats.Inc(UpdateFail)

			return nil, etcdErr.NewError(etcdErr.EcodeNotFile, nodePath)
		}

	} else { // if the node is a file, we can update value and ttl
		e.PrevValue = n.Value

		if len(value) != 0 {
			e.Value = value
		}

		n.Write(value, index, term)
	}

	// update ttl
	n.UpdateTTL(expireTime, s)

	e.Expiration = &n.ExpireTime
	e.TTL = int64(expireTime.Sub(time.Now())/time.Second) + 1
	s.WatcherHub.notify(e)

	s.Stats.Inc(UpdateSuccess)

	return e, nil
}

func (s *Store) TestAndSet(nodePath string, prevValue string, prevIndex uint64,
	value string, expireTime time.Time, index uint64, term uint64) (*Event, error) {

	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	n, err := s.internalGet(nodePath, index, term)

	if err != nil {
		s.Stats.Inc(TestAndSetFail)
		return nil, err
	}

	if n.IsDir() { // can only test and set file
		s.Stats.Inc(TestAndSetFail)
		return nil, etcdErr.NewError(etcdErr.EcodeNotFile, nodePath)
	}

	if n.Value == prevValue || n.ModifiedIndex == prevIndex {
		// if test succeed, write the value
		e := newEvent(TestAndSet, nodePath, index, term)
		e.PrevValue = n.Value
		e.Value = value
		n.Write(value, index, term)

		n.UpdateTTL(expireTime, s)

		s.WatcherHub.notify(e)
		s.Stats.Inc(TestAndSetSuccess)
		return e, nil
	}

	cause := fmt.Sprintf("[%v != %v] [%v != %v]", prevValue, n.Value, prevIndex, n.ModifiedIndex)
	s.Stats.Inc(TestAndSetFail)
	return nil, etcdErr.NewError(etcdErr.EcodeTestFailed, cause)
}

// Delete function deletes the node at the given path.
// If the node is a directory, recursive must be true to delete it.
func (s *Store) Delete(nodePath string, recursive bool, index uint64, term uint64) (*Event, error) {
	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	n, err := s.internalGet(nodePath, index, term)

	if err != nil { // if the node does not exist, return error
		s.Stats.Inc(DeleteFail)
		return nil, err
	}

	e := newEvent(Delete, nodePath, index, term)

	if n.IsDir() {
		e.Dir = true
	} else {
		e.PrevValue = n.Value
	}

	callback := func(path string) { // notify function
		s.WatcherHub.notifyWithPath(e, path, true)
	}

	err = n.Remove(recursive, callback)

	if err != nil {
		s.Stats.Inc(DeleteFail)
		return nil, err
	}

	s.WatcherHub.notify(e)
	s.Stats.Inc(DeleteSuccess)

	return e, nil
}

func (s *Store) Watch(prefix string, recursive bool, sinceIndex uint64, index uint64, term uint64) (<-chan *Event, error) {
	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	s.Index, s.Term = index, term

	if sinceIndex == 0 {
		return s.WatcherHub.watch(prefix, recursive, index+1)
	}

	return s.WatcherHub.watch(prefix, recursive, sinceIndex)
}

// walk function walks all the nodePath and apply the walkFunc on each directory
func (s *Store) walk(nodePath string, walkFunc func(prev *Node, component string) (*Node, error)) (*Node, error) {
	components := strings.Split(nodePath, "/")

	curr := s.Root

	var err error
	for i := 1; i < len(components); i++ {
		if len(components[i]) == 0 { // ignore empty string
			return curr, nil
		}

		curr, err = walkFunc(curr, components[i])
		if err != nil {
			return nil, err
		}

	}

	return curr, nil
}

// InternalGet function get the node of the given nodePath.
func (s *Store) internalGet(nodePath string, index uint64, term uint64) (*Node, error) {
	nodePath = path.Clean(path.Join("/", nodePath))

	// update file system known index and term
	s.Index, s.Term = index, term

	walkFunc := func(parent *Node, name string) (*Node, error) {

		if !parent.IsDir() {
			return nil, etcdErr.NewError(etcdErr.EcodeNotDir, parent.Path)
		}

		child, ok := parent.Children[name]
		if ok {
			return child, nil
		}

		return nil, etcdErr.NewError(etcdErr.EcodeKeyNotFound, path.Join(parent.Path, name))
	}

	f, err := s.walk(nodePath, walkFunc)

	if err != nil {
		return nil, err
	}

	return f, nil
}

// checkDir function will check whether the component is a directory under parent node.
// If it is a directory, this function will return the pointer to that node.
// If it does not exist, this function will create a new directory and return the pointer to that node.
// If it is a file, this function will return error.
func (s *Store) checkDir(parent *Node, dirName string) (*Node, error) {
	subDir, ok := parent.Children[dirName]

	if ok {
		return subDir, nil
	}

	n := newDir(path.Join(parent.Path, dirName), s.Index, s.Term, parent, parent.ACL, Permanent)

	parent.Children[dirName] = n

	return n, nil
}

// Save function saves the static state of the store system.
// Save function will not be able to save the state of watchers.
// Save function will not save the parent field of the node. Or there will
// be cyclic dependencies issue for the json package.
func (s *Store) Save() ([]byte, error) {
	s.worldLock.Lock()

	clonedStore := New()
	clonedStore.Index = s.Index
	clonedStore.Term = s.Term
	clonedStore.Root = s.Root.Clone()
	clonedStore.WatcherHub = s.WatcherHub.clone()
	clonedStore.Stats = s.Stats.clone()

	s.worldLock.Unlock()

	b, err := json.Marshal(clonedStore)

	if err != nil {
		return nil, err
	}

	return b, nil
}

// recovery function recovery the store system from a static state.
// It needs to recovery the parent field of the nodes.
// It needs to delete the expired nodes since the saved time and also
// need to create monitor go routines.
func (s *Store) Recovery(state []byte) error {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	err := json.Unmarshal(state, s)

	if err != nil {
		return err
	}

	s.Root.recoverAndclean(s)
	return nil
}

func (s *Store) JsonStats() []byte {
	s.Stats.Watchers = uint64(s.WatcherHub.count)
	return s.Stats.toJson()
}
