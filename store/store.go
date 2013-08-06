package store

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"sync"
	"time"
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// The main struct of the Key-Value store
type Store struct {

	// key-value store structure
	Tree *tree

	// This mutex protects everything except add watcher member.
	// Add watch member does not depend on the current state of the store.
	// And watch will return when other protected function is called and reach
	// the watching condition.
	// It is needed so that clone() can atomically replicate the Store
	// and do the log snapshot in a go routine.
	mutex sync.Mutex

	// WatcherHub is where we register all the clients
	// who issue a watch request
	watcher *WatcherHub

	// The string channel to send messages to the outside world
	// Now we use it to send changes to the hub of the web service
	messager *chan string

	// A map to keep the recent response to the clients
	ResponseMap map[string]*Response

	// The max number of the recent responses we can record
	ResponseMaxSize int

	// The current number of the recent responses we have recorded
	ResponseCurrSize uint

	// The index of the first recent responses we have
	ResponseStartIndex uint64

	// Current index of the raft machine
	Index uint64

	// Basic statistics information of etcd storage
	BasicStats EtcdStats
}

// A Node represents a Value in the Key-Value pair in the store
// It has its value, expire time and a channel used to update the
// expire time (since we do countdown in a go routine, we need to
// communicate with it via channel)
type Node struct {
	// The string value of the node
	Value string `json:"value"`

	// If the node is a permanent one the ExprieTime will be Unix(0,0)
	// Otherwise after the expireTime, the node will be deleted
	ExpireTime time.Time `json:"expireTime"`

	// A channel to update the expireTime of the node
	update chan time.Time `json:"-"`
}

// The response from the store to the user who issue a command
type Response struct {
	Action    string `json:"action"`
	Key       string `json:"key"`
	Dir       bool   `json:"dir,omitempty"`
	PrevValue string `json:"prevValue,omitempty"`
	Value     string `json:"value,omitempty"`

	// If the key did not exist before the action,
	// this field should be set to true
	NewKey bool `json:"newKey,omitempty"`

	Expiration *time.Time `json:"expiration,omitempty"`

	// Time to live in second
	TTL int64 `json:"ttl,omitempty"`

	// The command index of the raft machine when the command is executed
	Index uint64 `json:"index"`
}

// A listNode represent the simplest Key-Value pair with its type
// It is only used when do list opeartion
// We want to have a file system like store, thus we distingush "file"
// and "directory"
type ListNode struct {
	Key   string
	Value string
	Type  string
}

var PERMANENT = time.Unix(0, 0)

//------------------------------------------------------------------------------
//
// Methods
//
//------------------------------------------------------------------------------

// Create a new stroe
// Arguement max is the max number of response we want to record
func CreateStore(max int) *Store {
	s := new(Store)

	s.messager = nil

	s.ResponseMap = make(map[string]*Response)
	s.ResponseStartIndex = 0
	s.ResponseMaxSize = max
	s.ResponseCurrSize = 0

	s.Tree = &tree{
		&treeNode{
			Node{
				"/",
				time.Unix(0, 0),
				nil,
			},
			true,
			make(map[string]*treeNode),
		},
	}

	s.watcher = newWatcherHub()

	return s
}

// Set the messager of the store
func (s *Store) SetMessager(messager *chan string) {
	s.messager = messager
}

func (s *Store) Set(key string, value string, expireTime time.Time, index uint64) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.internalSet(key, value, expireTime, index)

}

// Set the key to value with expiration time
func (s *Store) internalSet(key string, value string, expireTime time.Time, index uint64) ([]byte, error) {
	//Update index
	s.Index = index

	//Update stats
	s.BasicStats.Sets++

	key = path.Clean("/" + key)

	isExpire := !expireTime.Equal(PERMANENT)

	// base response
	resp := Response{
		Action: "SET",
		Key:    key,
		Value:  value,
		Index:  index,
	}

	// When the slow follower receive the set command
	// the key may be expired, we should not add the node
	// also if the node exist, we need to delete the node
	if isExpire && expireTime.Sub(time.Now()) < 0 {
		return s.internalDelete(key, index)
	}

	var TTL int64

	// Update ttl
	if isExpire {
		TTL = int64(expireTime.Sub(time.Now()) / time.Second)
		resp.Expiration = &expireTime
		resp.TTL = TTL
	}

	// Get the node
	node, ok := s.Tree.get(key)

	if ok {
		// Update when node exists

		// Node is not permanent
		if !node.ExpireTime.Equal(PERMANENT) {

			// If node is not permanent
			// Update its expireTime
			node.update <- expireTime

		} else {

			// If we want the permanent node to have expire time
			// We need to create create a go routine with a channel
			if isExpire {
				node.update = make(chan time.Time)
				go s.monitorExpiration(key, node.update, expireTime)
			}

		}

		// Update the information of the node
		s.Tree.set(key, Node{value, expireTime, node.update})

		resp.PrevValue = node.Value

		s.watcher.notify(resp)

		msg, err := json.Marshal(resp)

		// Send to the messager
		if s.messager != nil && err == nil {

			*s.messager <- string(msg)
		}

		s.addToResponseMap(index, &resp)

		return msg, err

		// Add new node
	} else {

		update := make(chan time.Time)

		ok := s.Tree.set(key, Node{value, expireTime, update})

		if !ok {
			err := NotFile(key)
			return nil, err
		}

		if isExpire {
			go s.monitorExpiration(key, update, expireTime)
		}

		resp.NewKey = true

		msg, err := json.Marshal(resp)

		// Nofity the watcher
		s.watcher.notify(resp)

		// Send to the messager
		if s.messager != nil && err == nil {

			*s.messager <- string(msg)
		}

		s.addToResponseMap(index, &resp)
		return msg, err
	}

}

// Get the value of the key and return the raw response
func (s *Store) internalGet(key string) *Response {

	key = path.Clean("/" + key)

	node, ok := s.Tree.get(key)

	if ok {
		var TTL int64
		var isExpire bool = false

		isExpire = !node.ExpireTime.Equal(PERMANENT)

		resp := &Response{
			Action: "GET",
			Key:    key,
			Value:  node.Value,
			Index:  s.Index,
		}

		// Update ttl
		if isExpire {
			TTL = int64(node.ExpireTime.Sub(time.Now()) / time.Second)
			resp.Expiration = &node.ExpireTime
			resp.TTL = TTL
		}

		return resp

	} else {
		// we do not found the key
		return nil
	}
}

// Get all the items under key
// If key is a file return the file
// If key is a directory reuturn an array of files
func (s *Store) Get(key string) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	resps, err := s.RawGet(key)

	if err != nil {
		return nil, err
	}

	if len(resps) == 1 {
		return json.Marshal(resps[0])
	}

	return json.Marshal(resps)
}

func (s *Store) RawGet(key string) ([]*Response, error) {
	// Update stats
	s.BasicStats.Gets++

	key = path.Clean("/" + key)

	nodes, keys, ok := s.Tree.list(key)

	if ok {

		node, ok := nodes.(*Node)

		if ok {
			resps := make([]*Response, 1)

			isExpire := !node.ExpireTime.Equal(PERMANENT)

			resps[0] = &Response{
				Action: "GET",
				Index:  s.Index,
				Key:    key,
				Value:  node.Value,
			}

			// Update ttl
			if isExpire {
				TTL := int64(node.ExpireTime.Sub(time.Now()) / time.Second)
				resps[0].Expiration = &node.ExpireTime
				resps[0].TTL = TTL
			}

			return resps, nil
		}

		nodes, _ := nodes.([]*Node)

		resps := make([]*Response, len(nodes))
		for i := 0; i < len(nodes); i++ {

			var TTL int64
			var isExpire bool = false

			isExpire = !nodes[i].ExpireTime.Equal(PERMANENT)

			resps[i] = &Response{
				Action: "GET",
				Index:  s.Index,
				Key:    path.Join(key, keys[i]),
			}

			if len(nodes[i].Value) != 0 {
				resps[i].Value = nodes[i].Value
			} else {
				resps[i].Dir = true
			}

			// Update ttl
			if isExpire {
				TTL = int64(nodes[i].ExpireTime.Sub(time.Now()) / time.Second)
				resps[i].Expiration = &nodes[i].ExpireTime
				resps[i].TTL = TTL
			}

		}

		return resps, nil
	}

	err := NotFoundError(key)
	return nil, err
}

func (s *Store) Delete(key string, index uint64) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.internalDelete(key, index)
}

// Delete the key
func (s *Store) internalDelete(key string, index uint64) ([]byte, error) {

	// Update stats
	s.BasicStats.Deletes++

	key = path.Clean("/" + key)

	// Update index
	s.Index = index

	node, ok := s.Tree.get(key)

	if ok {

		resp := Response{
			Action:    "DELETE",
			Key:       key,
			PrevValue: node.Value,
			Index:     index,
		}

		if node.ExpireTime.Equal(PERMANENT) {

			s.Tree.delete(key)

		} else {
			resp.Expiration = &node.ExpireTime
			// Kill the expire go routine
			node.update <- PERMANENT
			s.Tree.delete(key)

		}

		msg, err := json.Marshal(resp)

		s.watcher.notify(resp)

		// notify the messager
		if s.messager != nil && err == nil {

			*s.messager <- string(msg)
		}

		s.addToResponseMap(index, &resp)

		return msg, err

	} else {
		err := NotFoundError(key)
		return nil, err
	}
}

// Set the value of the key to the value if the given prevValue is equal to the value of the key
func (s *Store) TestAndSet(key string, prevValue string, value string, expireTime time.Time, index uint64) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Update stats
	s.BasicStats.TestAndSets++

	resp := s.internalGet(key)

	if resp == nil {
		err := NotFoundError(key)
		return nil, err
	}

	if resp.Value == prevValue {

		// If test success, do set
		return s.internalSet(key, value, expireTime, index)
	} else {

		// If fails, return err
		err := TestFail(fmt.Sprintf("TestAndSet: %s!=%s", resp.Value, prevValue))
		return nil, err
	}

}

// Add a channel to the watchHub.
// The watchHub will send response to the channel when any key under the prefix
// changes [since the sinceIndex if given]
func (s *Store) AddWatcher(prefix string, watcher *Watcher, sinceIndex uint64) error {
	return s.watcher.addWatcher(prefix, watcher, sinceIndex, s.ResponseStartIndex, s.Index, &s.ResponseMap)
}

// This function should be created as a go routine to delete the key-value pair
// when it reaches expiration time

func (s *Store) monitorExpiration(key string, update chan time.Time, expireTime time.Time) {

	duration := expireTime.Sub(time.Now())

	for {
		select {

		// Timeout delete the node
		case <-time.After(duration):
			node, ok := s.Tree.get(key)

			if !ok {
				return

			} else {
				s.mutex.Lock()

				s.Tree.delete(key)

				resp := Response{
					Action:     "DELETE",
					Key:        key,
					PrevValue:  node.Value,
					Expiration: &node.ExpireTime,
					Index:      s.Index,
				}
				s.mutex.Unlock()

				msg, err := json.Marshal(resp)

				s.watcher.notify(resp)

				// notify the messager
				if s.messager != nil && err == nil {

					*s.messager <- string(msg)
				}

				return

			}

		case updateTime := <-update:
			// Update duration
			// If the node become a permanent one, the go routine is
			// not needed
			if updateTime.Equal(PERMANENT) {
				return
			}

			// Update duration
			duration = updateTime.Sub(time.Now())
		}
	}
}

// When we receive a command that will change the state of the key-value store
// We will add the result of it to the ResponseMap for the use of watch command
// Also we may remove the oldest response when we add new one
func (s *Store) addToResponseMap(index uint64, resp *Response) {

	// zero case
	if s.ResponseMaxSize == 0 {
		return
	}

	strIndex := strconv.FormatUint(index, 10)
	s.ResponseMap[strIndex] = resp

	// unlimited
	if s.ResponseMaxSize < 0 {
		s.ResponseCurrSize++
		return
	}

	// if we reach the max point, we need to delete the most latest
	// response and update the startIndex
	if s.ResponseCurrSize == uint(s.ResponseMaxSize) {
		s.ResponseStartIndex++
		delete(s.ResponseMap, strconv.FormatUint(s.ResponseStartIndex, 10))
	} else {
		s.ResponseCurrSize++
	}
}

func (s *Store) clone() *Store {
	newStore := &Store{
		ResponseMaxSize:    s.ResponseMaxSize,
		ResponseCurrSize:   s.ResponseCurrSize,
		ResponseStartIndex: s.ResponseStartIndex,
		Index:              s.Index,
		BasicStats:         s.BasicStats,
	}

	newStore.Tree = s.Tree.clone()
	newStore.ResponseMap = make(map[string]*Response)

	for index, response := range s.ResponseMap {
		newStore.ResponseMap[index] = response
	}

	return newStore
}

// Save the current state of the storage system
func (s *Store) Save() ([]byte, error) {
	// first we clone the store
	// json is very slow, we cannot hold the lock for such a long time
	s.mutex.Lock()
	cloneStore := s.clone()
	s.mutex.Unlock()

	b, err := json.Marshal(cloneStore)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return b, nil
}

// Recovery the state of the stroage system from a previous state
func (s *Store) Recovery(state []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// we need to stop all the current watchers
	// recovery will clear watcherHub
	s.watcher.stopWatchers()

	err := json.Unmarshal(state, s)

	// The only thing need to change after the recovery is the
	// node with expiration time, we need to delete all the node
	// that have been expired and setup go routines to monitor the
	// other ones
	s.checkExpiration()

	return err
}

// Clean the expired nodes
// Set up go routines to mon
func (s *Store) checkExpiration() {
	s.Tree.traverse(s.checkNode, false)
}

// Check each node
func (s *Store) checkNode(key string, node *Node) {

	if node.ExpireTime.Equal(PERMANENT) {
		return
	} else {
		if node.ExpireTime.Sub(time.Now()) >= time.Second {

			node.update = make(chan time.Time)
			go s.monitorExpiration(key, node.update, node.ExpireTime)

		} else {
			// we should delete this node
			s.Tree.delete(key)
		}
	}
}
