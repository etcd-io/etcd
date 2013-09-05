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

func newWatchHub(capacity int) *watcherHub {
	return &watcherHub{
		watchers:     make(map[string]*list.List),
		EventHistory: newEventHistory(capacity),
	}
}

func (wh *watcherHub) watch(prefix string, index uint64) (error, <-chan *Event) {
	eventChan := make(chan *Event, 1)

	e, err := wh.EventHistory.scan(prefix, index)

	if err != nil {
		return err, nil
	}

	if e != nil {
		eventChan <- e
		return nil, eventChan
	}

	l, ok := wh.watchers[prefix]

	if ok {
		l.PushBack(eventChan)
	} else {
		l := list.New()
		l.PushBack(eventChan)
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

			for {
				element := l.Front()
				if element == nil {
					delete(wh.watchers, currPath)
					break
				}
				c, _ := element.Value.(chan *Event)
				c <- e
				l.Remove(element)
			}
		}

	}
}
