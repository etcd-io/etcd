package store

import (
	"path"
	"encoding/json"
	"time"
	"fmt"
	)

// CONSTANTS
const (
	ERROR = -1 + iota
	SET 
	DELETE
	GET
)

type Store struct {
	Nodes map[string]Node  `json:"nodes"`
	messager *chan string
}

type Node struct {
	Value string	`json:"value"`
	ExpireTime time.Time `json:"expireTime"`
	update chan time.Time `json:"-"`
}

type Response struct {
	Action	 int    `json:"action"`
	Key      string `json:"key"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
	Exist 	 bool `json:"exist"`
	Expiration time.Time `json:"expiration"`
}


// global store
var s *Store

func init() {
	s = createStore()
	s.messager = nil
}

// make a new stroe
func createStore() *Store{
	s := new(Store)
	s.Nodes = make(map[string]Node)
	return s
}

func GetStore() *Store {
	return s
}

func (s *Store)SetMessager(messager *chan string) {
	s.messager = messager
}	

// set the key to value, return the old value if the key exists 
func Set(key string, value string, expireTime time.Time) ([]byte, error) {

	key = path.Clean(key)

	var isExpire bool = false

	isExpire = !expireTime.Equal(time.Unix(0,0))

	// when the slow follower receive the set command
	// the key may be expired, we need also to delete 
	// the previous value of key
	if isExpire && expireTime.Sub(time.Now()) < 0 {
		return Delete(key)
	}

	node, ok := s.Nodes[key]

	if ok {
		//update := make(chan time.Time)
		//s.Nodes[key] = Node{value, expireTime, update}

		
		
		// if node is not permanent before 
		// update its expireTime
		if !node.ExpireTime.Equal(time.Unix(0,0)) {

				node.update <- expireTime

		} else {

			// if we want the permanent to have expire time
			// we need to create a chan and create a func
			if isExpire {
				node.update = make(chan time.Time)

				go expire(key, node.update, expireTime)
			}
		}

		node.ExpireTime = expireTime

		node.Value = value
		notify(SET, key, node.Value, value, true)
		
		msg, err := json.Marshal(Response{SET, key, node.Value, value, true, expireTime})

		// notify the web interface
		if (s.messager != nil && err == nil) {

			*s.messager <- string(msg)
		} 

		return msg, err

	} else {

		// add new node
		update := make(chan time.Time)

		s.Nodes[key] = Node{value, expireTime, update}

		// nofity the watcher
		notify(SET, key, "", value, false)

		if isExpire {
			go expire(key, update, expireTime)
		}

		msg, err := json.Marshal(Response{SET, key, "", value, false, expireTime})

		// notify the web interface
		if (s.messager != nil && err == nil) {

			*s.messager <- string(msg)
		} 

		return msg, err
	}
}

// delete the key when it expires
func expire(key string, update chan time.Time, expireTime time.Time) {
	duration := expireTime.Sub(time.Now())

	for {
		select {
		// timeout delte key
		case <-time.After(duration):
			fmt.Println("expired at ", time.Now())
			Delete(key)
			return
		case updateTime := <-update:
			//update duration
			if updateTime.Equal(time.Unix(0,0)) {
				fmt.Println("node became stable")
				return
			}
			duration = updateTime.Sub(time.Now())
		}
	}
}

// get the value of the key
func Get(key string) Response {
	key = path.Clean(key)

	node, ok := s.Nodes[key]

	if ok {
		return Response{GET, key, node.Value, node.Value, true, node.ExpireTime}
	} else {
		return Response{GET, key, "", "", false, time.Unix(0, 0)}
	}
}

// delete the key, return the old value if the key exists
func Delete(key string) ([]byte, error) {
	key = path.Clean(key)

	node, ok := s.Nodes[key]

	if ok {
		delete(s.Nodes, key)

		notify(DELETE, key, node.Value, "", true)

		msg, err := json.Marshal(Response{DELETE, key, node.Value, "", true, node.ExpireTime})

		// notify the web interface
		if (s.messager != nil && err == nil) {

			*s.messager <- string(msg)
		} 

		return msg, err

	} else {
		// no notify to the watcher and web interface

		return json.Marshal(Response{DELETE, key, "", "", false, time.Unix(0, 0)})
	}
}

// save the current state of the storage system
func (s *Store)Save() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return b, nil
}

// recovery the state of the stroage system from a previous state
func (s *Store)Recovery(state []byte) error {
	err := json.Unmarshal(state, s)
	clean()
	return err
}

// clean all expired keys
func clean() {
	for key, node := range s.Nodes{
		// stable node
		if node.ExpireTime.Equal(time.Unix(0,0)) {
			continue
		} else {
			if node.ExpireTime.Sub(time.Now()) >= time.Second {
				node.update = make(chan time.Time)
				go expire(key, node.update, node.ExpireTime)
			} else {
				// we should delete this node
				delete(s.Nodes, key)
			}
		}

	}
}
