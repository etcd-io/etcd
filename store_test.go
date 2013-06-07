package raftd

import (
	"testing"
)

func TestStoreGet(t *testing.T) {
	store := createStore()

	store.Set("foo", "bar")

	value, err := store.Get("foo")

	if err!= nil || value != "bar" {
		t.Fatalf("Cannot get stored value")
	}

	store.Delete("foo")
	value, err = store.Get("foo")

	if err == nil{
		t.Fatalf("Got deleted value")
	}
}

func TestSaveAndRecovery(t *testing.T) {
	store := createStore()

	store.Set("foo", "bar")

	state, err := store.Save()

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
