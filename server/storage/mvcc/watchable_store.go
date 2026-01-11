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

package mvcc

import (
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/verify"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// non-const so modifiable by tests
var (
	// chanBufLen is the length of the buffered chan
	// for sending out watched events.
	// See https://github.com/etcd-io/etcd/issues/11906 for more detail.
	chanBufLen = 128

	// maxWatchersPerSync is the number of watchers to sync in a single batch
	maxWatchersPerSync = 512

	// maxResyncPeriod is the period of executing resync.
	watchResyncPeriod = 100 * time.Millisecond
)

func ChanBufLen() int { return chanBufLen }

type watchable interface {
	watch(key, end []byte, startRev int64, id WatchID, ch chan<- WatchResponse, fcs ...FilterFunc) (*watcher, cancelFunc)
	progress(w *watcher)
	progressAll(watchers map[WatchID]*watcher) bool
	rev() int64
}

type watchableStore struct {
	*store

	// mu protects watcher groups and batches. It should never be locked
	// before locking store.mu to avoid deadlock.
	mu sync.RWMutex

	// unsynced contains all watchers that need to catch up with the store's progress.
	// This includes:
	// - Watchers with nil eventBatch: need to fetch events from DB
	// - Watchers with non-nil eventBatch: have events ready to retry sending
	unsynced watcherGroup

	// synced contains all watchers that are in sync with the progress of the store.
	// Watchers in synced always have nil eventBatch.
	synced watcherGroup

	// retryc signals when there are watchers with pending events that need retry.
	retryc chan struct{}

	stopc chan struct{}
	wg    sync.WaitGroup
}

var _ WatchableKV = (*watchableStore)(nil)

// cancelFunc updates unsynced and synced maps when running
// cancel operations.
type cancelFunc func()

func New(lg *zap.Logger, b backend.Backend, le lease.Lessor, cfg StoreConfig) WatchableKV {
	s := newWatchableStore(lg, b, le, cfg)
	s.wg.Add(2)
	go s.syncWatchersLoop()
	go s.retryLoop()
	return s
}

func newWatchableStore(lg *zap.Logger, b backend.Backend, le lease.Lessor, cfg StoreConfig) *watchableStore {
	if lg == nil {
		lg = zap.NewNop()
	}
	s := &watchableStore{
		store:    NewStore(lg, b, le, cfg),
		retryc:   make(chan struct{}, 1),
		unsynced: newWatcherGroup(),
		synced:   newWatcherGroup(),
		stopc:    make(chan struct{}),
	}
	s.store.ReadView = &readView{s}
	s.store.WriteView = &writeView{s}
	if s.le != nil {
		// use this store as the deleter so revokes trigger watch events
		s.le.SetRangeDeleter(func() lease.TxnDelete { return s.Write(traceutil.TODO()) })
	}
	return s
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
		watchers:  make(map[WatchID]*watcher),
	}
}

func (s *watchableStore) watch(key, end []byte, startRev int64, id WatchID, ch chan<- WatchResponse, fcs ...FilterFunc) (*watcher, cancelFunc) {
	wa := &watcher{
		key:      key,
		end:      end,
		startRev: startRev,
		minRev:   startRev,
		id:       id,
		ch:       ch,
		fcs:      fcs,
	}

	s.mu.Lock()
	s.revMu.RLock()
	synced := startRev > s.store.currentRev || startRev == 0
	if synced {
		wa.minRev = s.store.currentRev + 1
		if startRev > wa.minRev {
			wa.minRev = startRev
		}
		s.synced.add(wa)
	} else {
		slowWatcherGauge.Inc()
		s.unsynced.add(wa)
	}
	s.revMu.RUnlock()
	s.mu.Unlock()

	watcherGauge.Inc()

	return wa, func() { s.cancelWatcher(wa) }
}

// cancelWatcher removes references of the watcher from the watchableStore
func (s *watchableStore) cancelWatcher(wa *watcher) {
	// Check if already canceled first to handle double-cancel scenarios
	// (e.g., Cancel and Close racing on the same watcher)
	if wa.ch == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.unsynced.delete(wa) {
		slowWatcherGauge.Dec()
		watcherGauge.Dec()
	} else if s.synced.delete(wa) {
		watcherGauge.Dec()
	} else if wa.compacted {
		// Watcher was compacted and already removed from unsynced,
		// but we wait until now to decrement gauges.
		slowWatcherGauge.Dec()
		watcherGauge.Dec()
	} else {
		// This should never happen (it would indicate a bug in the watcher management)
		panic(fmt.Errorf("watcher (ID: %d key: %s) to cancel was not found", wa.id, wa.key))
	}

	wa.ch = nil
}

func (s *watchableStore) Restore(b backend.Backend) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.store.Restore(b)
	if err != nil {
		return err
	}

	for wa := range s.synced.watchers {
		wa.restore = true
		s.unsynced.add(wa)
	}
	s.synced = newWatcherGroup()
	return nil
}

// syncWatchersLoop syncs the watcher in the unsynced map every 100ms.
func (s *watchableStore) syncWatchersLoop() {
	defer s.wg.Done()

	delayTicker := time.NewTicker(watchResyncPeriod)
	defer delayTicker.Stop()
	var evs []mvccpb.Event

	for {
		s.mu.RLock()
		st := time.Now()
		lastUnsyncedWatchers := s.unsynced.size()
		s.mu.RUnlock()

		unsyncedWatchers := 0
		if lastUnsyncedWatchers > 0 {
			unsyncedWatchers, evs = s.syncWatchers(evs)
		}
		syncDuration := time.Since(st)

		delayTicker.Reset(watchResyncPeriod)
		// more work pending?
		if unsyncedWatchers != 0 && lastUnsyncedWatchers > unsyncedWatchers {
			// be fair to other store operations by yielding time taken
			delayTicker.Reset(syncDuration)
		}

		select {
		case <-delayTicker.C:
		case <-s.stopc:
			return
		}
	}
}

// retryLoop retries sending events to watchers that have pending event batches.
// These are watchers in unsynced group with non-nil eventBatch.
func (s *watchableStore) retryLoop() {
	defer s.wg.Done()

	for {
		for s.retryPendingEvents() != 0 {
			// try to update all victim watchers
		}

		var tickc <-chan time.Time
		if s.hasPendingEvents() {
			tickc = time.After(10 * time.Millisecond)
		}

		select {
		case <-tickc:
		case <-s.retryc:
		case <-s.stopc:
			return
		}
	}
}

// retryPendingEvents tries to send events to watchers that have pending event batches.
// Returns the number of watchers whose pending events were successfully sent.
func (s *watchableStore) retryPendingEvents() (sent int) {
	// This function holds s.mu lock during the entire processing loop.
	// This is acceptable because w.send() is non-blocking.
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store.revMu.RLock()
	curRev := s.store.currentRev
	s.store.revMu.RUnlock()

	for w, eb := range s.unsynced.watchers {
		if eb == nil {
			continue
		}

		// Try to send the pending events
		rev := w.minRev - 1
		if !w.send(WatchResponse{WatchID: w.id, Events: eb.evs, Revision: rev}) {
			// Failed to send, stay as unsynced
			continue
		}
		pendingEventsGauge.Add(float64(len(eb.evs)))
		sent++

		// Update minRev if there are more events beyond this batch.
		// If moreRev == 0, minRev is already correct (set by syncWatchers/notify)
		if eb.moreRev != 0 {
			w.minRev = eb.moreRev
		}

		// Clear pending events and determine next state
		if w.minRev <= curRev {
			// Still behind - stay in unsynced but clear eventBatch for DB fetch
			s.unsynced.watchers[w] = nil
		} else {
			// Caught up - move to synced
			s.unsynced.delete(w)
			s.synced.add(w)
			slowWatcherGauge.Dec()
		}
	}
	return sent
}

func (s *watchableStore) hasPendingEvents() bool {
	// Check if there are still watchers with pending events
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, eb := range s.unsynced.watchers {
		if eb != nil {
			return true
		}
	}
	return false
}

// syncWatchers syncs unsynced watchers by:
//  1. choose a set of watchers (with nil eventBatch) from the unsynced watcher group.
//     Note: those with watch events will be handled by retryLoop to resend responses.
//  2. iterate over the set to get the minimum revision and remove compacted watchers
//  3. use minimum revision to get all key-value pairs and send those events to watchers
//  4. remove synced watchers in set from unsynced group and move to synced group
func (s *watchableStore) syncWatchers(evs []mvccpb.Event) (int, []mvccpb.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.unsynced.size() == 0 {
		return 0, []mvccpb.Event{}
	}

	s.store.revMu.RLock()
	defer s.store.revMu.RUnlock()

	// in order to find key-value pairs from unsynced watchers, we need to
	// find min revision index, and these revisions can be used to
	// query the backend store of key-value pairs
	curRev := s.store.currentRev
	compactionRev := s.store.compactMainRev

	// Select a batch of watchers that need DB fetch
	wg, minRev := s.selectForSync(maxWatchersPerSync, curRev, compactionRev)
	if wg == nil {
		return s.unsynced.size(), evs
	}
	evs = rangeEventsWithReuse(s.store.lg, s.store.b, evs, minRev, curRev+1)

	// Match events to watchers
	wb := newWatcherBatch(wg, evs)

	var hasPendingEvents bool
	for w := range wg.watchers {
		if w.minRev < compactionRev {
			// Skip the watcher that failed to send compacted watch response due to w.ch is full.
			// Next retry of syncWatchers would try to resend the compacted watch response to w.ch
			continue
		}
		w.minRev = max(curRev+1, w.minRev)

		eb, ok := wb[w]
		if !ok {
			// bring un-notified watcher to synced
			s.synced.add(w)
			s.unsynced.delete(w)
			continue
		}

		if eb.moreRev != 0 {
			w.minRev = eb.moreRev
		}

		if w.send(WatchResponse{WatchID: w.id, Events: eb.evs, Revision: curRev}) {
			pendingEventsGauge.Add(float64(len(eb.evs)))
			if eb.moreRev == 0 {
				s.synced.add(w)
				s.unsynced.delete(w)
			}
		} else {
			s.unsynced.watchers[w] = eb
			hasPendingEvents = true
		}
	}

	if hasPendingEvents {
		s.signalRetryLoop()
	}

	slowWatcherGauge.Set(float64(s.unsynced.size()))
	return s.unsynced.size(), evs
}

// selectForSync selects up to maxWatchers from unsynced group that need to sync from DB.
// It skips watchers with pending events (non-nil eventBatch) as they are handled by retryLoop.
// It also handles compacted watchers by sending compaction responses and removing them.
// Returns the selected watchers and the minimum revision needed for DB fetch.
func (s *watchableStore) selectForSync(maxWatchers int, curRev, compactRev int64) (*watcherGroup, int64) {
	if s.unsynced.size() == 0 {
		return nil, int64(math.MaxInt64)
	}

	minRev := int64(math.MaxInt64)
	ret := newWatcherGroup()
	for w, eb := range s.unsynced.watchers {
		// Skip watchers with pending events - they are handled by retryLoop
		if eb != nil {
			continue
		}
		if w.minRev > curRev {
			// Watchers moved from synced to unsynced during Restore() may have future revisions.
			// They will be included but won't match any events, then moved back to synced.
			if !w.restore {
				panic(fmt.Errorf("watcher minimum revision %d should not exceed current revision %d", w.minRev, curRev))
			}
			w.restore = false
		}
		if w.minRev < compactRev {
			select {
			case w.ch <- WatchResponse{WatchID: w.id, CompactRevision: compactRev}:
				w.compacted = true
				s.unsynced.delete(w)
				// Gauge is decremented in cancelWatcher when client cancels
			default:
				// retry next time
			}
			continue
		}
		if minRev > w.minRev {
			minRev = w.minRev
		}
		ret.add(w)
		if ret.size() >= maxWatchers {
			break
		}
	}
	if ret.size() == 0 {
		return nil, minRev
	}
	return &ret, minRev
}

// rangeEventsWithReuse returns events in range [minRev, maxRev), while reusing already provided events.
func rangeEventsWithReuse(lg *zap.Logger, b backend.Backend, evs []mvccpb.Event, minRev, maxRev int64) []mvccpb.Event {
	if len(evs) == 0 {
		return rangeEvents(lg, b, minRev, maxRev)
	}
	// append from left
	if evs[0].Kv.ModRevision > minRev {
		evs = append(rangeEvents(lg, b, minRev, evs[0].Kv.ModRevision), evs...)
	}
	// cut from left
	prefixIndex := 0
	for prefixIndex < len(evs) && evs[prefixIndex].Kv.ModRevision < minRev {
		prefixIndex++
	}
	evs = evs[prefixIndex:]

	if len(evs) == 0 {
		return rangeEvents(lg, b, minRev, maxRev)
	}
	// append from right
	if evs[len(evs)-1].Kv.ModRevision+1 < maxRev {
		evs = append(evs, rangeEvents(lg, b, evs[len(evs)-1].Kv.ModRevision+1, maxRev)...)
	}
	// cut from right
	suffixIndex := len(evs) - 1
	for suffixIndex >= 0 && evs[suffixIndex].Kv.ModRevision >= maxRev {
		suffixIndex--
	}
	evs = evs[:suffixIndex+1]
	return evs
}

// rangeEvents returns events in range [minRev, maxRev).
func rangeEvents(lg *zap.Logger, b backend.Backend, minRev, maxRev int64) []mvccpb.Event {
	if minRev < 0 {
		lg.Warn("Unexpected negative revision range start", zap.Int64("minRev", minRev))
		minRev = 0
	}
	minBytes, maxBytes := NewRevBytes(), NewRevBytes()
	minBytes = RevToBytes(Revision{Main: minRev}, minBytes)
	maxBytes = RevToBytes(Revision{Main: maxRev}, maxBytes)

	// UnsafeRange returns keys and values. And in boltdb, keys are revisions.
	// values are actual key-value pairs in backend.
	tx := b.ReadTx()
	tx.RLock()
	revs, vals := tx.UnsafeRange(schema.Key, minBytes, maxBytes, 0)
	evs := kvsToEvents(lg, revs, vals)
	// Must unlock after kvsToEvents, because vals (come from boltdb memory) is not deep copy.
	// We can only unlock after Unmarshal, which will do deep copy.
	// Otherwise we will trigger SIGSEGV during boltdb re-mmap.
	tx.RUnlock()
	return evs
}

// kvsToEvents gets all events for the watchers from all key-value pairs
func kvsToEvents(lg *zap.Logger, revs, vals [][]byte) (evs []mvccpb.Event) {
	for i, v := range vals {
		var kv mvccpb.KeyValue
		if err := kv.Unmarshal(v); err != nil {
			lg.Panic("failed to unmarshal mvccpb.KeyValue", zap.Error(err))
		}

		ty := mvccpb.PUT
		if isTombstone(revs[i]) {
			ty = mvccpb.DELETE
			// patch in mod revision so watchers won't skip
			kv.ModRevision = BytesToRev(revs[i]).Main
		}
		evs = append(evs, mvccpb.Event{Kv: &kv, Type: ty})
	}
	return evs
}

// notify notifies the fact that given event at the given rev just happened to
// watchers that watch on the key of the event.
func (s *watchableStore) notify(rev int64, evs []mvccpb.Event) {
	var hasPendingEvents bool
	for w, eb := range newWatcherBatch(&s.synced, evs) {
		if eb.revs != 1 {
			s.store.lg.Panic(
				"unexpected multiple revisions in watch notification",
				zap.Int("number-of-revisions", eb.revs),
			)
		}
		if w.send(WatchResponse{WatchID: w.id, Events: eb.evs, Revision: rev}) {
			pendingEventsGauge.Add(float64(len(eb.evs)))
		} else {
			// Move slow watcher to unsynced with its pending events.
			// The retryLoop will retry sending these events.
			s.synced.delete(w)
			s.unsynced.addWithEventBatch(w, eb)
			slowWatcherGauge.Inc()
			hasPendingEvents = true
		}
		// always update minRev
		// in case 'send' returns true and watcher stays synced, this is needed for Restore when all watchers become unsynced
		// in case 'send' returns false, this is needed for retryLoop
		w.minRev = rev + 1
	}
	if hasPendingEvents {
		s.signalRetryLoop()
	}
}

// signalRetryLoop signals the retryLoop that there are watchers with pending events.
func (s *watchableStore) signalRetryLoop() {
	select {
	case s.retryc <- struct{}{}:
	default:
	}
}

func (s *watchableStore) rev() int64 { return s.store.Rev() }

func (s *watchableStore) progress(w *watcher) {
	s.progressIfSync(map[WatchID]*watcher{w.id: w}, w.id)
}

func (s *watchableStore) progressAll(watchers map[WatchID]*watcher) bool {
	return s.progressIfSync(watchers, clientv3.InvalidWatchID)
}

func (s *watchableStore) progressIfSync(watchers map[WatchID]*watcher, responseWatchID WatchID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rev := s.rev()
	// Any watcher unsynced?
	for _, w := range watchers {
		if _, ok := s.synced.watchers[w]; !ok {
			return false
		}
		if rev < w.startRev {
			return false
		}
	}

	// If all watchers are synchronised, send out progress
	// notification on first watcher. Note that all watchers
	// should have the same underlying stream, and the progress
	// notification will be broadcasted client-side if required
	// (see dispatchEvent in client/v3/watch.go)
	for _, w := range watchers {
		w.send(WatchResponse{WatchID: responseWatchID, Revision: rev})
		return true
	}
	return true
}

type watcher struct {
	// the watcher key
	key []byte
	// end indicates the end of the range to watch.
	// If end is set, the watcher is on a range.
	end []byte

	// compacted is set when the compaction response is sent.
	// The watcher is removed from the watcher group but the gauge is not decremented
	// until cancelWatcher is called.
	compacted bool

	// restore is true when the watcher is being restored from leader snapshot
	// which means that this watcher has just been moved from "synced" to "unsynced"
	// watcher group, possibly with a future revision when it was first added
	// to the synced watcher
	// "unsynced" watcher revision must always be <= current revision,
	// except when the watcher were to be moved from "synced" watcher group
	restore bool

	startRev int64
	// minRev is the minimum revision update the watcher will accept
	minRev int64
	id     WatchID

	fcs []FilterFunc
	// a chan to send out the watch response.
	// The chan might be shared with other watchers.
	ch chan<- WatchResponse
}

// send puts the watch response into the stream's shared channel.
// This must finish quickly, otherwise we risk blocking the DB store
// as a PUT operation involves sending the event to subscribed watchers.
// And retryPendingEvents also requires this to be non-blocking.
func (w *watcher) send(wr WatchResponse) bool {
	progressEvent := len(wr.Events) == 0

	if len(w.fcs) != 0 {
		ne := make([]mvccpb.Event, 0, len(wr.Events))
		for i := range wr.Events {
			filtered := false
			for _, filter := range w.fcs {
				if filter(wr.Events[i]) {
					filtered = true
					break
				}
			}
			if !filtered {
				ne = append(ne, wr.Events[i])
			}
		}
		wr.Events = ne
	}

	verify.Verify("Event.ModRevision is less than the w.startRev for watchID", func() (bool, map[string]any) {
		if w.startRev > 0 {
			for _, ev := range wr.Events {
				if ev.Kv.ModRevision < w.startRev {
					return false, map[string]any{
						"Event.ModRevision": ev.Kv.ModRevision,
						"w.startRev":        w.startRev,
						"watchID":           w.id,
					}
				}
			}
		}
		return true, nil
	})

	// if all events are filtered out, we should send nothing.
	if !progressEvent && len(wr.Events) == 0 {
		return true
	}
	select {
	case w.ch <- wr:
		return true
	default:
		return false
	}
}
