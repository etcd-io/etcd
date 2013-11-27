package store

import (
	"container/list"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	etcdErr "github.com/coreos/etcd/error"
)

// A watcherHub contains all subscribed watchers
// watchers is a map with watched path as key and watcher as value
// EventHistory keeps the old events for watcherHub. It is used to help
// watcher to get a continuous event history. Or a watcher might miss the
// event happens between the end of the first watch command and the start
// of the second command.
type watcherHub struct {
	watchers     map[string]*list.List
	count        int64 // current number of watchers.
	EventHistory *EventHistory
	rwl          sync.RWMutex
}

// newWatchHub creates a watchHub. The capacity determines how many events we will
// keep in the eventHistory.
// Typically, we only need to keep a small size of history[smaller than 20K].
// Ideally, it should smaller than 20K/s[max throughput] * 2 * 50ms[RTT] = 2000
func newWatchHub(capacity int) *watcherHub {
	return &watcherHub{
		watchers:     make(map[string]*list.List),
		EventHistory: newEventHistory(capacity),
	}
}

// watch function returns an Event channel.
// If recursive is true, the first change after index under prefix will be sent to the event channel.
// If recursive is false, the first change after index at prefix will be sent to the event channel.
// If index is zero, watch will start from the current index + 1.
func (wh *watcherHub) watch(prefix string, recursive bool, index uint64) (*watcher, *etcdErr.Error) {
	wh.rwl.Lock()
	defer wh.rwl.Unlock()

	event, err := wh.EventHistory.scan(prefix, index)
	if err != nil {
		return nil, err
	}

	var elem *list.Element // points to the watcher below

	w := &watcher{
		EventChan: make(chan *Event, 1), // use a buffered channel
		Cancel: func() {
			if l, ok := wh.watchers[prefix]; ok {
				wh.rwl.Lock()
				defer wh.rwl.Unlock()

				l.Remove(elem)
				if l.Len() == 0 {
					delete(wh.watchers, prefix)
				}

				atomic.AddInt64(&wh.count, -1)
			}
		},
		recursive:  recursive,
		sinceIndex: index,
	}

	if event != nil {
		w.EventChan <- event
		return w, nil
	}

	l, ok := wh.watchers[prefix]

	if ok { // add the new watcher to the back of the list
		l.PushBack(w)

	} else { // create a new list and add the new watcher
		l := list.New()
		elem = l.PushBack(w)
		wh.watchers[prefix] = l
	}

	atomic.AddInt64(&wh.count, 1)

	return w, nil
}

// notify function accepts an event and notify to the watchers.
func (wh *watcherHub) notify(e *Event) {
	wh.rwl.RLock()
	defer wh.rwl.RUnlock()

	e = wh.EventHistory.addEvent(e) // add event into the eventHistory

	segments := strings.Split(e.Key, "/")

	currPath := "/"

	// walk through all the segments of the path and notify the watchers
	// if the path is "/foo/bar", it will notify watchers with path "/",
	// "/foo" and "/foo/bar"

	for _, segment := range segments {
		currPath = path.Join(currPath, segment)
		// notify the watchers who interests in the changes of current path
		wh.notifyWatchers(e, currPath, false)
	}
}

func (wh *watcherHub) notifyWatchers(e *Event, path string, deleted bool) {
	l, ok := wh.watchers[path]
	if ok {
		for elem := l.Front(); elem != nil; elem = elem.Next() {
			w := elem.Value.(*watcher)
			w.notify(e, e.Key == path, deleted)
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
