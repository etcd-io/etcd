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
	"errors"
	"fmt"
	"sync"

	"github.com/google/btree"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrNotReady = fmt.Errorf("cache: store not ready")

// The store keeps a bounded history of snapshots using ringBuffer so that
// reads at historical revisions can be served until they fall out of the window.
type store struct {
	mu      sync.RWMutex
	degree  int
	latest  snapshot              // latest is the mutable working snapshot
	history ringBuffer[*snapshot] // history stores immutable cloned snapshots
}

func newStore(degree int, historyCapacity int) *store {
	tree := btree.New(degree)
	return &store{
		degree:  degree,
		latest:  snapshot{rev: 0, tree: tree},
		history: *newRingBuffer(historyCapacity, func(s *snapshot) int64 { return s.rev }),
	}
}

type kvItem struct {
	key string
	kv  *mvccpb.KeyValue
}

func newKVItem(kv *mvccpb.KeyValue) *kvItem {
	return &kvItem{key: string(kv.Key), kv: kv}
}

func (a *kvItem) Less(b btree.Item) bool {
	return a.key < b.(*kvItem).key
}

func (s *store) Get(startKey, endKey []byte, rev int64) ([]*mvccpb.KeyValue, int64, error) {
	snapshot, _, err := s.getSnapshot(rev)
	if err != nil {
		return nil, 0, err
	}
	return snapshot.Range(startKey, endKey), rev, nil
}

func (s *store) getSnapshot(rev int64) (*snapshot, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest.rev == 0 {
		return nil, 0, ErrNotReady
	}
	if rev < 0 {
		return nil, 0, fmt.Errorf("invalid revision: %d", rev)
	}
	if rev == 0 {
		rev = s.latest.rev
	}
	if rev > s.latest.rev {
		return nil, 0, rpctypes.ErrFutureRev
	}
	oldestRev := s.history.PeekOldest()
	if rev < oldestRev {
		return nil, 0, rpctypes.ErrCompacted
	}

	var targetSnapshot *snapshot
	s.history.AscendGreaterOrEqual(rev, func(rev int64, snap *snapshot) bool {
		targetSnapshot = snap
		return false
	})
	// If s.history < rev < s.latest.rev serve latest.
	if targetSnapshot == nil {
		targetSnapshot = &s.latest
	}

	return targetSnapshot, s.latest.rev, nil
}

// Restore replaces state with the bootstrap snapshot and resets history.
func (s *store) Restore(kvs []*mvccpb.KeyValue, rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.latest.tree = btree.New(s.degree)
	// fmt.Printf("CACHE %p restore rev: %d\n", s, rev)
	for _, kv := range kvs {
		// fmt.Printf("CACHE %p restore key: %+v\n", s, kv)
		s.latest.tree.ReplaceOrInsert(newKVItem(kv))
	}

	s.history.RebaseHistory()
	s.latest.rev = rev
	s.history.Append(newClonedSnapshot(rev, s.latest.tree))
}

func (s *store) Apply(resp clientv3.WatchResponse) error {
	if resp.Canceled {
		return errors.New("canceled")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateRevisions(resp, s.latest.rev); err != nil {
		return err
	}

	switch {
	case resp.IsProgressNotify():
		s.applyProgressNotifyLocked(resp.Header.Revision)
		return nil
	case len(resp.Events) != 0:
		return s.applyEventsLocked(resp.Events)
	default:
		return nil
	}
}

func (s *store) applyProgressNotifyLocked(revision int64) {
	if s.latest.rev == 0 {
		return
	}
	s.latest.rev = revision
}

func (s *store) applyEventsLocked(events []*clientv3.Event) error {
	for i := 0; i < len(events); {
		rev := events[i].Kv.ModRevision

		for i < len(events) && events[i].Kv.ModRevision == rev {
			ev := events[i]
			// fmt.Printf("CACHE %p apply event: %+v\n", s, ev)
			switch ev.Type {
			case clientv3.EventTypeDelete:
				if removed := s.latest.tree.Delete(&kvItem{key: string(ev.Kv.Key)}); removed == nil {
					return fmt.Errorf("cache: delete non-existent key %s", string(ev.Kv.Key))
				}
			case clientv3.EventTypePut:
				s.latest.tree.ReplaceOrInsert(newKVItem(ev.Kv))
			}
			i++
		}
		s.latest.rev = rev
		s.history.Append(newClonedSnapshot(rev, s.latest.tree))
	}
	return nil
}

func (s *store) LatestRev() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest.rev
}

func validateRevisions(resp clientv3.WatchResponse, latestRev int64) error {
	if resp.IsProgressNotify() {
		if resp.Header.Revision < latestRev {
			return fmt.Errorf("cache: progress notification out of order (progress %d < latest %d)", resp.Header.Revision, latestRev)
		}
		return nil
	}
	events := resp.Events
	if len(events) == 0 {
		return nil
	}
	for _, ev := range events {
		r := ev.Kv.ModRevision
		if r < latestRev {
			return fmt.Errorf("cache: stale event batch (rev %d < latest %d)", r, latestRev)
		}
		if r == latestRev {
			return fmt.Errorf("cache: duplicate revision batch breaks atomic guarantee (rev %d == latest %d)", r, latestRev)
		}
	}
	return nil
}
