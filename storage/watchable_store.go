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
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/storage/backend"
	"github.com/coreos/etcd/storage/storagepb"
)

const (
	// chanBufLen is the length of the buffered chan
	// for sending out watched events.
	// TODO: find a good buf value. 1024 is just a random one that
	// seems to be reasonable.
	chanBufLen = 1024
)

var (
	// watchBatchMaxRevs is the maximum distinct revisions that
	// may be sent to an unsynced watcher at a time. Declared as
	// var instead of const for testing purposes.
	watchBatchMaxRevs = 1000
)

type eventBatch struct {
	// evs is a batch of revision-ordered events
	evs []storagepb.Event
	// revs is the minimum unique revisions observed for this batch
	revs int
	// moreRev is first revision with more events following this batch
	moreRev int64
}

type (
	watcherSetByKey map[string]watcherSet
	watcherSet      map[*watcher]struct{}
	watcherBatch    map[*watcher]*eventBatch
)

func (eb *eventBatch) add(ev storagepb.Event) {
	if eb.revs > watchBatchMaxRevs {
		// maxed out batch size
		return
	}

	if len(eb.evs) == 0 {
		// base case
		eb.revs = 1
		eb.evs = append(eb.evs, ev)
		return
	}

	// revision accounting
	ebRev := eb.evs[len(eb.evs)-1].Kv.ModRevision
	evRev := ev.Kv.ModRevision
	if evRev > ebRev {
		eb.revs++
		if eb.revs > watchBatchMaxRevs {
			eb.moreRev = evRev
			return
		}
	}

	eb.evs = append(eb.evs, ev)
}

func (wb watcherBatch) add(w *watcher, ev storagepb.Event) {
	eb := wb[w]
	if eb == nil {
		eb = &eventBatch{}
		wb[w] = eb
	}
	eb.add(ev)
}

func (w watcherSet) add(wa *watcher) {
	if _, ok := w[wa]; ok {
		panic("add watcher twice!")
	}
	w[wa] = struct{}{}
}

func (w watcherSetByKey) add(wa *watcher) {
	set := w[string(wa.key)]
	if set == nil {
		set = make(watcherSet)
		w[string(wa.key)] = set
	}
	set.add(wa)
}

func (w watcherSetByKey) getSetByKey(key string) (watcherSet, bool) {
	set, ok := w[key]
	return set, ok
}

func (w watcherSetByKey) delete(wa *watcher) bool {
	k := string(wa.key)
	if v, ok := w[k]; ok {
		if _, ok := v[wa]; ok {
			delete(v, wa)
			// if there is nothing in the set,
			// remove the set
			if len(v) == 0 {
				delete(w, k)
			}
			return true
		}
	}
	return false
}

type watchable interface {
	watch(key []byte, prefix bool, startRev int64, id WatchID, ch chan<- WatchResponse) (*watcher, cancelFunc)
	rev() int64
}

type watchableStore struct {
	mu sync.Mutex

	*store

	// contains all unsynced watchers that needs to sync with events that have happened
	unsynced watcherSetByKey

	// contains all synced watchers that are in sync with the progress of the store.
	// The key of the map is the key that the watcher watches on.
	synced watcherSetByKey

	stopc chan struct{}
	wg    sync.WaitGroup
}

// cancelFunc updates unsynced and synced maps when running
// cancel operations.
type cancelFunc func()

func newWatchableStore(b backend.Backend, le lease.Lessor) *watchableStore {
	s := &watchableStore{
		store:    NewStore(b, le),
		unsynced: make(watcherSetByKey),
		synced:   make(watcherSetByKey),
		stopc:    make(chan struct{}),
	}
	if s.le != nil {
		// use this store as the deleter so revokes trigger watch events
		s.le.SetRangeDeleter(s)
	}
	s.wg.Add(1)
	go s.syncWatchersLoop()
	return s
}

func (s *watchableStore) Put(key, value []byte, lease lease.LeaseID) (rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rev = s.store.Put(key, value, lease)
	changes := s.store.getChanges()
	if len(changes) != 1 {
		log.Panicf("unexpected len(changes) != 1 after put")
	}

	ev := storagepb.Event{
		Type: storagepb.PUT,
		Kv:   &changes[0],
	}
	s.notify(rev, []storagepb.Event{ev})
	return rev
}

func (s *watchableStore) DeleteRange(key, end []byte) (n, rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, rev = s.store.DeleteRange(key, end)
	changes := s.store.getChanges()

	if len(changes) != int(n) {
		log.Panicf("unexpected len(changes) != n after deleteRange")
	}

	if n == 0 {
		return n, rev
	}

	evs := make([]storagepb.Event, n)
	for i, change := range changes {
		evs[i] = storagepb.Event{
			Type: storagepb.DELETE,
			Kv:   &change}
		evs[i].Kv.ModRevision = rev
	}
	s.notify(rev, evs)
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

	changes := s.getChanges()
	if len(changes) == 0 {
		s.mu.Unlock()
		return nil
	}

	rev := s.store.Rev()
	evs := make([]storagepb.Event, len(changes))
	for i, change := range changes {
		switch change.Value {
		case nil:
			evs[i] = storagepb.Event{
				Type: storagepb.DELETE,
				Kv:   &changes[i]}
			evs[i].Kv.ModRevision = rev
		default:
			evs[i] = storagepb.Event{
				Type: storagepb.PUT,
				Kv:   &changes[i]}
		}
	}

	s.notify(rev, evs)
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
		ch:        make(chan WatchResponse, chanBufLen),
		cancels:   make(map[WatchID]cancelFunc),
	}
}

func (s *watchableStore) watch(key []byte, prefix bool, startRev int64, id WatchID, ch chan<- WatchResponse) (*watcher, cancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wa := &watcher{
		key:    key,
		prefix: prefix,
		cur:    startRev,
		id:     id,
		ch:     ch,
	}

	s.store.mu.Lock()
	synced := startRev > s.store.currentRev.main || startRev == 0
	if synced {
		wa.cur = s.store.currentRev.main + 1
	}
	s.store.mu.Unlock()
	if synced {
		if startRev > wa.cur {
			panic("can't watch past sync revision")
		}
		s.synced.add(wa)
	} else {
		slowWatcherGauge.Inc()
		s.unsynced.add(wa)
	}
	watcherGauge.Inc()

	cancel := cancelFunc(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		// remove references of the watcher
		if s.unsynced.delete(wa) {
			slowWatcherGauge.Dec()
			watcherGauge.Dec()
			return
		}

		if s.synced.delete(wa) {
			watcherGauge.Dec()
		}
		// If we cannot find it, it should have finished watch.
	})

	return wa, cancel
}

// syncWatchersLoop syncs the watcher in the unsynced map every 100ms.
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
	prefixes, minRev := s.scanUnsync()
	curRev := s.store.currentRev.main
	minBytes, maxBytes := newRevBytes(), newRevBytes()
	revToBytes(revision{main: minRev}, minBytes)
	revToBytes(revision{main: curRev + 1}, maxBytes)

	// UnsafeRange returns keys and values. And in boltdb, keys are revisions.
	// values are actual key-value pairs in backend.
	tx := s.store.b.BatchTx()
	tx.Lock()
	revs, vs := tx.UnsafeRange(keyBucketName, minBytes, maxBytes, 0)
	evs := kvsToEvents(revs, vs, s.unsynced, prefixes)
	tx.Unlock()

	for w, eb := range newWatcherBatch(s.unsynced, evs) {
		select {
		// s.store.Rev also uses Lock, so just return directly
		case w.ch <- WatchResponse{WatchID: w.id, Events: eb.evs, Revision: s.store.currentRev.main}:
			pendingEventsGauge.Add(float64(len(eb.evs)))
		default:
			// TODO: handle the full unsynced watchers.
			// continue to process other watchers for now, the full ones
			// will be processed next time and hopefully it will not be full.
			continue
		}
		if eb.moreRev != 0 {
			w.cur = eb.moreRev
			continue
		}
		w.cur = curRev
		s.synced.add(w)
		s.unsynced.delete(w)
	}

	slowWatcherGauge.Set(float64(len(s.unsynced)))
}

func (s *watchableStore) scanUnsync() (prefixes map[string]struct{}, minRev int64) {
	curRev := s.store.currentRev.main
	compactionRev := s.store.compactMainRev

	prefixes = make(map[string]struct{})
	minRev = int64(math.MaxInt64)
	for _, set := range s.unsynced {
		for w := range set {
			k := string(w.key)

			if w.cur > curRev {
				panic("watcher current revision should not exceed current revision")
			}

			if w.cur < compactionRev {
				select {
				case w.ch <- WatchResponse{WatchID: w.id, CompactRevision: compactionRev}:
					s.unsynced.delete(w)
				default:
					// retry next time
				}
				continue
			}

			if minRev > w.cur {
				minRev = w.cur
			}

			if w.prefix {
				prefixes[k] = struct{}{}
			}
		}
	}

	return prefixes, minRev
}

// kvsToEvents gets all events for the watchers from all key-value pairs
func kvsToEvents(revs, vals [][]byte, wsk watcherSetByKey, pfxs map[string]struct{}) (evs []storagepb.Event) {
	for i, v := range vals {
		var kv storagepb.KeyValue
		if err := kv.Unmarshal(v); err != nil {
			log.Panicf("storage: cannot unmarshal event: %v", err)
		}

		k := string(kv.Key)
		if _, ok := wsk.getSetByKey(k); !ok && !matchPrefix(k, pfxs) {
			continue
		}

		ty := storagepb.PUT
		if isTombstone(revs[i]) {
			ty = storagepb.DELETE
			// patch in mod revision so watchers won't skip
			kv.ModRevision = bytesToRev(revs[i]).main
		}
		evs = append(evs, storagepb.Event{Kv: &kv, Type: ty})
	}
	return evs
}

// notify notifies the fact that given event at the given rev just happened to
// watchers that watch on the key of the event.
func (s *watchableStore) notify(rev int64, evs []storagepb.Event) {
	we := newWatcherBatch(s.synced, evs)
	for _, wm := range s.synced {
		for w := range wm {
			eb, ok := we[w]
			if !ok {
				continue
			}
			if eb.revs != 1 {
				panic("unexpected multiple revisions in notification")
			}
			select {
			case w.ch <- WatchResponse{WatchID: w.id, Events: eb.evs, Revision: s.Rev()}:
				pendingEventsGauge.Add(float64(len(eb.evs)))
			default:
				// move slow watcher to unsynced
				w.cur = rev
				s.unsynced.add(w)
				delete(wm, w)
				slowWatcherGauge.Inc()
			}
		}
	}
}

func (s *watchableStore) rev() int64 { return s.store.Rev() }

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
	id  WatchID

	// a chan to send out the watch response.
	// The chan might be shared with other watchers.
	ch chan<- WatchResponse
}

// newWatcherBatch maps watchers to their matched events. It enables quick
// events look up by watcher.
func newWatcherBatch(sm watcherSetByKey, evs []storagepb.Event) watcherBatch {
	wb := make(watcherBatch)
	for _, ev := range evs {
		key := string(ev.Kv.Key)

		// check all prefixes of the key to notify all corresponded watchers
		for i := 0; i <= len(key); i++ {
			for w := range sm[key[:i]] {
				// don't double notify
				if ev.Kv.ModRevision < w.cur {
					continue
				}

				// the watcher needs to be notified when either it watches prefix or
				// the key is exactly matched.
				if !w.prefix && i != len(ev.Kv.Key) {
					continue
				}
				wb.add(w, ev)
			}
		}
	}

	return wb
}

// matchPrefix returns true if key has any matching prefix
// from prefixes map.
func matchPrefix(key string, prefixes map[string]struct{}) bool {
	for p := range prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}
