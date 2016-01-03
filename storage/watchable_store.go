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
	"fmt"
	"log"
	"math"
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
	watch(key []byte, prefix bool, startRev, id int64, ch chan<- []storagepb.Event) (*watcher, CancelFunc)
}

type watchableStore struct {
	mu sync.Mutex

	*store

	// contains all unsynced watchers that needs to sync with events that have happened
	unsynced map[*watcher]struct{}

	// contains all synced watchers that are in sync with the progress of the store.
	// The key of the map is the key that the watcher watches on.
	synced map[string]map[*watcher]struct{}
	tx     *ongoingTx

	stopc chan struct{}
	wg    sync.WaitGroup
}

func newWatchableStore(path string) *watchableStore {
	s := &watchableStore{
		store:    newDefaultStore(path),
		unsynced: make(map[*watcher]struct{}),
		synced:   make(map[string]map[*watcher]struct{}),
		stopc:    make(chan struct{}),
	}
	s.wg.Add(1)
	go s.syncWatchersLoop()
	return s
}

func (s *watchableStore) Put(key, value []byte) (rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rev = s.store.Put(key, value)
	// TODO: avoid this range
	kvs, _, err := s.store.Range(key, nil, 0, rev)
	if err != nil {
		log.Panicf("unexpected range error (%v)", err)
	}
	ev := storagepb.Event{
		Type: storagepb.PUT,
		Kv:   &kvs[0],
	}
	s.handle(rev, []storagepb.Event{ev})
	return rev
}

func (s *watchableStore) DeleteRange(key, end []byte) (n, rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: avoid this range
	kvs, _, err := s.store.Range(key, end, 0, 0)
	if err != nil {
		log.Panicf("unexpected range error (%v)", err)
	}
	n, rev = s.store.DeleteRange(key, end)
	evs := make([]storagepb.Event, len(kvs))
	for i, kv := range kvs {
		evs[i] = storagepb.Event{
			Type: storagepb.DELETE,
			Kv: &storagepb.KeyValue{
				Key: kv.Key,
			}}
	}
	s.handle(rev, evs)
	return n, rev
}

func (s *watchableStore) TxnBegin() int64 {
	s.mu.Lock()
	s.tx = newOngoingTx()
	return s.store.TxnBegin()
}

func (s *watchableStore) TxnPut(txnID int64, key, value []byte) (rev int64, err error) {
	rev, err = s.store.TxnPut(txnID, key, value)
	if err == nil {
		s.tx.put(string(key))
	}
	return rev, err
}

func (s *watchableStore) TxnDeleteRange(txnID int64, key, end []byte) (n, rev int64, err error) {
	kvs, _, err := s.store.TxnRange(txnID, key, end, 0, 0)
	if err != nil {
		log.Panicf("unexpected range error (%v)", err)
	}
	n, rev, err = s.store.TxnDeleteRange(txnID, key, end)
	if err == nil {
		for _, kv := range kvs {
			s.tx.del(string(kv.Key))
		}
	}
	return n, rev, err
}

func (s *watchableStore) TxnEnd(txnID int64) error {
	err := s.store.TxnEnd(txnID)
	if err != nil {
		return err
	}

	_, rev, _ := s.store.Range(nil, nil, 0, 0)

	evs := []storagepb.Event{}

	for k := range s.tx.putm {
		kvs, _, err := s.store.Range([]byte(k), nil, 0, 0)
		if err != nil {
			log.Panicf("unexpected range error (%v)", err)
		}
		ev := storagepb.Event{
			Type: storagepb.PUT,
			Kv:   &kvs[0],
		}
		evs = append(evs, ev)
	}

	for k := range s.tx.delm {
		ev := storagepb.Event{
			Type: storagepb.DELETE,
			Kv: &storagepb.KeyValue{
				Key: []byte(k),
			},
		}
		evs = append(evs, ev)
	}

	s.handle(rev, evs)

	s.mu.Unlock()
	return nil
}

func (s *watchableStore) Close() error {
	close(s.stopc)
	s.wg.Wait()
	return s.store.Close()
}

func (s *watchableStore) NewWatchStream() WatchStream {
	watchStreamGauge.Inc()
	return &watchStream{
		watchable: s,
		ch:        make(chan []storagepb.Event, chanBufLen),
		cancels:   make(map[int64]CancelFunc),
	}
}

func (s *watchableStore) watch(key []byte, prefix bool, startRev, id int64, ch chan<- []storagepb.Event) (*watcher, CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wa := &watcher{
		key:    key,
		prefix: prefix,
		cur:    startRev,
		id:     id,
		ch:     ch,
	}

	k := string(key)
	if startRev == 0 {
		if err := unsafeAddWatcher(&s.synced, k, wa); err != nil {
			log.Panicf("error unsafeAddWatcher (%v) for key %s", err, k)
		}
	} else {
		slowWatcherGauge.Inc()
		s.unsynced[wa] = struct{}{}
	}
	watcherGauge.Inc()

	cancel := CancelFunc(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		// remove global references of the watcher
		if _, ok := s.unsynced[wa]; ok {
			delete(s.unsynced, wa)
			slowWatcherGauge.Dec()
			watcherGauge.Dec()
			return
		}

		if v, ok := s.synced[k]; ok {
			if _, ok := v[wa]; ok {
				delete(v, wa)
				// if there is nothing in s.synced[k],
				// remove the key from the synced
				if len(v) == 0 {
					delete(s.synced, k)
				}
				watcherGauge.Dec()
			}
		}
		// If we cannot find it, it should have finished watch.
	})

	return wa, cancel
}

// syncWatchersLoop syncs the watcher in the unsyncd map every 100ms.
func (s *watchableStore) syncWatchersLoop() {
	defer s.wg.Done()

	for {
		s.mu.Lock()
		s.syncWatchers()
		s.mu.Unlock()

		select {
		case <-time.After(100 * time.Millisecond):
		case <-s.stopc:
			return
		}
	}
}

// syncWatchers periodically syncs unsynced watchers by: Iterate all unsynced
// watchers to get the minimum revision within its range, skipping the
// watcher if its current revision is behind the compact revision of the
// store. And use this minimum revision to get all key-value pairs. Then send
// those events to watchers.
func (s *watchableStore) syncWatchers() {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	if len(s.unsynced) == 0 {
		return
	}

	// in order to find key-value pairs from unsynced watchers, we need to
	// find min revision index, and these revisions can be used to
	// query the backend store of key-value pairs
	minRev := int64(math.MaxInt64)

	curRev := s.store.currentRev.main
	compactionRev := s.store.compactMainRev

	// TODO: change unsynced struct type same to this
	keyToUnsynced := make(map[string]map[*watcher]struct{})

	for w := range s.unsynced {
		k := string(w.key)

		if w.cur > curRev {
			panic("watcher current revision should not exceed current revision")
		}

		if w.cur < compactionRev {
			// TODO: return error compacted to that watcher instead of
			// just removing it sliently from unsynced.
			delete(s.unsynced, w)
			continue
		}

		if minRev >= w.cur {
			minRev = w.cur
		}

		if _, ok := keyToUnsynced[k]; !ok {
			keyToUnsynced[k] = make(map[*watcher]struct{})
		}
		keyToUnsynced[k][w] = struct{}{}
	}

	minBytes, maxBytes := newRevBytes(), newRevBytes()
	revToBytes(revision{main: minRev}, minBytes)
	revToBytes(revision{main: curRev + 1}, maxBytes)

	// UnsafeRange returns keys and values. And in boltdb, keys are revisions.
	// values are actual key-value pairs in backend.
	tx := s.store.b.BatchTx()
	tx.Lock()
	ks, vs := tx.UnsafeRange(keyBucketName, minBytes, maxBytes, 0)
	tx.Unlock()

	evs := []storagepb.Event{}

	// get the list of all events from all key-value pairs
	for i, v := range vs {
		var kv storagepb.KeyValue
		if err := kv.Unmarshal(v); err != nil {
			log.Panicf("storage: cannot unmarshal event: %v", err)
		}

		k := string(kv.Key)
		if _, ok := keyToUnsynced[k]; !ok {
			continue
		}

		var ev storagepb.Event
		switch {
		case isTombstone(ks[i]):
			ev.Type = storagepb.DELETE
		default:
			ev.Type = storagepb.PUT
		}
		ev.Kv = &kv

		evs = append(evs, ev)
	}

	for w, es := range newWatcherToEventMap(keyToUnsynced, evs) {
		select {
		case w.ch <- es:
			pendingEventsGauge.Add(float64(len(es)))
		default:
			// TODO: handle the full unsynced watchers.
			// continue to process other watchers for now, the full ones
			// will be processed next time and hopefully it will not be full.
			continue
		}
		k := string(w.key)
		if err := unsafeAddWatcher(&s.synced, k, w); err != nil {
			log.Panicf("error unsafeAddWatcher (%v) for key %s", err, k)
		}
		delete(s.unsynced, w)
	}

	slowWatcherGauge.Set(float64(len(s.unsynced)))
}

// handle handles the change of the happening event on all watchers.
func (s *watchableStore) handle(rev int64, evs []storagepb.Event) {
	s.notify(rev, evs)
}

// notify notifies the fact that given event at the given rev just happened to
// watchers that watch on the key of the event.
func (s *watchableStore) notify(rev int64, evs []storagepb.Event) {
	we := newWatcherToEventMap(s.synced, evs)
	for _, wm := range s.synced {
		for w := range wm {
			if _, ok := we[w]; !ok {
				continue
			}
			es := we[w]
			select {
			case w.ch <- es:
				pendingEventsGauge.Add(float64(len(es)))
			default:
				// move slow watcher to unsynced
				w.cur = rev
				s.unsynced[w] = struct{}{}
				delete(wm, w)
				slowWatcherGauge.Inc()
			}
		}
	}
}

type ongoingTx struct {
	// keys put/deleted in the ongoing txn
	putm map[string]struct{}
	delm map[string]struct{}
}

func newOngoingTx() *ongoingTx {
	return &ongoingTx{
		putm: make(map[string]struct{}),
		delm: make(map[string]struct{}),
	}
}

func (tx *ongoingTx) put(k string) {
	tx.putm[k] = struct{}{}
	if _, ok := tx.delm[k]; ok {
		delete(tx.delm, k)
	}
}

func (tx *ongoingTx) del(k string) {
	tx.delm[k] = struct{}{}
	if _, ok := tx.putm[k]; ok {
		delete(tx.putm, k)
	}
}

type watcher struct {
	// the watcher key
	key []byte
	// prefix indicates if watcher is on a key or a prefix.
	// If prefix is true, the watcher is on a prefix.
	prefix bool
	// cur is the current watcher revision.
	// If cur is behind the current revision of the KV,
	// watcher is unsynced and needs to catch up.
	cur int64
	id  int64

	// a chan to send out the watched events.
	// The chan might be shared with other watchers.
	ch chan<- []storagepb.Event
}

// unsafeAddWatcher puts watcher with key k into watchableStore's synced.
// Make sure to this is thread-safe using mutex before and after.
func unsafeAddWatcher(synced *map[string]map[*watcher]struct{}, k string, wa *watcher) error {
	if wa == nil {
		return fmt.Errorf("nil watcher received")
	}
	mp := *synced
	if v, ok := mp[k]; ok {
		if _, ok := v[wa]; ok {
			return fmt.Errorf("put the same watcher twice: %+v", wa)
		} else {
			v[wa] = struct{}{}
		}
		return nil
	}

	mp[k] = make(map[*watcher]struct{})
	mp[k][wa] = struct{}{}
	return nil
}

// newWatcherToEventMap creates a map that has watcher as key and events as
// value. It enables quick events look up by watcher.
func newWatcherToEventMap(sm map[string]map[*watcher]struct{}, evs []storagepb.Event) map[*watcher][]storagepb.Event {
	watcherToEvents := make(map[*watcher][]storagepb.Event)
	for _, ev := range evs {
		key := string(ev.Kv.Key)

		// check all prefixes of the key to notify all corresponded watchers
		for i := 0; i <= len(key); i++ {
			k := string(key[:i])

			wm, ok := sm[k]
			if !ok {
				continue
			}

			for w := range wm {
				// the watcher needs to be notified when either it watches prefix or
				// the key is exactly matched.
				if !w.prefix && i != len(ev.Kv.Key) {
					continue
				}
				ev.WatchID = w.id

				if _, ok := watcherToEvents[w]; !ok {
					watcherToEvents[w] = []storagepb.Event{}
				}
				watcherToEvents[w] = append(watcherToEvents[w], ev)
			}
		}
	}

	return watcherToEvents
}
