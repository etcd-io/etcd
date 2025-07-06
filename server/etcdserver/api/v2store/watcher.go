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

type Watcher interface {
	EventChan() chan *Event
	StartIndex() uint64 // The EtcdIndex at which the Watcher was created
	Remove()
}

type watcher struct {
	// 当该 watcher 实例被修改操作触发时，会将对应的 Event实例写入到该通道中，后续由网络层读取该通道， 并通知客户端此次修改。
	// 在后面介绍 watcherHub 实现时会看到，在创建 watcher 实例时会将该通道的缓冲区设置成 100 。
	eventChan  chan *Event
	stream     bool
	recursive  bool   // 标识当前 watcher实例是否监听目标节点的子节点
	sinceIndex uint64 // 标识该 watcher 实例从哪个 CurrentIndex 值开始监 听对应 的节点。
	startIndex uint64 // 记录创建该 watcher实例时对应的 CurrentIndex 值

	// 在 watcherHub 中维护了该watcher实例与其监听的节点路径的映射关系， watcherHub 的具体实现在后面详细分析
	hub     *watcherHub
	removed bool   // 标记当前 watcher 实例是 否 己经被删除
	remove  func() // 用于删除当前 watcher 实例的回调函数，具体的调用时机在后面详细介绍 。
}

func (w *watcher) EventChan() chan *Event {
	return w.eventChan
}

func (w *watcher) StartIndex() uint64 {
	return w.startIndex
}

// notify function notifies the watcher. If the watcher interests in the given path,
// the function will return true.
// 将触发该 watcher 的 Event 实例写入 eventChan 通道中，在如下三种场景下当前 watcher会被触发:
// 当修改操作发生在当前 watcher 实例监昕的节点上时，会触发该 watcher 实例。
// 当前 watcher 实例不仅监听当前节点的变化，同时也监昕其子节点的变化，当修改操作发生在子节点上时， 也会触发该 watcher 实例。
// 当删除某个目录节点时，需要通知在其子节点上监听的全部 watcher实例。
func (w *watcher) notify(e *Event, originalPath bool, deleted bool) bool {
	// watcher is interested the path in three cases and under one condition
	// the condition is that the event happens after the watcher's sinceIndex

	// 1. the path at which the event happens is the path the watcher is watching at.
	// For example if the watcher is watching at "/foo" and the event happens at "/foo",
	// the watcher must be interested in that event.

	// 2. the watcher is a recursive watcher, it interests in the event happens after
	// its watching path. For example if watcher A watches at "/foo" and it is a recursive
	// one, it will interest in the event happens at "/foo/bar".

	// 3. when we delete a directory, we need to force notify all the watchers who watches
	// at the file we need to delete.
	// For example a watcher is watching at "/foo/bar". And we deletes "/foo". The watcher
	// should get notified even if "/foo" is not the path it is watching.
	if (w.recursive || originalPath || deleted) && e.Index() >= w.sinceIndex {
		// We cannot block here if the eventChan capacity is full, otherwise
		// etcd will hang. eventChan capacity is full when the rate of
		// notifications are higher than our send rate.
		// If this happens, we close the channel.
		select {
		case w.eventChan <- e: // 将修改操作对应的 Event 写入 eventChan 等待处理
		default:
			// We have missed a notification. Remove the watcher.
			// Removing the watcher also closes the eventChan.
			// 如采当前 watcher.eventChan 通道被填充满了，则会将该通道关闭，这可能导致事件丢失
			w.remove()
		}
		return true
	}
	return false
}

// Remove removes the watcher from watcherHub
// The actual remove function is guaranteed to only be executed once
func (w *watcher) Remove() {
	w.hub.mutex.Lock()
	defer w.hub.mutex.Unlock()

	close(w.eventChan)
	if w.remove != nil {
		w.remove()
	}
}

// nopWatcher is a watcher that receives nothing, always blocking.
type nopWatcher struct{}

func NewNopWatcher() Watcher                 { return &nopWatcher{} }
func (w *nopWatcher) EventChan() chan *Event { return nil }
func (w *nopWatcher) StartIndex() uint64     { return 0 }
func (w *nopWatcher) Remove()                {}
