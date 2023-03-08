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
	"log"
	"sync"

	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/raft/v3/raftpb"
)

// a key-value store backed by raft
type kvstore struct {
	proposeC        chan<- string // channel for proposing updates
	mu              sync.RWMutex
	kvStore         map[string]string // current committed key-value pairs
	snapshotStorage SnapshotStorage
}

type kv struct {
	Key string
	Val string
}

// newKVStore creates and returns a new `kvstore` and loads and
// applies the most recent snapshot (if any are available). The caller
// must call `processCommits()` on the return value (normally, in a
// goroutine) to make it start apply commits from raft to the state.
func newKVStore(snapshotStorage SnapshotStorage, proposeC chan<- string) *kvstore {
	s := &kvstore{
		proposeC:        proposeC,
		kvStore:         make(map[string]string),
		snapshotStorage: snapshotStorage,
	}
	s.loadAndApplySnapshot()
	return s
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

// processCommits() reads commits from `commitC` and applies them into
// the kvStore map until that channel is closed.
func (s *kvstore) processCommits(commitC <-chan *commit, errorC <-chan error) {
	for commit := range commitC {
		if commit == nil {
			s.loadAndApplySnapshot()
			continue
		}

		s.applyCommits(commit)
	}
	if err, ok := <-errorC; ok {
		log.Fatal(err)
	}
}

// loadAndApplySnapshot load the most recent snapshot from the
// snapshot storage and applies it to the current state.
func (s *kvstore) loadAndApplySnapshot() {
	// signaled to load snapshot
	snapshot, err := s.loadSnapshot()
	if err != nil {
		log.Panic(err)
	}
	if snapshot != nil {
		log.Printf("loading snapshot at term %d and index %d", snapshot.Metadata.Term, snapshot.Metadata.Index)
		if err := s.recoverFromSnapshot(snapshot.Data); err != nil {
			log.Panic(err)
		}
	}
}

// applyCommits decodes and applies each of the commits in `commit` to
// the current state, then signals that it is done by closing
// `commit.applyDoneC`.
func (s *kvstore) applyCommits(commit *commit) {
	for _, data := range commit.data {
		var dataKv kv
		dec := gob.NewDecoder(bytes.NewBufferString(data))
		if err := dec.Decode(&dataKv); err != nil {
			log.Fatalf("raftexample: could not decode message (%v)", err)
		}
		s.mu.Lock()
		s.kvStore[dataKv.Key] = dataKv.Val
		s.mu.Unlock()
	}
	close(commit.applyDoneC)
}

func (s *kvstore) getSnapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.kvStore)
}

func (s *kvstore) loadSnapshot() (*raftpb.Snapshot, error) {
	snapshot, err := s.snapshotStorage.Load()
	if err == snap.ErrNoSnapshot {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *kvstore) recoverFromSnapshot(snapshot []byte) error {
	var store map[string]string
	if err := json.Unmarshal(snapshot, &store); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvStore = store
	return nil
}
