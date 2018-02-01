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

	"github.com/coreos/etcd/internal/raftsnap"
)

// a key-value store backed by raft
type kvstore struct {
	proposeC    chan<- string // channel for proposing updates
	mu          sync.RWMutex
	kvStore     map[string]string // current committed key-value pairs
	snapshotter *raftsnap.Snapshotter
}

type kv struct {
	Key string
	Val string
}

func newKVStore(snapshotter *raftsnap.Snapshotter, proposeC chan<- string, commitC <-chan *string, errorC <-chan error) *kvstore {
	s := &kvstore{proposeC: proposeC, kvStore: make(map[string]string), snapshotter: snapshotter}
	// must wait synchronously until replay has done then can we start listen new commits
	s.replayCommits(commitC, errorC)
	// read commits from raft into kvStore map until error
	go s.readCommits(commitC, errorC)
	return s
}

func (s *kvstore) Lookup(key string) (string, bool) {
	s.mu.RLock()
	v, ok := s.kvStore[key]
	s.mu.RUnlock()
	return v, ok
}

func (s *kvstore) Propose(k string, v string) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(kv{k, v}); err != nil {
		log.Fatal(err)
	}
	s.proposeC <- buf.String()
}

func (s *kvstore) replayCommits(commitC <-chan *string, errorC <-chan error) {
	for {
		select {
		case data := <- commitC:
			if data == nil {
				// replaying log has done
				snapshot, err := s.snapshotter.Load()
				if err == raftsnap.ErrNoSnapshot {
					return
				}
				if err != nil && err != raftsnap.ErrNoSnapshot {
					log.Panic(err)
				}
				log.Printf("loading snapshot at term %d and index %d", snapshot.Metadata.Term, snapshot.Metadata.Index)
				// restore from snapshot rather than overwrite kvStore with snapshot
				if err := s.restoreFromSnapshot(snapshot.Data); err != nil {
					log.Panic(err)
				}
				return
			}

			s.saveCommit(*data)
		case err := <-errorC:
			log.Fatal(err)
		}
	}
}

func (s *kvstore) readCommits(commitC <-chan *string, errorC <-chan error) {
	for {
		select {
		case data := <- commitC:
			if data == nil {
				// signaled to load snapshot
				snapshot, err := s.snapshotter.Load()
				if err == raftsnap.ErrNoSnapshot {
					return
				}
				if err != nil && err != raftsnap.ErrNoSnapshot {
					log.Panic(err)
				}
				log.Printf("loading snapshot at term %d and index %d", snapshot.Metadata.Term, snapshot.Metadata.Index)
				// overwrite entire kvStore with snapshot received from leader
				if err := s.recoverFromSnapshot(snapshot.Data); err != nil {
					log.Panic(err)
				}
				continue
			}

			s.saveCommit(*data)
		case err := <-errorC:
			log.Fatal(err)
		}
	}
}

func (s *kvstore) saveCommit(data string) {
	var dataKv kv
	dec := gob.NewDecoder(bytes.NewBufferString(data))
	if err := dec.Decode(&dataKv); err != nil {
		log.Fatalf("raftexample: could not decode message (%v)", err)
	}
	s.mu.Lock()
	s.kvStore[dataKv.Key] = dataKv.Val
	s.mu.Unlock()
}

func (s *kvstore) getSnapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Marshal(s.kvStore)
}

func (s *kvstore) restoreFromSnapshot(snapshot []byte) error {
	var store map[string]string
	if err := json.Unmarshal(snapshot, &store); err != nil {
		return err
	}
	s.mu.Lock()
	for key, value := range store {
		s.kvStore[key] = value
	}
	s.mu.Unlock()
	return nil
}

func (s *kvstore) recoverFromSnapshot(snapshot []byte) error {
	var store map[string]string
	if err := json.Unmarshal(snapshot, &store); err != nil {
		return err
	}
	s.mu.Lock()
	s.kvStore = store
	s.mu.Unlock()
	return nil
}
