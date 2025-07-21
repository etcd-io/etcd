// Copyright 2025 The etcd Authors
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

package cache

import (
	"bytes"
	"fmt"
	"sort"
	"sync"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrNotInitialized = fmt.Errorf("cache: store not initialized")

type store struct {
	mu        sync.RWMutex
	kvs       map[string]*mvccpb.KeyValue
	latestRev int64
}

func newStore() *store {
	return &store{kvs: make(map[string]*mvccpb.KeyValue)}
}

func (s *store) Get(startKey, endKey []byte) ([]*mvccpb.KeyValue, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latestRev == 0 {
		return nil, 0, ErrNotInitialized
	}

	var out []*mvccpb.KeyValue
	switch {
	case len(endKey) == 0:
		out = s.getSingle(startKey)
	case isPrefixScan(endKey):
		out = s.scanPrefix(startKey)
	default:
		out = s.scanRange(startKey, endKey)
	}

	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Key, out[j].Key) < 0 // default: lexicographical, ascending‐by‐key sort
	})
	return out, s.latestRev, nil
}

func (s *store) Restore(kvs []*mvccpb.KeyValue, rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kvs = make(map[string]*mvccpb.KeyValue, len(kvs))
	for _, kv := range kvs {
		s.kvs[string(kv.Key)] = kv
	}
	s.latestRev = rev
}

// Reset purges all in-memory state when the upstream watch stream reports compaction.
func (s *store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvs = make(map[string]*mvccpb.KeyValue)
	s.latestRev = 0
}

func (s *store) Apply(events []*clientv3.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ev := range events {
		if ev.Kv.ModRevision < s.latestRev {
			return fmt.Errorf("cache: stale event batch (rev %d < latest %d)", ev.Kv.ModRevision, s.latestRev)
		}
	}

	for _, ev := range events {
		switch ev.Type {
		case clientv3.EventTypeDelete:
			delete(s.kvs, string(ev.Kv.Key))
		case clientv3.EventTypePut:
			s.kvs[string(ev.Kv.Key)] = ev.Kv
		}
		if ev.Kv.ModRevision > s.latestRev {
			s.latestRev = ev.Kv.ModRevision
		}
	}
	return nil
}

func (s *store) LatestRev() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestRev
}

// getSingle fetches one key or nil
func (s *store) getSingle(key []byte) []*mvccpb.KeyValue {
	if kv, ok := s.kvs[string(key)]; ok {
		return []*mvccpb.KeyValue{kv}
	}
	return nil
}

// scanPrefix returns all keys >= startKey
func (s *store) scanPrefix(startKey []byte) []*mvccpb.KeyValue {
	var res []*mvccpb.KeyValue
	for _, kv := range s.kvs {
		if bytes.Compare(kv.Key, startKey) >= 0 {
			res = append(res, kv)
		}
	}
	return res
}

// scanRange returns all keys in [startKey, endKey)
func (s *store) scanRange(startKey, endKey []byte) []*mvccpb.KeyValue {
	var res []*mvccpb.KeyValue
	for _, kv := range s.kvs {
		if bytes.Compare(kv.Key, startKey) >= 0 && bytes.Compare(kv.Key, endKey) < 0 {
			res = append(res, kv)
		}
	}
	return res
}

// isPrefixScan detects endKey=={0} semantics
func isPrefixScan(endKey []byte) bool {
	return len(endKey) == 1 && endKey[0] == 0
}
