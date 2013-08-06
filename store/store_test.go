package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStoreGetDelete(t *testing.T) {

	s := CreateStore(100)
	s.Set("foo", "bar", time.Unix(0, 0), 1)
	res, err := s.Get("foo")

	if err != nil {
		t.Fatalf("Unknown error")
	}

	var result Response
	json.Unmarshal(res, &result)

	if result.Value != "bar" {
		t.Fatalf("Cannot get stored value")
	}

	s.Delete("foo", 2)
	_, err = s.Get("foo")

	if err == nil {
		t.Fatalf("Got deleted value")
	}
}

func TestSaveAndRecovery(t *testing.T) {

	s := CreateStore(100)
	s.Set("foo", "bar", time.Unix(0, 0), 1)
	s.Set("foo2", "bar2", time.Now().Add(time.Second*5), 2)
	state, err := s.Save()

	if err != nil {
		t.Fatalf("Cannot Save %s", err)
	}

	newStore := CreateStore(100)

	// wait for foo2 expires
	time.Sleep(time.Second * 6)

	newStore.Recovery(state)

	res, err := newStore.Get("foo")

	var result Response
	json.Unmarshal(res, &result)

	if result.Value != "bar" {
		t.Fatalf("Recovery Fail")
	}

	res, err = newStore.Get("foo2")

	if err == nil {
		t.Fatalf("Get expired value")
	}

	s.Delete("foo", 3)

}

func TestExpire(t *testing.T) {
	// test expire
	s := CreateStore(100)
	s.Set("foo", "bar", time.Now().Add(time.Second*1), 0)
	time.Sleep(2 * time.Second)

	_, err := s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

	//test change expire time
	s.Set("foo", "bar", time.Now().Add(time.Second*10), 1)

	_, err = s.Get("foo")

	if err != nil {
		t.Fatalf("Cannot get Value")
	}

	s.Set("foo", "barbar", time.Now().Add(time.Second*1), 2)

	time.Sleep(2 * time.Second)

	_, err = s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

	// test change expire to stable
	s.Set("foo", "bar", time.Now().Add(time.Second*1), 3)

	s.Set("foo", "bar", time.Unix(0, 0), 4)

	time.Sleep(2 * time.Second)

	_, err = s.Get("foo")

	if err != nil {
		t.Fatalf("Cannot get Value")
	}

	// test stable to expire
	s.Set("foo", "bar", time.Now().Add(time.Second*1), 5)
	time.Sleep(2 * time.Second)
	_, err = s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

	// test set older node
	s.Set("foo", "bar", time.Now().Add(-time.Second*1), 6)
	_, err = s.Get("foo")

	if err == nil {
		t.Fatalf("Got expired value")
	}

}

func BenchmarkStoreSet(b *testing.B) {
	s := CreateStore(100)

	keys := GenKeys(10000, 5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {

		for i, key := range keys {
			s.Set(key, "barbarbarbarbar", time.Unix(0, 0), uint64(i))
		}

		s = CreateStore(100)
	}
}

func BenchmarkStoreGet(b *testing.B) {
	s := CreateStore(100)

	keys := GenKeys(10000, 5)

	for i, key := range keys {
		s.Set(key, "barbarbarbarbar", time.Unix(0, 0), uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {

		for _, key := range keys {
			s.Get(key)
		}

	}
}

func BenchmarkStoreSnapshotCopy(b *testing.B) {
	s := CreateStore(100)

	keys := GenKeys(10000, 5)

	for i, key := range keys {
		s.Set(key, "barbarbarbarbar", time.Unix(0, 0), uint64(i))
	}

	var state []byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.clone()
	}
	b.SetBytes(int64(len(state)))
}

func BenchmarkSnapshotSaveJson(b *testing.B) {
	s := CreateStore(100)

	keys := GenKeys(10000, 5)

	for i, key := range keys {
		s.Set(key, "barbarbarbarbar", time.Unix(0, 0), uint64(i))
	}

	var state []byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state, _ = s.Save()
	}
	b.SetBytes(int64(len(state)))
}

func BenchmarkSnapshotRecovery(b *testing.B) {
	s := CreateStore(100)

	keys := GenKeys(10000, 5)

	for i, key := range keys {
		s.Set(key, "barbarbarbarbar", time.Unix(0, 0), uint64(i))
	}

	state, _ := s.Save()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newStore := CreateStore(100)
		newStore.Recovery(state)
	}
	b.SetBytes(int64(len(state)))
}
