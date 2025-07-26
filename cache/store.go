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

type store struct {
	mu        sync.RWMutex
	kvs       map[string]*mvccpb.KeyValue
	latestRev int64
}

func (s *store) Get(startKey, endKey []byte) ([]*mvccpb.KeyValue, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(endKey) == 0 { // fast path: single key lookup
		if kv, ok := s.kvs[string(startKey)]; ok {
			return []*mvccpb.KeyValue{kv}, s.latestRev
		}
		return nil, s.latestRev
	}

	var out []*mvccpb.KeyValue
	for _, kv := range s.kvs { // range / prefix scan
		k := kv.Key
		switch {
		case len(endKey) == 1 && endKey[0] == 0: // prefix & fromKey
			if bytes.Compare(k, startKey) >= 0 {
				out = append(out, kv)
			}
		default: // [startKey, endKey)
			if bytes.Compare(k, startKey) >= 0 && bytes.Compare(k, endKey) < 0 {
				out = append(out, kv)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Key, out[j].Key) < 0 // default: lexicographical, ascending‐by‐key sort
	})
	return out, s.latestRev
}

// Reset purges all in-memory state when the upstream watch stream reports compaction and we have to rebuild the cache from scratch.
func (s *store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvs = make(map[string]*mvccpb.KeyValue)
	s.latestRev = 0
}

func newStore() *store {
	return &store{kvs: make(map[string]*mvccpb.KeyValue)}
}

func (s *store) apply(events []*clientv3.Event) error {
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
