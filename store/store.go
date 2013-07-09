package store

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
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

	// WatcherHub is where we register all the clients
	// who issue a watch request
	watcher *WatcherHub

	// The string channel to send messages to the outside world
	// Now we use it to send changes to the hub of the web service
	messager *chan string

	// A map to keep the recent response to the clients
	ResponseMap map[string]Response

	// The max number of the recent responses we can record
	ResponseMaxSize int

	// The current number of the recent responses we have recorded
	ResponseCurrSize uint

	// The index of the first recent responses we have
	ResponseStartIndex uint64

	// Current index of the raft machine
	Index uint64
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
	PrevValue string `json:"prevValue"`
	Value     string `json:"value"`

	// If the key existed before the action, this field should be true
	// If the key did not exist before the action, this field should be false
	Exist bool `json:"exist"`

	Expiration time.Time `json:"expiration"`

	// Time to live in second
	TTL int64 `json:"ttl"`

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

	s.ResponseMap = make(map[string]Response)
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

	s.watcher = createWatcherHub()

	return s
}

// Set the messager of the store
func (s *Store) SetMessager(messager *chan string) {
	s.messager = messager
}

// Set the key to value with expiration time
func (s *Store) Set(key string, value string, expireTime time.Time, index uint64) ([]byte, error) {

	//Update index
	s.Index = index

	key = path.Clean("/" + key)

	isExpire := !expireTime.Equal(PERMANENT)

	// When the slow follower receive the set command
	// the key may be expired, we should not add the node
	// also if the node exist, we need to delete the node
	if isExpire && expireTime.Sub(time.Now()) < 0 {
		return s.Delete(key, index)
	}

	var TTL int64

	// Update ttl
	if isExpire {
		TTL = int64(expireTime.Sub(time.Now()) / time.Second)
	} else {
		// For permanent value, we set ttl to -1
		TTL = -1
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

		resp := Response{"SET", key, node.Value, value, true, expireTime, TTL, index}

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

		s.Tree.set(key, Node{value, expireTime, update})

		if isExpire {
			go s.monitorExpiration(key, update, expireTime)
		}

		resp := Response{"SET", key, "", value, false, expireTime, TTL, index}

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

// Get the value of the key
func (s *Store) Get(key string) Response {

	key = path.Clean("/" + key)

	node, ok := s.Tree.get(key)

	if ok {
		var TTL int64
		var isExpire bool = false

		isExpire = !node.ExpireTime.Equal(PERMANENT)

		// Update ttl
		if isExpire {
			TTL = int64(node.ExpireTime.Sub(time.Now()) / time.Second)
		} else {
			TTL = -1
		}

		return Response{"GET", key, node.Value, node.Value, true, node.ExpireTime, TTL, s.Index}

	} else {
		// we do not found the key
		return Response{"GET", key, "", "", false, time.Unix(0, 0), 0, s.Index}
	}
}

// List all the item in the prefix
func (s *Store) List(prefix string) ([]byte, error) {

	nodes, keys, dirs, ok := s.Tree.list(prefix)

	var ln []ListNode

	if ok {
		ln = make([]ListNode, len(nodes))
		for i := 0; i < len(nodes); i++ {
			ln[i] = ListNode{keys[i], nodes[i].Value, dirs[i]}
		}
	}

	return json.Marshal(ln)
}

// Delete the key
func (s *Store) Delete(key string, index uint64) ([]byte, error) {

	key = path.Clean("/" + key)

	//Update index
	s.Index = index

	node, ok := s.Tree.get(key)

	if ok {

		if node.ExpireTime.Equal(PERMANENT) {

			s.Tree.delete(key)

		} else {

			// Kill the expire go routine
			node.update <- PERMANENT
			s.Tree.delete(key)

		}

		resp := Response{"DELETE", key, node.Value, "", true, node.ExpireTime, 0, index}

		msg, err := json.Marshal(resp)

		s.watcher.notify(resp)

		// notify the messager
		if s.messager != nil && err == nil {

			*s.messager <- string(msg)
		}

		s.addToResponseMap(index, &resp)

		return msg, err

	} else {

		resp := Response{"DELETE", key, "", "", false, time.Unix(0, 0), 0, index}

		s.addToResponseMap(index, &resp)

		return json.Marshal(resp)
	}
}

// Set the value of the key to the value if the given prevValue is equal to the value of the key
func (s *Store) TestAndSet(key string, prevValue string, value string, expireTime time.Time, index uint64) ([]byte, error) {
	resp := s.Get(key)

	if resp.PrevValue == prevValue {

		// If test success, do set
		return s.Set(key, value, expireTime, index)
	} else {

		// If fails, return the result of get which contains the current
		// status of the key-value pair
		return json.Marshal(resp)
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

				s.Tree.delete(key)

				resp := Response{"DELETE", key, node.Value, "", true, node.ExpireTime, 0, s.Index}

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
	s.ResponseMap[strIndex] = *resp

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

// Save the current state of the storage system
func (s *Store) Save() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return b, nil
}

// Recovery the state of the stroage system from a previous state
func (s *Store) Recovery(state []byte) error {
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
