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

package v2store

import (
	"container/list"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"go.etcd.io/etcd/server/v3/etcdserver/api/v2error"
)

// A watcherHub contains all subscribed watchers
// watchers is a map with watched path as key and watcher as value
// EventHistory keeps the old events for watcherHub. It is used to help
// watcher to get a continuous event history. Or a watcher might miss the
// event happens between the end of the first watch command and the start
// of the second command.
type watcherHub struct {
	// count must be the first element to keep 64-bit alignment for atomic
	// access

	count int64 // current number of watchers. 当前 watcherHub 实例中保存的 watcher实例个数

	mutex sync.Mutex

	// 维护节点与监听该节点的 watcher 实例的对应关系，其中 key是 node.Path字段值， value则是监听该节点的 watcher实例列表
	watchers map[string]*list.List

	// 保存最近发生的修改操作对应的 Event实例，在 前面的介绍中也提到过， 当 EventHistory 的容量达到上限 之后，继续 向其中添加 Event 实例，则会自动删除最早添加的 Event实例。
	EventHistory *EventHistory
}

// newWatchHub creates a watcherHub. The capacity determines how many events we will
// keep in the eventHistory.
// Typically, we only need to keep a small size of history[smaller than 20K].
// Ideally, it should smaller than 20K/s[max throughput] * 2 * 50ms[RTT] = 2000
func newWatchHub(capacity int) *watcherHub {
	return &watcherHub{
		watchers:     make(map[string]*list.List),
		EventHistory: newEventHistory(capacity),
	}
}

// Watch function returns a Watcher.
// If recursive is true, the first change after index under key will be sent to the event channel of the watcher.
// If recursive is false, the first change after index at key will be sent to the event channel of the watcher.
// If index is zero, watch will start from the current index + 1.
// 客户端请求 etcd服务端为某个指定的节点添加 watcher
// 在添加 watcher 之前，会先查找 EventHistory 确定 sinceIndex ~ CurrentIndex 之间 是否发生了触发待添加 watcher的操作
func (wh *watcherHub) watch(key string, recursive, stream bool, index, storeIndex uint64) (Watcher, *v2error.Error) {
	reportWatchRequest()

	// 在 EventHistory 中，从 index 开始查找对应的 Event,
	event, err := wh.EventHistory.scan(key, recursive, index)

	if err != nil {
		err.Index = storeIndex
		return nil, err
	}

	w := &watcher{
		eventChan:  make(chan *Event, 100), // use a buffered channel
		recursive:  recursive,
		stream:     stream,
		sinceIndex: index,
		startIndex: storeIndex,
		hub:        wh,
	}

	wh.mutex.Lock()
	defer wh.mutex.Unlock()
	// If the event exists in the known history, append the EtcdIndex and return immediately
	// 如果在 EventHistory 中存在合适的 Event，则表示从 sinceIndex 到 CurrentIndex 为止，发生了触发该 watcher 的操作
	if event != nil {
		ne := event.Clone()
		ne.EtcdIndex = storeIndex
		// 将查找到的 Event 写入 eventChan 通道中等待处理
		w.eventChan <- ne
		return w, nil
	}

	// 如果 未查找到合适的 Event , 则表示从 sinceIndex 到 CurrentIndex 为止，并没有发生过触发该 watcher 的操作
	l, ok := wh.watchers[key]

	var elem *list.Element

	if ok { // add the new watcher to the back of the list
		// 将当前 watcher 添加到列表尾部
		elem = l.PushBack(w)
	} else { // create a new list and add the new watcher
		// 第一次为节点添加 watcher 时，会先创建对应的 watcher 列表
		l = list.New()
		elem = l.PushBack(w)
		wh.watchers[key] = l
	}

	// 初始化当前 watcher.remove 字段
	w.remove = func() {
		if w.removed { // avoid removing it twice
			return
		}
		w.removed = true
		l.Remove(elem)
		atomic.AddInt64(&wh.count, -1)
		reportWatcherRemoved()
		if l.Len() == 0 {
			delete(wh.watchers, key)
		}
	}

	atomic.AddInt64(&wh.count, 1)
	reportWatcherAdded()

	return w, nil
}

func (wh *watcherHub) add(e *Event) {
	wh.EventHistory.addEvent(e)
}

// notify function accepts an event and notify to the watchers.
// 当发生修改操作时，storage 会 通过 watcherHub.notify()方法触发监昕对应节点 的 watcher
func (wh *watcherHub) notify(e *Event) {

	// 首先将修改操作对应的 Event 实例保存到 EventHistory 中
	e = wh.EventHistory.addEvent(e) // add event into the eventHistory

	segments := strings.Split(e.Node.Key, "/")

	currPath := "/"

	// walk through all the segments of the path and notify the watchers
	// if the path is "/foo/bar", it will notify watchers with path "/",
	// "/foo" and "/foo/bar"

	// 遍历 切分结果，并调用 notifyWatchers() 方 法，通知当前节点和所有父节点上的 watcher ,
	// 这里举个例子，如果“/foo/bar”节点被修 改了 ，那么不仅监听“/foo/bar"节点的 watcher 会被触发，
	// 监听“/fo” 节点的 watcher 也可能被触发
	for _, segment := range segments {
		currPath = path.Join(currPath, segment)
		// notify the watchers who interests in the changes of current path
		wh.notifyWatchers(e, currPath, false)
	}
}

// 会查找节点对应的 watcher 列表 ， 并逐一触发列表中的 watcher实例，
func (wh *watcherHub) notifyWatchers(e *Event, nodePath string, deleted bool) {
	wh.mutex.Lock()
	defer wh.mutex.Unlock()

	l, ok := wh.watchers[nodePath]
	if ok {
		curr := l.Front()

		for curr != nil {
			next := curr.Next() // save reference to the next one in the list

			w, _ := curr.Value.(*watcher)

			originalPath := e.Node.Key == nodePath

			// 调用 watcher.notify() 方法，触发相应的 watcher
			if (originalPath || !isHidden(nodePath, e.Node.Key)) && w.notify(e, originalPath, deleted) {
				if !w.stream { // do not remove the stream watcher
					// if we successfully notify a watcher
					// we need to remove the watcher from the list
					// and decrease the counter
					// 检测是否为 stream watcher，如果不是，需要将该 watcher 清除
					w.removed = true
					l.Remove(curr)
					atomic.AddInt64(&wh.count, -1)
					reportWatcherRemoved()
				}
			}

			curr = next // update current to the next element in the list
		}

		if l.Len() == 0 {
			// if we have notified all watcher in the list
			// we can delete the list
			// 如果列表中的watcher实例全部被清空，则将该列表从watchHub中清除
			delete(wh.watchers, nodePath)
		}
	}
}

// clone function clones the watcherHub and return the cloned one.
// only clone the static content. do not clone the current watchers.
func (wh *watcherHub) clone() *watcherHub {
	clonedHistory := wh.EventHistory.clone()

	return &watcherHub{
		EventHistory: clonedHistory,
	}
}

// isHidden checks to see if key path is considered hidden to watch path i.e. the
// last element is hidden or it's within a hidden directory
func isHidden(watchPath, keyPath string) bool {
	// When deleting a directory, watchPath might be deeper than the actual keyPath
	// For example, when deleting /foo we also need to notify watchers on /foo/bar.
	if len(watchPath) > len(keyPath) {
		return false
	}
	// if watch path is just a "/", after path will start without "/"
	// add a "/" to deal with the special case when watchPath is "/"
	afterPath := path.Clean("/" + keyPath[len(watchPath):])
	return strings.Contains(afterPath, "/_")
}
