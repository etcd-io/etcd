package store

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	etcdErr "github.com/coreos/etcd/error"
)

// The default version to set when the store is first initialized.
const defaultVersion = 2

type Store interface {
	Version() int
	CommandFactory() CommandFactory
	Get(nodePath string, recursive, sorted bool, index uint64, term uint64) (*Event, error)
	Set(nodePath string, value string, expireTime time.Time, index uint64, term uint64) (*Event, error)
	Update(nodePath string, newValue string, expireTime time.Time, index uint64, term uint64) (*Event, error)
	Create(nodePath string, value string, incrementalSuffix bool, expireTime time.Time,
		index uint64, term uint64) (*Event, error)
	CompareAndSwap(nodePath string, prevValue string, prevIndex uint64,
		value string, expireTime time.Time, index uint64, term uint64) (*Event, error)
	Delete(nodePath string, recursive bool, index uint64, term uint64) (*Event, error)
	Watch(prefix string, recursive bool, sinceIndex uint64, index uint64, term uint64) (<-chan *Event, error)
	Save() ([]byte, error)
	Recovery(state []byte) error
	TotalTransactions() uint64
	JsonStats() []byte
}

type store struct {
	Root           *Node
	WatcherHub     *watcherHub
	Index          uint64
	Term           uint64
	Stats          *Stats
	CurrentVersion int
	worldLock      sync.RWMutex // stop the world lock
}

func New() Store {
	return newStore()
}

func newStore() *store {
	s := new(store)
	s.CurrentVersion = defaultVersion
	s.Root = newDir(s, "/", UndefIndex, UndefTerm, nil, "", Permanent)
	s.Stats = newStats()
	s.WatcherHub = newWatchHub(1000)
	return s
}

// Version retrieves current version of the store.
func (s *store) Version() int {
	return s.CurrentVersion
}

// CommandFactory retrieves the command factory for the current version of the store.
func (s *store) CommandFactory() CommandFactory {
	return GetCommandFactory(s.Version())
}

// Get function returns a get event.
// If recursive is true, it will return all the content under the node path.
// If sorted is true, it will sort the content by keys.
func (s *store) Get(nodePath string, recursive, sorted bool, index uint64, term uint64) (*Event, error) {
	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	nodePath = path.Clean(path.Join("/", nodePath))

	n, err := s.internalGet(nodePath, index, term)

	if err != nil {
		s.Stats.Inc(GetFail)
		return nil, err
	}

	e := newEvent(Get, nodePath, index, term)

	if n.IsDir() { // node is a directory
		e.Dir = true

		children, _ := n.List()
		e.KVPairs = make([]KeyValuePair, len(children))

		// we do not use the index in the children slice directly
		// we need to skip the hidden one
		i := 0

		for _, child := range children {
			if child.IsHidden() { // get will not return hidden nodes
				continue
			}

			e.KVPairs[i] = child.Pair(recursive, sorted)
			i++
		}

		// eliminate hidden nodes
		e.KVPairs = e.KVPairs[:i]

		if sorted {
			sort.Sort(e.KVPairs)
		}

	} else { // node is a file
		e.Value, _ = n.Read()
	}

	e.Expiration, e.TTL = n.ExpirationAndTTL()

	s.Stats.Inc(GetSuccess)

	return e, nil
}

// Create function creates the Node at nodePath. Create will help to create intermediate directories with no ttl.
// If the node has already existed, create will fail.
// If any node on the path is a file, create will fail.
func (s *store) Create(nodePath string, value string, unique bool,
	expireTime time.Time, index uint64, term uint64) (*Event, error) {
	nodePath = path.Clean(path.Join("/", nodePath))

	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	e, err := s.internalCreate(nodePath, value, unique, false, expireTime, index, term, Create)

	if err == nil {
		s.Stats.Inc(CreateSuccess)
	} else {
		s.Stats.Inc(CreateFail)
	}

	return e, err
}

// Set function creates or replace the Node at nodePath.
func (s *store) Set(nodePath string, value string, expireTime time.Time, index uint64, term uint64) (*Event, error) {
	nodePath = path.Clean(path.Join("/", nodePath))

	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	e, err := s.internalCreate(nodePath, value, false, true, expireTime, index, term, Set)

	if err == nil {
		s.Stats.Inc(SetSuccess)
	} else {
		s.Stats.Inc(SetFail)
	}

	return e, err
}

func (s *store) CompareAndSwap(nodePath string, prevValue string, prevIndex uint64,
	value string, expireTime time.Time, index uint64, term uint64) (*Event, error) {

	nodePath = path.Clean(path.Join("/", nodePath))

	s.worldLock.Lock()
	defer s.worldLock.Unlock()

	n, err := s.internalGet(nodePath, index, term)

	if err != nil {
		s.Stats.Inc(CompareAndSwapFail)
		return nil, err
	}

	if n.IsDir() { // can only test and set file
		s.Stats.Inc(CompareAndSwapFail)
		return nil, etcdErr.NewError(etcdErr.EcodeNotFile, nodePath, index, term)
	}

	// If both of the prevValue and prevIndex are given, we will test both of them.
	// Command will be executed, only if both of the tests are successful.
	if (prevValue == "" || n.Value == prevValue) && (prevIndex == 0 || n.ModifiedIndex == prevIndex) {
		e := newEvent(CompareAndSwap, nodePath, index, term)
		e.PrevValue = n.Value

		// if test succeed, write the value
		n.Write(value, index, term)
		n.UpdateTTL(expireTime)

		e.Value = value
		e.Expiration, e.TTL = n.ExpirationAndTTL()

		s.WatcherHub.notify(e)
		s.Stats.Inc(CompareAndSwapSuccess)
		return e, nil
	}

	cause := fmt.Sprintf("[%v != %v] [%v != %v]", prevValue, n.Value, prevIndex, n.ModifiedIndex)
	s.Stats.Inc(CompareAndSwapFail)
	return nil, etcdErr.NewError(etcdErr.EcodeTestFailed, cause, index, term)
}

// Delete function deletes the node at the given path.
// If the node is a directory, recursive must be true to delete it.
func (s *store) Delete(nodePath string, recursive bool, index uint64, term uint64) (*Event, error) {
	nodePath = path.Clean(path.Join("/", nodePath))

	s.worldLock.Lock()
	defer s.worldLock.Unlock()

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
		// notify the watchers with delted set true
		s.WatcherHub.notifyWatchers(e, path, true)
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

func (s *store) Watch(prefix string, recursive bool, sinceIndex uint64, index uint64, term uint64) (<-chan *Event, error) {
	prefix = path.Clean(path.Join("/", prefix))

	s.worldLock.RLock()
	defer s.worldLock.RUnlock()

	s.Index, s.Term = index, term

	var c <-chan *Event
	var err *etcdErr.Error

	if sinceIndex == 0 {
		c, err = s.WatcherHub.watch(prefix, recursive, index+1)

	} else {
		c, err = s.WatcherHub.watch(prefix, recursive, sinceIndex)
	}

	if err != nil {
		err.Index = index
		err.Term = term
		return nil, err
	}

	return c, nil
}

// walk function walks all the nodePath and apply the walkFunc on each directory
func (s *store) walk(nodePath string, walkFunc func(prev *Node, component string) (*Node, *etcdErr.Error)) (*Node, *etcdErr.Error) {
	components := strings.Split(nodePath, "/")

	curr := s.Root
	var err *etcdErr.Error

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

// Update function updates the value/ttl of the node.
// If the node is a file, the value and the ttl can be updated.
// If the node is a directory, only the ttl can be updated.
func (s *store) Update(nodePath string, newValue string, expireTime time.Time, index uint64, term uint64) (*Event, error) {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	nodePath = path.Clean(path.Join("/", nodePath))

	n, err := s.internalGet(nodePath, index, term)

	if err != nil { // if the node does not exist, return error
		s.Stats.Inc(UpdateFail)
		return nil, err
	}

	e := newEvent(Update, nodePath, s.Index, s.Term)

	if len(newValue) != 0 {
		if n.IsDir() {
			// if the node is a directory, we cannot update value
			s.Stats.Inc(UpdateFail)
			return nil, etcdErr.NewError(etcdErr.EcodeNotFile, nodePath, index, term)
		}

		e.PrevValue = n.Value
		n.Write(newValue, index, term)
	}

	// update ttl
	n.UpdateTTL(expireTime)

	e.Value = newValue

	e.Expiration, e.TTL = n.ExpirationAndTTL()

	s.WatcherHub.notify(e)

	s.Stats.Inc(UpdateSuccess)

	return e, nil
}

func (s *store) internalCreate(nodePath string, value string, unique bool, replace bool,
	expireTime time.Time, index uint64, term uint64, action string) (*Event, error) {

	s.Index, s.Term = index, term

	if unique { // append unique item under the node path
		nodePath += "/" + strconv.FormatUint(index, 10)
	}

	nodePath = path.Clean(path.Join("/", nodePath))

	dir, newNodeName := path.Split(nodePath)

	// walk through the nodePath, create dirs and get the last directory node
	d, err := s.walk(dir, s.checkDir)

	if err != nil {
		s.Stats.Inc(SetFail)
		err.Index, err.Term = s.Index, s.Term
		return nil, err
	}

	e := newEvent(action, nodePath, s.Index, s.Term)

	n, _ := d.GetChild(newNodeName)

	// force will try to replace a existing file
	if n != nil {
		if replace {
			if n.IsDir() {
				return nil, etcdErr.NewError(etcdErr.EcodeNotFile, nodePath, index, term)
			}
			e.PrevValue, _ = n.Read()

			n.Remove(false, nil)
		} else {
			return nil, etcdErr.NewError(etcdErr.EcodeNodeExist, nodePath, index, term)
		}
	}

	if len(value) != 0 { // create file
		e.Value = value

		n = newKV(s, nodePath, value, index, term, d, "", expireTime)

	} else { // create directory
		e.Dir = true

		n = newDir(s, nodePath, index, term, d, "", expireTime)

	}

	// we are sure d is a directory and does not have the children with name n.Name
	d.Add(n)

	// Node with TTL
	if expireTime.Sub(Permanent) != 0 {
		n.Expire()
		e.Expiration, e.TTL = n.ExpirationAndTTL()
	}

	s.WatcherHub.notify(e)
	return e, nil
}

// InternalGet function get the node of the given nodePath.
func (s *store) internalGet(nodePath string, index uint64, term uint64) (*Node, *etcdErr.Error) {
	nodePath = path.Clean(path.Join("/", nodePath))

	// update file system known index and term
	if index > s.Index {
		s.Index, s.Term = index, term
	}

	walkFunc := func(parent *Node, name string) (*Node, *etcdErr.Error) {

		if !parent.IsDir() {
			err := etcdErr.NewError(etcdErr.EcodeNotDir, parent.Path, index, term)
			return nil, err
		}

		child, ok := parent.Children[name]
		if ok {
			return child, nil
		}

		return nil, etcdErr.NewError(etcdErr.EcodeKeyNotFound, path.Join(parent.Path, name), index, term)
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
func (s *store) checkDir(parent *Node, dirName string) (*Node, *etcdErr.Error) {
	node, ok := parent.Children[dirName]

	if ok {
		if node.IsDir() {
			return node, nil
		}

		return nil, etcdErr.NewError(etcdErr.EcodeNotDir, parent.Path, UndefIndex, UndefTerm)
	}

	n := newDir(s, path.Join(parent.Path, dirName), s.Index, s.Term, parent, parent.ACL, Permanent)

	parent.Children[dirName] = n

	return n, nil
}

// Save function saves the static state of the store system.
// Save function will not be able to save the state of watchers.
// Save function will not save the parent field of the node. Or there will
// be cyclic dependencies issue for the json package.
func (s *store) Save() ([]byte, error) {
	s.worldLock.Lock()

	clonedStore := newStore()
	clonedStore.Index = s.Index
	clonedStore.Term = s.Term
	clonedStore.Root = s.Root.Clone()
	clonedStore.WatcherHub = s.WatcherHub.clone()
	clonedStore.Stats = s.Stats.clone()
	clonedStore.CurrentVersion = s.CurrentVersion

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
func (s *store) Recovery(state []byte) error {
	s.worldLock.Lock()
	defer s.worldLock.Unlock()
	err := json.Unmarshal(state, s)

	if err != nil {
		return err
	}

	s.Root.recoverAndclean()
	return nil
}

func (s *store) JsonStats() []byte {
	s.Stats.Watchers = uint64(s.WatcherHub.count)
	return s.Stats.toJson()
}

func (s *store) TotalTransactions() uint64 {
	return s.Stats.TotalTranscations()
}
