/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

type Watcher struct {
	EventChan  chan *Event
	stream     bool
	recursive  bool
	sinceIndex uint64
	removed    bool
	remove     func()
}

// notify function notifies the watcher. If the watcher interests in the given path,
// the function will return true.
func (w *Watcher) notify(e *Event, originalPath bool, deleted bool) bool {
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
		select {
		case w.EventChan <- e:

		// the stream watcher might be slow
		// but we cannot block here. blocking will lead the whole etcd system to hang.
		// create a go-routine to handle the blocking case
		default:
			go func() {
				// TODO add a warning here should be helpful
				w.EventChan <- e
			}()
		}
		return true
	}
	return false
}

// Remove removes the watcher from watcherHub
// The actual remove function is guaranteed to only be executed once
func (w *Watcher) Remove() {
	if w.remove != nil {
		w.remove()
	} else {
		// We attached a remove function to watcher
		// Other pkg cannot change it, so this should not happen
		panic("missing Watcher remove function")
	}
}
