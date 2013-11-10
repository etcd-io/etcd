package store

import (
	"fmt"
	"strings"
	"sync"

	etcdErr "github.com/coreos/etcd/error"
)

type EventHistory struct {
	Queue      eventQueue
	StartIndex uint64
	LastIndex  uint64
	rwl        sync.RWMutex
}

func newEventHistory(capacity int) *EventHistory {
	return &EventHistory{
		Queue: eventQueue{
			Capacity: capacity,
			Events:   make([]*Event, capacity),
		},
	}
}

// addEvent function adds event into the eventHistory
func (eh *EventHistory) addEvent(e *Event) *Event {
	eh.rwl.Lock()
	defer eh.rwl.Unlock()

	eh.Queue.insert(e)

	eh.LastIndex = e.Index()

	eh.StartIndex = eh.Queue.Events[eh.Queue.Front].ModifiedIndex

	return e
}

// scan function is enumerating events from the index in history and
// stops till the first point where the key has identified prefix
func (eh *EventHistory) scan(prefix string, index uint64) (*Event, *etcdErr.Error) {
	eh.rwl.RLock()
	defer eh.rwl.RUnlock()

	// the index should locate after the event history's StartIndex
	if index-eh.StartIndex < 0 {
		return nil,
			etcdErr.NewError(etcdErr.EcodeEventIndexCleared,
				fmt.Sprintf("the requested history has been cleared [%v/%v]",
					eh.StartIndex, index), 0)
	}

	// the index should locate before the size of the queue minus the duplicate count
	if index > eh.LastIndex { // future index
		return nil, nil
	}

	i := eh.Queue.Front

	for {
		e := eh.Queue.Events[i]

		if strings.HasPrefix(e.Key, prefix) && index <= e.Index() { // make sure we bypass the smaller one
			return e, nil
		}

		i = (i + 1) % eh.Queue.Capacity

		if i > eh.Queue.back() {
			return nil, nil
		}
	}
}

// clone will be protected by a stop-world lock
// do not need to obtain internal lock
func (eh *EventHistory) clone() *EventHistory {
	clonedQueue := eventQueue{
		Capacity: eh.Queue.Capacity,
		Events:   make([]*Event, eh.Queue.Capacity),
		Size:     eh.Queue.Size,
		Front:    eh.Queue.Front,
	}

	for i, e := range eh.Queue.Events {
		clonedQueue.Events[i] = e
	}

	return &EventHistory{
		StartIndex: eh.StartIndex,
		Queue:      clonedQueue,
		LastIndex:  eh.LastIndex,
	}

}
