package fileSystem

import (
	"container/list"
	"path"
	"strings"
)

type watcherHub struct {
	watchers     map[string]*list.List
	count        uint64 // current number of watchers
	EventHistory *EventHistory
}

type watcher struct {
	eventChan chan *Event
	recursive bool
}

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
func (wh *watcherHub) watch(prefix string, recursive bool, index uint64) (error, <-chan *Event) {
	eventChan := make(chan *Event, 1)

	e, err := wh.EventHistory.scan(prefix, index)

	if err != nil {
		return err, nil
	}

	if e != nil {
		eventChan <- e
		return nil, eventChan
	}

	w := &watcher{
		eventChan: eventChan,
		recursive: recursive,
	}

	l, ok := wh.watchers[prefix]

	if ok { // add the new watcher to the back of the list
		l.PushBack(w)

	} else { // create a new list and add the new watcher
		l := list.New()
		l.PushBack(w)
		wh.watchers[prefix] = l
	}

	return nil, eventChan
}

func (wh *watcherHub) notify(e *Event) {

	segments := strings.Split(e.Key, "/")
	currPath := "/"

	// walk through all the paths
	for _, segment := range segments {
		currPath = path.Join(currPath, segment)

		l, ok := wh.watchers[currPath]

		if ok {

			curr := l.Front()
			notifiedAll := true

			for {

				if curr == nil { // we have reached the end of the list

					if notifiedAll {
						// if we have notified all watcher in the list
						// we can delete the list
						delete(wh.watchers, currPath)
					}
					break
				}

				next := curr.Next() // save the next

				w, _ := curr.Value.(*watcher)

				if w.recursive {
					w.eventChan <- e
					l.Remove(curr)
				} else {
					if e.Key == currPath { // only notify the same path
						w.eventChan <- e
						l.Remove(curr)
					} else { // we do not notify all watcher in the list
						notifiedAll = false
					}
				}

				curr = next // go to the next one

			}
		}

	}
}
