package store

import (
	"encoding/json"
	"fmt"
	"path"
	"time"
	"strconv"
)

// global store
var s *Store

// CONSTANTS
const (
	ERROR = -1 + iota
	SET
	DELETE
	GET
)

var PERMANENT = time.Unix(0, 0)

type Store struct {
	// // use the build-in hash map as the key-value store structure
	// Nodes map[string]Node `json:"nodes"`

	// use treeMap as the key-value stroe structure
	Tree *tree
	// the string channel to send messages to the outside world
	// now we use it to send changes to the hub of the web service
	messager *chan string

	// 
	ResponseMap map[string]Response

	//
	ResponseMaxSize int

	ResponseCurrSize uint

	// at some point, we may need to compact the Response
	ResponseStartIndex uint64

	// current Index
	Index uint64
}

type Node struct {
	Value string `json:"value"`

	// if the node is a permanent one the ExprieTime will be Unix(0,0)
	// Otherwise after the expireTime, the node will be deleted
	ExpireTime time.Time `json:"expireTime"`

	// a channel to update the expireTime of the node
	update chan time.Time `json:"-"`
}

type Response struct {
	Action   int    `json:"action"`
	Key      string `json:"key"`
	PrevValue string `json:"prevValue"`
	Value string `json:"value"`

	// if the key existed before the action, this field should be true
	// if the key did not exist before the action, this field should be false
	Exist bool `json:"exist"`

	Expiration time.Time `json:"expiration"`

	// countdown until expiration in seconds
	TTL int64 `json:"TTL"`

	Index uint64 `json:"index"`
}

// make a new stroe
func CreateStore(max int) *Store {
	s = new(Store)
	s.messager = nil
	s.ResponseMap = make(map[string]Response)
	s.ResponseStartIndex = 0
	s.ResponseMaxSize = max
	s.ResponseCurrSize = 0

	s.Tree = &tree{ 
		&treeNode{ 
			Node {
				"/",
				time.Unix(0,0),
				nil,
			},
			true, 
			make(map[string]*treeNode),
		},
	} 

	return s
}

// return a pointer to the store
func GetStore() *Store {
	return s
}

// set the messager of the store
func (s *Store) SetMessager(messager *chan string) {
	s.messager = messager
}

// set the key to value, return the old value if the key exists
func Set(key string, value string, expireTime time.Time, index uint64) ([]byte, error) {

	//update index
	s.Index = index
	
	key = "/" + key

	key = path.Clean(key)

	var isExpire bool = false

	isExpire = !expireTime.Equal(PERMANENT)

	// when the slow follower receive the set command
	// the key may be expired, we should not add the node
	// also if the node exist, we need to delete the node
	if isExpire && expireTime.Sub(time.Now()) < 0 {
		return Delete(key, index)
	}

	var TTL int64
	// update ttl
	if isExpire {
		TTL = int64(expireTime.Sub(time.Now()) / time.Second)
	} else {
		TTL = -1
	}

	// get the node
	node, ok := s.Tree.get(key)

	if ok {
		// if node is not permanent before
		// update its expireTime
		if !node.ExpireTime.Equal(PERMANENT) {

			node.update <- expireTime

		} else {
			// if we want the permanent node to have expire time
			// we need to create a chan and create a go routine
			if isExpire {
				node.update = make(chan time.Time)
				go expire(key, node.update, expireTime)
			}
			
		}

		// update the information of the node
		s.Tree.set(key, Node{value, expireTime, node.update})

		resp := Response{SET, key, node.Value, value, true, expireTime, TTL, index}

		msg, err := json.Marshal(resp)

		notify(resp)

		// send to the messager
		if s.messager != nil && err == nil {

			*s.messager <- string(msg)
		}

		updateMap(index, &resp)

		return msg, err

	// add new node
	} else {

		update := make(chan time.Time)

		s.Tree.set(key, Node{value, expireTime, update})

		if isExpire {
			go expire(key, update, expireTime)
		}

		resp := Response{SET, key, "", value, false, expireTime, TTL, index}

		msg, err := json.Marshal(resp)

		// nofity the watcher
		notify(resp)

		// notify the web interface
		if s.messager != nil && err == nil {

			*s.messager <- string(msg)
		}

		updateMap(index, &resp)
		return msg, err
	}
}

// should be used as a go routine to delete the key when it expires
func expire(key string, update chan time.Time, expireTime time.Time) {
	duration := expireTime.Sub(time.Now())

	for {
		select {
		// timeout delete the node
		case <-time.After(duration):
			node, ok := s.Tree.get(key)
			if !ok {
				return
			} else {

				s.Tree.delete(key)

				resp := Response{DELETE, key, node.Value, "", true, node.ExpireTime, 0, s.Index}

				msg, err := json.Marshal(resp)

				notify(resp)

				// notify the messager
				if s.messager != nil && err == nil {

					*s.messager <- string(msg)
				}

				return

			}

		case updateTime := <-update:
			//update duration
			// if the node become a permanent one, the go routine is
			// not needed
			if updateTime.Equal(PERMANENT) {
				fmt.Println("permanent")
				return
			}
			// update duration
			duration = updateTime.Sub(time.Now())
		}
	}
}

func updateMap(index uint64, resp *Response) {

	if s.ResponseMaxSize == 0 {
		return
	}

	strIndex := strconv.FormatUint(index, 10)
	s.ResponseMap[strIndex] = *resp

	// unlimited
	if s.ResponseMaxSize < 0{
		s.ResponseCurrSize++
		return
	}

	if s.ResponseCurrSize == uint(s.ResponseMaxSize) {
		s.ResponseStartIndex++
		delete(s.ResponseMap, strconv.FormatUint(s.ResponseStartIndex, 10))
	} else {
		s.ResponseCurrSize++
	}
}


// get the value of the key
func Get(key string) Response {
	key = "/" + key
	
	key = path.Clean(key)

	node, ok := s.Tree.get(key)

	if ok {
		var TTL int64
		var isExpire bool = false

		isExpire = !node.ExpireTime.Equal(PERMANENT)

		// update ttl
		if isExpire {
			TTL = int64(node.ExpireTime.Sub(time.Now()) / time.Second)
		} else {
			TTL = -1
		}

		return Response{GET, key, node.Value, node.Value, true, node.ExpireTime, TTL, s.Index}
	} else {

		return Response{GET, key, "", "", false, time.Unix(0, 0), 0, s.Index}
	}
}

// delete the key
func Delete(key string, index uint64) ([]byte, error) {
	//update index
	key = "/" + key

	s.Index = index

	key = path.Clean(key)

	node, ok := s.Tree.get(key)

	if ok {

		if node.ExpireTime.Equal(PERMANENT) {

			s.Tree.delete(key)

		} else {

			// kill the expire go routine
			node.update <- PERMANENT
			s.Tree.delete(key)

		}

		resp := Response{DELETE, key, node.Value, "", true, node.ExpireTime, 0, index}

		msg, err := json.Marshal(resp)

		notify(resp)

		// notify the messager
		if s.messager != nil && err == nil {

			*s.messager <- string(msg)
		}

		updateMap(index, &resp)
		return msg, err

	} else {

		resp := Response{DELETE, key, "", "", false, time.Unix(0, 0), 0, index}

		updateMap(index, &resp)

		return json.Marshal(resp)
	}
}

// save the current state of the storage system
func (s *Store) Save() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return b, nil
}

// recovery the state of the stroage system from a previous state
func (s *Store) Recovery(state []byte) error {
	err := json.Unmarshal(state, s)

	// clean the expired nodes
	clean()

	return err
}

// clean all expired keys
func clean() {
	s.Tree.traverse(cleanNode, false)
}


func cleanNode(key string, node *Node) {

	if node.ExpireTime.Equal(PERMANENT) {
		return
	} else {

		if node.ExpireTime.Sub(time.Now()) >= time.Second {
			node.update = make(chan time.Time)
			go expire(key, node.update, node.ExpireTime)

		} else {
			// we should delete this node
			s.Tree.delete(key)
		}
	}
}
