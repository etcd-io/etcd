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
	"errors"
	"log"
	"sync"
	"time"

	"github.com/coreos/etcd/storage/storagepb"
)

// ReachEnd is the error returned by Watcher.Err when watcher reaches its end revision and
// no more event is available.
var ExceedEnd = errors.New("storage: watcher reaches end revision")

type watchableStore struct {
	mu sync.Mutex

	KV

	// contains all unsynced watchers that needs to sync events that have happened
	// TODO: use map to reduce cancel cost
	unsynced []*watcher
	// contains all synced watchers that are tracking the events that will happen
	// The key of the map is the key that the watcher is watching on.
	synced map[string][]*watcher
	// contains all synced watchers that have an end revision
	// The key of the map is the end revision of the watcher.
	endm map[int64][]*watcher
	tx   *ongoingTx

	stopc chan struct{}
	wg    sync.WaitGroup
}

func newWatchableStore(path string) *watchableStore {
	s := &watchableStore{
		KV:     newStore(path),
		synced: make(map[string][]*watcher),
		endm:   make(map[int64][]*watcher),
		stopc:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.syncWatchersLoop()
	return s
}

func (s *watchableStore) Put(key, value []byte) (rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rev = s.KV.Put(key, value)
	// TODO: avoid this range
	kvs, _, err := s.KV.Range(key, nil, 0, rev)
	if err != nil {
		log.Panicf("unexpected range error (%v)", err)
	}
	s.handle(rev, storagepb.Event{
		Type: storagepb.PUT,
		Kv:   &kvs[0],
	})
	return rev
}

func (s *watchableStore) DeleteRange(key, end []byte) (n, rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: avoid this range
	kvs, _, err := s.KV.Range(key, end, 0, 0)
	if err != nil {
		log.Panicf("unexpected range error (%v)", err)
	}
	n, rev = s.KV.DeleteRange(key, end)
	for _, kv := range kvs {
		s.handle(rev, storagepb.Event{
			Type: storagepb.DELETE,
			Kv: &storagepb.KeyValue{
				Key: kv.Key,
			},
		})
	}
	return n, rev
}

func (s *watchableStore) TxnBegin() int64 {
	s.mu.Lock()
	s.tx = newOngoingTx()
	return s.KV.TxnBegin()
}

func (s *watchableStore) TxnPut(txnID int64, key, value []byte) (rev int64, err error) {
	rev, err = s.KV.TxnPut(txnID, key, value)
	if err == nil {
		s.tx.put(string(key))
	}
	return rev, err
}

func (s *watchableStore) TxnDeleteRange(txnID int64, key, end []byte) (n, rev int64, err error) {
	kvs, _, err := s.KV.TxnRange(txnID, key, end, 0, 0)
	if err != nil {
		log.Panicf("unexpected range error (%v)", err)
	}
	n, rev, err = s.KV.TxnDeleteRange(txnID, key, end)
	if err == nil {
		for _, kv := range kvs {
			s.tx.del(string(kv.Key))
		}
	}
	return n, rev, err
}

func (s *watchableStore) TxnEnd(txnID int64) error {
	err := s.KV.TxnEnd(txnID)
	if err != nil {
		return err
	}

	_, rev, _ := s.KV.Range(nil, nil, 0, 0)
	for k := range s.tx.putm {
		kvs, _, err := s.KV.Range([]byte(k), nil, 0, 0)
		if err != nil {
			log.Panicf("unexpected range error (%v)", err)
		}
		s.handle(rev, storagepb.Event{
			Type: storagepb.PUT,
			Kv:   &kvs[0],
		})
	}
	for k := range s.tx.delm {
		s.handle(rev, storagepb.Event{
			Type: storagepb.DELETE,
			Kv: &storagepb.KeyValue{
				Key: []byte(k),
			},
		})
	}
	s.mu.Unlock()
	return nil
}

func (s *watchableStore) Close() error {
	close(s.stopc)
	s.wg.Wait()
	return s.KV.Close()
}

func (s *watchableStore) Watcher(key []byte, prefix bool, startRev, endRev int64) (Watcher, CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wa := newWatcher(key, prefix, startRev, endRev)
	k := string(key)
	if startRev == 0 {
		s.synced[k] = append(s.synced[k], wa)
		if endRev != 0 {
			s.endm[endRev] = append(s.endm[endRev], wa)
		}
	} else {
		slowWatchersGauge.Inc()
		s.unsynced = append(s.unsynced, wa)
	}
	watchersGauge.Inc()

	cancel := CancelFunc(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		wa.stopWithError(ErrCanceled)

		// remove global references of the watcher
		for i, w := range s.unsynced {
			if w == wa {
				s.unsynced = append(s.unsynced[:i], s.unsynced[i+1:]...)
				slowWatchersGauge.Dec()
				watchersGauge.Dec()
				return
			}
		}

		for i, w := range s.synced[k] {
			if w == wa {
				s.synced[k] = append(s.synced[k][:i], s.synced[k][i+1:]...)
				watchersGauge.Dec()
			}
		}
		if wa.end != 0 {
			for i, w := range s.endm[wa.end] {
				if w == wa {
					s.endm[wa.end] = append(s.endm[wa.end][:i], s.endm[wa.end][i+1:]...)
				}
			}
		}
		// If we cannot find it, it should have finished watch.
	})

	return wa, cancel
}

// keepSyncWatchers syncs the watchers in the unsyncd map every 100ms.
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

// syncWatchers syncs the watchers in the unsyncd map.
func (s *watchableStore) syncWatchers() {
	_, curRev, _ := s.KV.Range(nil, nil, 0, 0)

	// filtering without allocating
	// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	nws := s.unsynced[:0]
	for _, w := range s.unsynced {
		var end []byte
		if w.prefix {
			end = make([]byte, len(w.key))
			copy(end, w.key)
			end[len(w.key)-1]++
		}
		limit := cap(w.ch) - len(w.ch)
		// the channel is full, try it in the next round
		if limit == 0 {
			nws = append(nws, w)
			continue
		}
		evs, nextRev, err := s.KV.(*store).RangeEvents(w.key, end, int64(limit), w.cur, w.end)
		if err != nil {
			w.stopWithError(err)
			continue
		}

		// push events to the channel
		for _, ev := range evs {
			w.ch <- ev
			pendingEventsGauge.Inc()
		}
		// stop watcher if it reaches the end
		if w.end > 0 && nextRev >= w.end {
			w.stopWithError(ExceedEnd)
			continue
		}
		// switch to tracking future events if needed
		if nextRev > curRev {
			s.synced[string(w.key)] = append(s.synced[string(w.key)], w)
			if w.end != 0 {
				s.endm[w.end] = append(s.endm[w.end], w)
			}
			continue
		}
		// put it back to try it in the next round
		w.cur = nextRev
		nws = append(nws, w)
	}
	s.unsynced = nws
	slowWatchersGauge.Set(float64(len(s.unsynced)))
}

// handle handles the change of the happening event on all watchers.
func (s *watchableStore) handle(rev int64, ev storagepb.Event) {
	s.notify(rev, ev)
	s.stopWatchers(rev)
}

// notify notifies the fact that given event at the given rev just happened to
// watchers that watch on the key of the event.
func (s *watchableStore) notify(rev int64, ev storagepb.Event) {
	// check all prefixes of the key to notify all corresponded watchers
	for i := 0; i <= len(ev.Kv.Key); i++ {
		ws := s.synced[string(ev.Kv.Key[:i])]
		nws := ws[:0]
		for _, w := range ws {
			// the watcher needs to be notified when either it watches prefix or
			// the key is exactly matched.
			if !w.prefix && i != len(ev.Kv.Key) {
				continue
			}
			select {
			case w.ch <- ev:
				pendingEventsGauge.Inc()
				nws = append(nws, w)
			default:
				// put it back to unsynced place
				if w.end != 0 {
					for i, ew := range s.endm[w.end] {
						if ew == w {
							s.endm[w.end] = append(s.endm[w.end][:i], s.endm[w.end][i+1:]...)
						}
					}
				}
				w.cur = rev
				s.unsynced = append(s.unsynced, w)
				slowWatchersGauge.Inc()
			}
		}
		s.synced[string(ev.Kv.Key[:i])] = nws
	}
}

// stopWatchers stops watchers with limit equal to rev.
func (s *watchableStore) stopWatchers(rev int64) {
	for i, wa := range s.endm[rev+1] {
		k := string(wa.key)
		for _, w := range s.synced[k] {
			if w == wa {
				s.synced[k] = append(s.synced[k][:i], s.synced[k][i+1:]...)
				watchersGauge.Dec()
			}
		}
		wa.stopWithError(ExceedEnd)
	}
	delete(s.endm, rev+1)
}

type ongoingTx struct {
	// keys put/deleted in the ongoing txn
	putm map[string]bool
	delm map[string]bool
}

func newOngoingTx() *ongoingTx {
	return &ongoingTx{
		putm: make(map[string]bool),
		delm: make(map[string]bool),
	}
}

func (tx *ongoingTx) put(k string) {
	tx.putm[k] = true
	tx.delm[k] = false
}

func (tx *ongoingTx) del(k string) {
	tx.delm[k] = true
	tx.putm[k] = false
}
