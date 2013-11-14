package store

import (
	"container/list"
	"path"
	"strings"
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
func (wh *watcherHub) watch(prefix string, recursive bool, index uint64) (<-chan *Event, *etcdErr.Error) {
	event, err := wh.EventHistory.scan(prefix, index)

	if err != nil {
		return nil, err
	}

	eventChan := make(chan *Event, 1) // use a buffered channel

	if event != nil {
		eventChan <- event

		return eventChan, nil
	}

	w := &watcher{
		eventChan:  eventChan,
		recursive:  recursive,
		sinceIndex: index,
	}

	l, ok := wh.watchers[prefix]

	if ok { // add the new watcher to the back of the list
		l.PushBack(w)

	} else { // create a new list and add the new watcher
		l := list.New()
		l.PushBack(w)
		wh.watchers[prefix] = l
	}

	atomic.AddInt64(&wh.count, 1)

	return eventChan, nil
}

// notify function accepts an event and notify to the watchers.
func (wh *watcherHub) notify(e *Event) {
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
		curr := l.Front()

		for {
			if curr == nil { // we have reached the end of the list
				if l.Len() == 0 {
					// if we have notified all watcher in the list
					// we can delete the list
					delete(wh.watchers, path)
				}
				break
			}

			next := curr.Next() // save reference to the next one in the list

			w, _ := curr.Value.(*watcher)

			if w.notify(e, e.Key == path, deleted) {

				// if we successfully notify a watcher
				// we need to remove the watcher from the list
				// and decrease the counter
				l.Remove(curr)
				atomic.AddInt64(&wh.count, -1)

			}

			curr = next // update current to the next
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
