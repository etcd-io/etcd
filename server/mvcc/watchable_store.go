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
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"

	"go.uber.org/zap"
)

// non-const so modifiable by tests
var (
	// chanBufLen is the length of the buffered chan
	// for sending out watched events.
	// See https://github.com/etcd-io/etcd/issues/11906 for more detail.
	chanBufLen = 128

	// maxWatchersPerSync is the number of watchers to sync in a single batch
	maxWatchersPerSync = 512
)

type watchable interface {
	watch(key, end []byte, startRev int64, id WatchID, ch chan<- WatchResponse, fcs ...FilterFunc) (*watcher, cancelFunc)
	progress(w *watcher)
	rev() int64
}

// etcd初始化时候创建的一个全局的配置项
type watchableStore struct {
	*store

	// mu protects watcher groups and batches. It should never be locked
	// before locking store.mu to avoid deadlock.
	mu sync.RWMutex

	// victims are watcher batches that were blocked on the watch channel
	// 如果watcher实例关联的ch通道被阻塞了，则对应的 watcherBatch 实例会暂时记录到该字段中。
	// 在 watchableStore 实例中还会启动一个后台 goroutine 来处理该字段中保存的 watcherBatch 实例。
	// watcherBatch 实际上就是 map[*watcher]*eventBatch类型
	// watcherBatch只提供了一个 add()方法，用来添加被触发的 watcher实例 与 其eventBatch之间的映射关系。
	// victims是在变更发生，发送数据到ch但是通道满时，被阻塞的watcher集合
	victims []watcherBatch
	// 当有新的 watcherBatch 实例添加到 victims 字段中时，会 向该通道中发送 一个空结构体作为信号。
	// 用于通知是否需要清理victims
	victimc chan struct{}

	// contains all unsynced watchers that needs to sync with events that have happened
	// watchableStore实例会启动一个后台的goroutine持续同步 unsynced watcherGroup， 然 后将完成同步的 watcher 实例迁移到 synced watcherGroup 中储存。
	// 未同步完成的watcher
	// 产生watcher堆积的原因主要是两种
	// 一种是当客户端执行watch时候指定了历史版本号，该操作需要从boltDB中取值，不能直接放到synced队列中等待新的变更，需要放到unsync中。
	// 第二种是积压的原因是因为检测到了watch变更，在将数据发送到ch时候，ch缓冲已满，此时需要将watcher存到其他区域（victims）。如果硬往里放数据，该协程会被park住，阻塞其他操作。
	// 检测到watcher变更发生在put一个键值时候，此时事务提交，最终写入之前会调用notify()方法检测是否有针对该键值的watcher。
	unsynced watcherGroup

	// contains all synced watchers that are in sync with the progress of the store.
	// The key of the map is the key that the watcher watches on.
	// 全部 的 watcher 实例都已经同步完毕，并等待新的更新操作
	// synced watcherGroup 中的 watcher 实例都落后于当前最新更新操作，并且有一个单独的后台 goroutine 帮助其进行追赶。
	// 全部watcher实例 都已经同步完成，在等待新的新的变更事件的watcher队列
	// 当 etcd服务端收到客户端的 watch请求时，如果请求携带了 revision参数，则比较该 请求的 revision 信息 和 store.currentRev 信息:如果请求中的 revision 信息较大，则放 入 synced watcherGroup 中，否则放入 unsynced watcherGroup
	synced watcherGroup

	stopc chan struct{}
	wg    sync.WaitGroup
}

// cancelFunc updates unsynced and synced maps when running
// cancel operations.
type cancelFunc func()

// 实现了WatchableKV，etcd初始化的时候会调用
func New(lg *zap.Logger, b backend.Backend, le lease.Lessor, cfg StoreConfig) WatchableKV {
	return newWatchableStore(lg, b, le, cfg)
}

func newWatchableStore(lg *zap.Logger, b backend.Backend, le lease.Lessor, cfg StoreConfig) *watchableStore {
	if lg == nil {
		lg = zap.NewNop()
	}
	s := &watchableStore{
		store:    NewStore(lg, b, le, cfg),
		victimc:  make(chan struct{}, 1),
		unsynced: newWatcherGroup(),
		synced:   newWatcherGroup(),
		stopc:    make(chan struct{}),
	}
	//
	s.store.ReadView = &readView{s}
	s.store.WriteView = &writeView{s}
	if s.le != nil {
		// use this store as the deleter so revokes trigger watch events
		s.le.SetRangeDeleter(func() lease.TxnDelete { return s.Write(traceutil.TODO()) })
	}
	// 创建两个协程，用于处理watcher数据
	s.wg.Add(2)
	go s.syncWatchersLoop() // 启动处理 unsynced watcherGroup的后台 goroutine
	go s.syncVictimsLoop()  // 启动处理 victims 的后台 goroutine
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
		//etcd启动时初始化的watchableStore
		watchable: s,
		//这个管道用于从etcd里面拿到变更数据，调用Chan()可以取出数据，buf长度是128
		ch:       make(chan WatchResponse, chanBufLen),
		cancels:  make(map[WatchID]cancelFunc),
		watchers: make(map[WatchID]*watcher),
	}
}

func (s *watchableStore) watch(key, end []byte, startRev int64, id WatchID, ch chan<- WatchResponse, fcs ...FilterFunc) (*watcher, cancelFunc) {
	//key以及ch被封装到了watcher结构体中
	wa := &watcher{
		key:    key,
		end:    end,
		minRev: startRev,
		id:     id,
		ch:     ch,
		fcs:    fcs,
	}

	s.mu.Lock()
	s.revMu.RLock()

	// 根据当前的watcher状态决定放到 已同步的池子 还是 未同步的池子
	// 如果指定的版本号version是历史的版本号，则将watcher放到unsynced中
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
	for {
		s.mu.Lock()
		//尝试从各个队列删除
		if s.unsynced.delete(wa) {
			slowWatcherGauge.Dec()
			watcherGauge.Dec()
			break
		} else if s.synced.delete(wa) {
			watcherGauge.Dec()
			break
		} else if wa.compacted {
			watcherGauge.Dec()
			break
		} else if wa.ch == nil {
			// already canceled (e.g., cancel/close race)
			break
		}

		if !wa.victim {
			s.mu.Unlock()
			panic("watcher not victim but not in watch groups")
		}

		//victims删除
		var victimBatch watcherBatch
		for _, wb := range s.victims {
			if wb[wa] != nil {
				victimBatch = wb
				break
			}
		}
		if victimBatch != nil {
			slowWatcherGauge.Dec()
			watcherGauge.Dec()
			delete(victimBatch, wa)
			break
		}

		// victim being processed so not accessible; retry
		s.mu.Unlock()
		time.Sleep(time.Millisecond)
	}

	wa.ch = nil
	s.mu.Unlock()
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

	for {
		s.mu.RLock()
		st := time.Now()
		lastUnsyncedWatchers := s.unsynced.size()
		s.mu.RUnlock()

		unsyncedWatchers := 0
		if lastUnsyncedWatchers > 0 {
			//如果队列大于0，则进入syncWatchers()同步watcher
			unsyncedWatchers = s.syncWatchers()
		}

		//定时器处理

		syncDuration := time.Since(st)

		waitDuration := 100 * time.Millisecond
		// more work pending?
		if unsyncedWatchers != 0 && lastUnsyncedWatchers > unsyncedWatchers {
			// be fair to other store operations by yielding time taken
			waitDuration = syncDuration
		}

		select {
		case <-time.After(waitDuration):
		case <-s.stopc:
			return
		}
	}
}

// syncVictimsLoop tries to write precomputed watcher responses to
// watchers that had a blocked watcher channel
func (s *watchableStore) syncVictimsLoop() {
	defer s.wg.Done()

	for {
		//通过moveVictims清除堆积数据
		for s.moveVictims() != 0 {
			// try to update all victim watchers
		}
		s.mu.RLock()
		isEmpty := len(s.victims) == 0
		s.mu.RUnlock()

		var tickc <-chan time.Time
		if !isEmpty {
			tickc = time.After(10 * time.Millisecond)
		}

		select {
		case <-tickc:
		case <-s.victimc: //接收到信号，开始进行清理
		case <-s.stopc:
			return
		}
	}
}

// moveVictims tries to update watches with already pending event data
// 清理过程也是通过尝试发送，发送受到阻塞则放入新的victims，发送成功则进一步判断是将watcher队列移动到synced或unsynced队列中，最后使用新的victims赋值，这样做保证了不会产生数据的丢失。
func (s *watchableStore) moveVictims() (moved int) {
	s.mu.Lock()
	//把victims队列取出来，并置s.victims为空，后续使用newVictim代替
	victims := s.victims
	s.victims = nil
	s.mu.Unlock()

	var newVictim watcherBatch

	//遍历队列，尝试发送
	for _, wb := range victims {
		// try to send responses again

		// 尝试发送，发送阻塞放到newVictim
		for w, eb := range wb {
			// watcher has observed the store up to, but not including, w.minRev
			rev := w.minRev - 1
			if w.send(WatchResponse{WatchID: w.id, Events: eb.evs, Revision: rev}) {
				pendingEventsGauge.Add(float64(len(eb.evs)))
			} else {
				if newVictim == nil {
					newVictim = make(watcherBatch)
				}
				newVictim[w] = eb
				continue
			}
			moved++
		}

		// assign completed victim watchers to unsync/sync
		s.mu.Lock()
		s.store.revMu.RLock()
		curRev := s.store.currentRev

		//遍历并判断是否将消息发送完了
		for w, eb := range wb {
			if newVictim != nil && newVictim[w] != nil {
				// couldn't send watch response; stays victim
				continue
			}
			w.victim = false
			if eb.moreRev != 0 {
				w.minRev = eb.moreRev
			}
			if w.minRev <= curRev {
				//如果版本号小于当前版本，则导入unsync队列
				s.unsynced.add(w)
			} else {
				//放入sync队列
				slowWatcherGauge.Dec()
				s.synced.add(w)
			}
		}
		s.store.revMu.RUnlock()
		s.mu.Unlock()
	}

	//把新的队列放置到victims中
	if len(newVictim) > 0 {
		s.mu.Lock()
		s.victims = append(s.victims, newVictim)
		s.mu.Unlock()
	}

	return moved
}

// syncWatchers syncs unsynced watchers by:
//  1. choose a set of watchers from the unsynced watcher group
//  2. iterate over the set to get the minimum revision and remove compacted watchers
//  3. use minimum revision to get all key-value pairs and send those events to watchers
//  4. remove synced watchers in set from unsynced group and move to synced group
//
// syncWatchersLoop通过一个定时器每隔100ms轮询一次unsynced watcher队列，如果队列不为空，就筛选出数据中的对应键值对以及相应版本号，并最终返还给客户端，将watcher移动到synced队列。
func (s *watchableStore) syncWatchers() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.unsynced.size() == 0 {
		return 0
	}

	s.store.revMu.RLock()
	defer s.store.revMu.RUnlock()

	// in order to find key-value pairs from unsynced watchers, we need to
	// find min revision index, and these revisions can be used to
	// query the backend store of key-value pairs
	curRev := s.store.currentRev
	compactionRev := s.store.compactMainRev

	//选出unsync队列中的watcher，返回一个watcherGroup
	// 一次批量同步的watcher数量
	wg, minRev := s.unsynced.choose(maxWatchersPerSync, curRev, compactionRev)
	minBytes, maxBytes := newRevBytes(), newRevBytes()
	revToBytes(revision{main: minRev}, minBytes)
	revToBytes(revision{main: curRev + 1}, maxBytes)

	// UnsafeRange returns keys and values. And in boltdb, keys are revisions.
	// values are actual key-value pairs in backend.
	//从boltdb中取出所有键值以及对应版本号
	tx := s.store.b.ReadTx()
	tx.RLock()
	revs, vs := tx.UnsafeRange(buckets.Key, minBytes, maxBytes, 0)
	tx.RUnlock()
	//由于上面取出的是所有的kv对以及版本号，所有要使用watcherGroup筛选出来监听的键值对应事件
	evs := kvsToEvents(s.store.lg, wg, revs, vs)

	var victims watcherBatch
	wb := newWatcherBatch(wg, evs)
	for w := range wg.watchers {
		w.minRev = curRev + 1

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

		//发送消息到watcher对应的ch，如果阻塞，放入victims队列中
		if w.send(WatchResponse{WatchID: w.id, Events: eb.evs, Revision: curRev}) {
			pendingEventsGauge.Add(float64(len(eb.evs)))
		} else {
			if victims == nil { //为空就make一个
				victims = make(watcherBatch)
			}
			w.victim = true
		}

		if w.victim {
			victims[w] = eb
		} else {
			if eb.moreRev != 0 {
				// stay unsynced; more to read
				continue
			}
			//消息发送完了，把watcher放入synced队列，等待新的变更事件
			s.synced.add(w)
		}
		//把unsynced中的watcher取消掉
		s.unsynced.delete(w)
	}

	//增加victim
	s.addVictim(victims)

	vsz := 0
	for _, v := range s.victims {
		vsz += len(v)
	}
	slowWatcherGauge.Set(float64(s.unsynced.size() + vsz))

	return s.unsynced.size()
}

// kvsToEvents gets all events for the watchers from all key-value pairs
func kvsToEvents(lg *zap.Logger, wg *watcherGroup, revs, vals [][]byte) (evs []mvccpb.Event) {
	for i, v := range vals {
		var kv mvccpb.KeyValue
		if err := kv.Unmarshal(v); err != nil {
			lg.Panic("failed to unmarshal mvccpb.KeyValue", zap.Error(err))
		}

		if !wg.contains(string(kv.Key)) {
			continue
		}

		ty := mvccpb.PUT
		if isTombstone(revs[i]) {
			ty = mvccpb.DELETE
			// patch in mod revision so watchers won't skip
			kv.ModRevision = bytesToRev(revs[i]).main
		}
		evs = append(evs, mvccpb.Event{Kv: &kv, Type: ty})
	}
	return evs
}

// notify notifies the fact that given event at the given rev just happened to
// watchers that watch on the key of the event.
func (s *watchableStore) notify(rev int64, evs []mvccpb.Event) {
	var victim watcherBatch

	//newWatcherBatch会遍历watchableStore的synced队列，并拿evs中kv对比是否有监听的key，返回一个watcher集合
	//for range遍历newWatcherBatch返回的watcher集合
	for w, eb := range newWatcherBatch(&s.synced, evs) {
		if eb.revs != 1 {
			s.store.lg.Panic(
				"unexpected multiple revisions in watch notification",
				zap.Int("number-of-revisions", eb.revs),
			)
		}
		//调用send方法将event发送到ch中，未阻塞的话，会被最上层的sendLoop接收到。
		if w.send(WatchResponse{WatchID: w.id, Events: eb.evs, Revision: rev}) {
			//promethous操作
			pendingEventsGauge.Add(float64(len(eb.evs)))
		} else {
			// move slow watcher to victims
			//将watcher添加到victims集合中
			w.minRev = rev + 1
			if victim == nil {
				victim = make(watcherBatch)
			}
			w.victim = true
			victim[w] = eb
			//删除synced队列中的该watch
			s.synced.delete(w)
			slowWatcherGauge.Inc()
		}
	}
	s.addVictim(victim)
}

func (s *watchableStore) addVictim(victim watcherBatch) {
	if victim == nil {
		return
	}
	//增加watcher到victims，并发送信号通知
	s.victims = append(s.victims, victim)
	select {
	case s.victimc <- struct{}{}:
	default:
	}
}

func (s *watchableStore) rev() int64 { return s.store.Rev() }

func (s *watchableStore) progress(w *watcher) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.synced.watchers[w]; ok {
		w.send(WatchResponse{WatchID: w.id, Revision: s.rev()})
		// If the ch is full, this watcher is receiving events.
		// We do not need to send progress at all.
	}
}

type watcher struct {
	// the watcher key
	// 该 watcher 实例监听的 原始 Key 值 。
	key []byte
	// end indicates the end of the range to watch.
	// If end is set, the watcher is on a range.
	// 该watcher实例监昕的结束位置(也是一个原始Key值〉。
	// 如果该字段有值， 则当前 watcher 实例是一个范围 watcher。 如果该字段未设置值，则当前 watcher 只监听上面的 key 字段对应的键值对。
	end []byte

	// victim is set when ch is blocked and undergoing victim processing
	// 当上述 ch chan<- WatchResponse 通道阻塞时 ， 会将该字段设置成 true。
	victim bool

	// compacted is set when the watcher is removed because of compaction
	// 如果该宇段被设置为 true，则表示当前 watcher 己经因为发生了压缩操作而被删除。
	compacted bool

	// restore is true when the watcher is being restored from leader snapshot
	// which means that this watcher has just been moved from "synced" to "unsynced"
	// watcher group, possibly with a future revision when it was first added
	// to the synced watcher
	// "unsynced" watcher revision must always be <= current revision,
	// except when the watcher were to be moved from "synced" watcher group
	restore bool

	// minRev is the minimum revision update the watcher will accept
	// 能够触发当前 watcher实例的最小 revision值。发生在该 revision 之前的更新操作是无法触发该 watcher 实例。
	minRev int64
	id     WatchID // 当前 watcher 实例的唯一标识。

	// 过滤器。触发当前 wathcer 实例的事件 (后面介绍的 Event 结构体就是对“事件”的抽象)需要经过这些过滤器的过滤才能封装进响应(后面介 绍的 WatchResponse 就是对“响应”的抽象)中 。
	fcs []FilterFunc
	// a chan to send out the watch response.
	// The chan might be shared with other watchers.
	// 当前 watcher 实例被触发之后，会 向该通道 中写 入 WatchResponse。该通道可能是由多个 watcher 实例共享的
	ch chan<- WatchResponse
}

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

	// if all events are filtered out, we should send nothing.
	if !progressEvent && len(wr.Events) == 0 {
		return true
	}
	select {
	//将消息发送到channel，如果ch满了就走default
	case w.ch <- wr:
		return true
	default:
		return false
	}
}
