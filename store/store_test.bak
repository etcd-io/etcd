package store

import (
	"fmt"
	"testing"
	"time"
)

func TestStoreGet(t *testing.T) {

	Set("foo", "bar", time.Unix(0, 0))

	res := Get("foo")

	if res.NewValue != "bar" {
		t.Fatalf("Cannot get stored value")
	}

	Delete("foo")
	res = Get("foo")

	if res.Exist {
		t.Fatalf("Got deleted value")
	}
}

// func TestSaveAndRecovery(t *testing.T) {

// 	Set("foo", "bar", time.Unix(0, 0))
// 	Set("foo2", "bar2", time.Now().Add(time.Second * 5))
// 	state, err := s.Save()

// 	if err != nil {
// 		t.Fatalf("Cannot Save")
// 	}

// 	newStore := createStore()

// 	// wait for foo2 expires
// 	time.Sleep(time.Second * 6)

// 	newStore.Recovery(state)

// 	res := newStore.Get("foo")

// 	if res.OldValue != "bar" {
// 		t.Fatalf("Cannot recovery")
// 	}

// 	res = newStore.Get("foo2")

// 	if res.Exist {
// 		t.Fatalf("Get expired value")
// 	}

// 	s.Delete("foo")

// }

func TestExpire(t *testing.T) {
	fmt.Println(time.Now())
	fmt.Println("TEST EXPIRE")

	// test expire
	Set("foo", "bar", time.Now().Add(time.Second*1))
	time.Sleep(2 * time.Second)

	res := Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}

	//test change expire time
	Set("foo", "bar", time.Now().Add(time.Second*10))

	res = Get("foo")

	if !res.Exist {
		t.Fatalf("Cannot get Value")
	}

	Set("foo", "barbar", time.Now().Add(time.Second*1))

	time.Sleep(2 * time.Second)

	res = Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}

	// test change expire to stable
	Set("foo", "bar", time.Now().Add(time.Second*1))

	Set("foo", "bar", time.Unix(0, 0))

	time.Sleep(2 * time.Second)

	res = s.Get("foo")

	if !res.Exist {
		t.Fatalf("Cannot get Value")
	}

	// test stable to expire
	s.Set("foo", "bar", time.Now().Add(time.Second*1))
	time.Sleep(2 * time.Second)
	res = s.Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}

	// test set older node
	s.Set("foo", "bar", time.Now().Add(-time.Second*1))
	res = s.Get("foo")

	if res.Exist {
		t.Fatalf("Got expired value")
	}

}
