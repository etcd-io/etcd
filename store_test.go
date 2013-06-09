package main

import (
	"testing"
)

func TestStoreGet(t *testing.T) {

	s.Set("foo", "bar")

	value, err := s.Get("foo")

	if err!= nil || value != "bar" {
		t.Fatalf("Cannot get stored value")
	}

	s.Delete("foo")
	value, err = s.Get("foo")

	if err == nil{
		t.Fatalf("Got deleted value")
	}
}

func TestSaveAndRecovery(t *testing.T) {

	s.Set("foo", "bar")

	state, err := s.Save()

	if err != nil {
		t.Fatalf("Cannot Save")
	}

	newStore := createStore()
	newStore.Recovery(state)

	value, err := newStore.Get("foo")

	if err!= nil || value != "bar" {
		t.Fatalf("Cannot recovery")
	}

}
