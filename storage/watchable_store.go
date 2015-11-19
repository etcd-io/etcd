// Copyright 2015 CoreOS, Inc.
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

package storage

import (
	"sync"
	"time"

	"github.com/coreos/etcd/storage/storagepb"
)

const (
	// chanBufLen is the length of the buffered chan
	// for sending out watched events.
	// TODO: find a good buf value. 1024 is just a random one that
	// seems to be reasonable.
	chanBufLen = 1024
)

type watchable interface {
	watch(key []byte, prefix bool, startRev int64, ch chan<- storagepb.Event) (*watching, CancelFunc)
}

type watchableStore struct {
	mu sync.Mutex

	*store

	// contains all unsynced watching that needs to sync events that have happened
	unsynced map[*watching]struct{}

	// contains all synced watching that are tracking the events that will happen
	// The key of the map is the key that the watching is watching on.
	synced map[string][]*watching

	stopc chan struct{}
	wg    sync.WaitGroup
}

func newWatchableStore(path string) *watchableStore {
	s := &watchableStore{
		store:    newStore(path),
		unsynced: make(map[*watching]struct{}),
		synced:   make(map[string][]*watching),
		stopc:    make(chan struct{}),
	}
	s.wg.Add(1)
	go s.syncWatchingsLoop()
	return s
}

func (s *watchableStore) Put(key, value []byte) (rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rev = s.store.Put(key, value)
	s.handle(rev, s.store.LatestTxnEvents())
	return rev
}

func (s *watchableStore) DeleteRange(key, end []byte) (n, rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, rev = s.store.DeleteRange(key, end)
	s.handle(rev, s.store.LatestTxnEvents())
	return n, rev
}

func (s *watchableStore) TxnBegin() int64 {
	s.mu.Lock()
	return s.store.TxnBegin()
}

func (s *watchableStore) TxnEnd(txnID int64) error {
	err := s.store.TxnEnd(txnID)
	if err != nil {
		return err
	}

	s.handle(s.store.Rev(), s.store.LatestTxnEvents())
	s.mu.Unlock()
	return nil
}

func (s *watchableStore) Close() error {
	close(s.stopc)
	s.wg.Wait()
	return s.store.Close()
}

func (s *watchableStore) NewWatcher() Watcher {
	watcherGauge.Inc()
	return &watcher{
		watchable: s,
		ch:        make(chan storagepb.Event, chanBufLen),
	}
}

func (s *watchableStore) watch(key []byte, prefix bool, startRev int64, ch chan<- storagepb.Event) (*watching, CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wa := &watching{
		key:    key,
		prefix: prefix,
		cur:    startRev,
		ch:     ch,
	}

	k := string(key)
	if startRev == 0 {
		s.synced[k] = append(s.synced[k], wa)
	} else {
		slowWatchingGauge.Inc()
		s.unsynced[wa] = struct{}{}
	}
	watchingGauge.Inc()

	cancel := CancelFunc(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		// remove global references of the watching
		if _, ok := s.unsynced[wa]; ok {
			delete(s.unsynced, wa)
			slowWatchingGauge.Dec()
			watchingGauge.Dec()
			return
		}

		for i, w := range s.synced[k] {
			if w == wa {
				s.synced[k] = append(s.synced[k][:i], s.synced[k][i+1:]...)
				watchingGauge.Dec()
			}
		}
		// If we cannot find it, it should have finished watch.
	})

	return wa, cancel
}

// syncWatchingsLoop syncs the watching in the unsyncd map every 100ms.
func (s *watchableStore) syncWatchingsLoop() {
	defer s.wg.Done()

	for {
		s.mu.Lock()
		s.syncWatchings()
		s.mu.Unlock()

		select {
		case <-time.After(100 * time.Millisecond):
		case <-s.stopc:
			return
		}
	}
}

// syncWatchings syncs the watchings in the unsyncd map.
func (s *watchableStore) syncWatchings() {
	_, curRev, _ := s.store.Range(nil, nil, 0, 0)
	for w := range s.unsynced {
		var end []byte
		if w.prefix {
			end = make([]byte, len(w.key))
			copy(end, w.key)
			end[len(w.key)-1]++
		}
		limit := cap(w.ch) - len(w.ch)
		// the channel is full, try it in the next round
		if limit == 0 {
			continue
		}
		revbs, kvs, nextRev, err := s.store.RangeHistory(w.key, end, int64(limit), w.cur)
		if err != nil {
			// TODO: send error event to watching
			delete(s.unsynced, w)
			continue
		}

		// push events to the channel
		for i, kv := range kvs {
			var evt storagepb.Event_EventType
			switch {
			case isTombstone(revbs[i]):
				evt = storagepb.DELETE
			default:
				evt = storagepb.PUT
			}

			w.ch <- storagepb.Event{
				Type: evt,
				Kv:   &kv,
			}
			pendingEventsGauge.Inc()
		}
		// switch to tracking future events if needed
		if nextRev > curRev {
			s.synced[string(w.key)] = append(s.synced[string(w.key)], w)
			delete(s.unsynced, w)
			continue
		}
		// put it back to try it in the next round
		w.cur = nextRev
	}
	slowWatchingGauge.Set(float64(len(s.unsynced)))
}

// handle handles the change of the happening event on all watchings.
func (s *watchableStore) handle(rev int64, evs []storagepb.Event) {
	for i, ev := range evs {
		var modified bool // whether the key is modified in left events
		for j := i + 1; j < len(evs); j++ {
			if string(ev.Kv.Key) == string(evs[j].Kv.Key) {
				modified = true
			}
		}
		// notify the event only if the event key is not modified
		// any more in this txn
		if !modified {
			s.notify(rev, ev)
		}
	}
}

// notify notifies the fact that given event at the given rev just happened to
// watchings that watch on the key of the event.
func (s *watchableStore) notify(rev int64, ev storagepb.Event) {
	// check all prefixes of the key to notify all corresponded watchings
	for i := 0; i <= len(ev.Kv.Key); i++ {
		ws := s.synced[string(ev.Kv.Key[:i])]
		nws := ws[:0]
		for _, w := range ws {
			// the watching needs to be notified when either it watches prefix or
			// the key is exactly matched.
			if !w.prefix && i != len(ev.Kv.Key) {
				continue
			}
			select {
			case w.ch <- ev:
				pendingEventsGauge.Inc()
				nws = append(nws, w)
			default:
				w.cur = rev
				s.unsynced[w] = struct{}{}
				slowWatchingGauge.Inc()
			}
		}
		s.synced[string(ev.Kv.Key[:i])] = nws
	}
}

type watching struct {
	// the watching key
	key []byte
	// prefix indicates if watching is on a key or a prefix.
	// If prefix is true, the watching is on a prefix.
	prefix bool
	// cur is the current watching revision.
	// If cur is behind the current revision of the KV,
	// watching is unsynced and needs to catch up.
	cur int64

	// a chan to send out the watched events.
	// The chan might be shared with other watchings.
	ch chan<- storagepb.Event
}
