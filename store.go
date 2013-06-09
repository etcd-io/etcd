package main

import (
	"path"
	"encoding/json"
	"fmt"
	)

// CONSTANTS
const (
	ERROR = -(1 + iota)
	SET 
	DELETE
)

type Store struct {
	Nodes map[string]string  `json:"nodes"`
}

type Response struct {
	OldValue string `json:oldvalue`
	Exist 	 bool `json:exist`
}


// global store
var s *Store

func init() {
	s = createStore()
}

// make a new stroe
func createStore() *Store{
	s := new(Store)
	s.Nodes = make(map[string]string)
	return s
}

// set the key to value, return the old value if the key exists 
func (s *Store) Set(key string, value string) Response {
	fmt.Println("Store SET")
	key = path.Clean(key)

	oldValue, ok := s.Nodes[key]

	if ok {
		s.Nodes[key] = value
		w.notify(SET, key, oldValue, value)
		return Response{oldValue, true}

	} else {
		s.Nodes[key] = value
		w.notify(SET, key, "", value)
		return Response{"", false}
	}
}

// get the value of the key
func (s *Store) Get(key string) Response {
	fmt.Println("Stroe Get")
	key = path.Clean(key)

	value, ok := s.Nodes[key]

	if ok {
		return Response{value, true}
	} else {
		return Response{"", false}
	}
}

// delete the key, return the old value if the key exists
func (s *Store) Delete(key string) Response {
	key = path.Clean(key)

	oldValue, ok := s.Nodes[key]

	if ok {
		delete(s.Nodes, key)

		w.notify(DELETE, key, oldValue, "")

		return Response{oldValue, true}
	} else {
		return Response{"", false}
	}
}

// save the current state of the storage system
func (s *Store) Save() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// recovery the state of the stroage system from a previous state
func (s *Store) Recovery(state []byte) error {
	err := json.Unmarshal(state, s)
	return err
}