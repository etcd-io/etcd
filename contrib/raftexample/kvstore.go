// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// a key-value store backed by raft
type kvstore struct {
	proposeC chan<- string // channel for proposing updates
	mu       sync.RWMutex
	kvStore  map[string]string // current committed key-value pairs
}

type kv struct {
	Key string
	Val string
}

// newKVStore creates and returns a new `kvstore`. The second return
// value can be used as the finite state machine that is driven by a
// `raftNode`.
func newKVStore(proposeC chan<- string) (*kvstore, kvfsm) {
	s := &kvstore{
		proposeC: proposeC,
		kvStore:  make(map[string]string),
	}
	fsm := kvfsm{
		kvs: s,
	}
	return s, fsm
}

func (s *kvstore) Lookup(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.kvStore[key]
	return v, ok
}

func (s *kvstore) Propose(k string, v string) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(kv{k, v}); err != nil {
		log.Fatal(err)
	}
	s.proposeC <- buf.String()
}

// Set sets a single value. It should only be called by `kvfsm`.
func (s *kvstore) set(k, v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvStore[k] = v
}

func (s *kvstore) restoreFromSnapshot(store map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvStore = store
}

// kvfsm implements the `FSM` interface for the underlying `*kvstore`.
type kvfsm struct {
	kvs *kvstore
}

// RestoreSnapshot restores the current state of the KV store to the
// value encoded in `snapshot`.
func (fsm kvfsm) RestoreSnapshot(snapshot []byte) error {
	var store map[string]string
	if err := json.Unmarshal(snapshot, &store); err != nil {
		return err
	}
	fsm.kvs.restoreFromSnapshot(store)
	return nil
}

func (fsm kvfsm) TakeSnapshot() ([]byte, error) {
	fsm.kvs.mu.RLock()
	defer fsm.kvs.mu.RUnlock()
	return json.Marshal(fsm.kvs.kvStore)
}

// ApplyCommits decodes and applies each of the commits in `commit` to
// the current state, then signals that it is done by closing
// `commit.applyDoneC`.
func (fsm kvfsm) ApplyCommits(commit *commit) error {
	for _, data := range commit.data {
		var dataKv kv
		dec := gob.NewDecoder(bytes.NewBufferString(data))
		if err := dec.Decode(&dataKv); err != nil {
			return fmt.Errorf("could not decode message: %w", err)
		}
		fsm.kvs.set(dataKv.Key, dataKv.Val)
	}
	close(commit.applyDoneC)
	return nil
}
