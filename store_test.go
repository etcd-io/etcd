package main

import (
	"testing"
	"time"
	"fmt"
)

func TestStoreGet(t *testing.T) {

	s.Set("foo", "bar", time.Unix(0, 0))

	res := s.Get("foo")

	if res.NewValue != "bar" {
		t.Fatalf("Cannot get stored value")
	}

	s.Delete("foo")
	res = s.Get("foo")

	if res.Exist {
		t.Fatalf("Got deleted value")
	}
}

func TestSaveAndRecovery(t *testing.T) {

	s.Set("foo", "bar", time.Unix(0, 0))

	state, err := s.Save()

	if err != nil {
		t.Fatalf("Cannot Save")
	}

	newStore := createStore()
	newStore.Recovery(state)

	res := newStore.Get("foo")

	if res.OldValue != "bar" {
		t.Fatalf("Cannot recovery")
	}
	s.Delete("foo")

}

func TestExpire(t *testing.T) {
	fmt.Println(time.Now())
	fmt.Println("TEST EXPIRE")

	// test expire
	s.Set("foo", "bar", time.Now().Add(time.Second * 1))
	time.Sleep(2*time.Second)

	res := s.Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}

	//test change expire time
	s.Set("foo", "bar", time.Now().Add(time.Second * 10))

	res = s.Get("foo")

	if !res.Exist {
		t.Fatalf("Cannot get Value")
	}

	s.Set("foo", "barbar", time.Now().Add(time.Second * 1))

	time.Sleep(2 * time.Second)

	res = s.Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}


	// test change expire to stable
	s.Set("foo", "bar", time.Now().Add(time.Second * 1))

	s.Set("foo", "bar", time.Unix(0,0))

	time.Sleep(2*time.Second)

	res = s.Get("foo")

	if !res.Exist {
		t.Fatalf("Cannot get Value")
	}

	// test stable to expire 
	s.Set("foo", "bar", time.Now().Add(time.Second * 1))
	time.Sleep(2*time.Second)
	res = s.Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}

	// test set older node 
	s.Set("foo", "bar", time.Now().Add(-time.Second * 1))
	res = s.Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}


}
