package store

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestStoreGetDelete(t *testing.T) {

	s := CreateStore(100)
	s.Set("foo", "bar", time.Unix(0, 0), 9)
	res, err := s.Get("foo")

	if err != nil {
		t.Fatalf("Unknown error")
	}

	var result Response
	json.Unmarshal(res, &result)

	if result.Value != "bar" {
		t.Fatalf("Cannot get stored value")
	}

	s.Delete("foo", 0)
	_, err = s.Get("foo")

	if err == nil {
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
	s := CreateStore(100)
	s.Set("foo", "bar", time.Now().Add(time.Second*1), 0)
	time.Sleep(2 * time.Second)

	_, err := s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

	//test change expire time
	s.Set("foo", "bar", time.Now().Add(time.Second*10), 0)

	_, err = s.Get("foo")

	if err != nil {
		t.Fatalf("Cannot get Value")
	}

	s.Set("foo", "barbar", time.Now().Add(time.Second*1), 0)

	time.Sleep(2 * time.Second)

	_, err = s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

	// test change expire to stable
	s.Set("foo", "bar", time.Now().Add(time.Second*1), 0)

	s.Set("foo", "bar", time.Unix(0, 0), 0)

	time.Sleep(2 * time.Second)

	_, err = s.Get("foo")

	if err != nil {
		t.Fatalf("Cannot get Value")
	}

	// test stable to expire
	s.Set("foo", "bar", time.Now().Add(time.Second*1), 0)
	time.Sleep(2 * time.Second)
	_, err = s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

	// test set older node
	s.Set("foo", "bar", time.Now().Add(-time.Second*1), 0)
	_, err = s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

}
