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
	LastTerm   uint64
	DupCnt     uint64 // help to compute the watching point with duplicated indexes in the queue
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

	if e.Index == eh.LastIndex {
		eh.DupCnt += 1
	}

	eh.Queue.insert(e)

	eh.LastIndex = e.Index
	eh.LastTerm = e.Term

	eh.StartIndex = eh.Queue.Events[eh.Queue.Front].Index

	return e
}

// scan function is enumerating events from the index in history and
// stops till the first point where the key has identified prefix
func (eh *EventHistory) scan(prefix string, index uint64) ([]*Event, *etcdErr.Error) {
	eh.rwl.RLock()
	defer eh.rwl.RUnlock()

	// the index should locate after the event history's StartIndex
	if index-eh.StartIndex < 0 {
		return nil,
			etcdErr.NewError(etcdErr.EcodeEventIndexCleared,
				fmt.Sprintf("the requested history has been cleared [%v/%v]",
					eh.StartIndex, index), UndefIndex, UndefTerm)
	}

	// the index should locate before the size of the queue minus the duplicate count
	if index > eh.LastIndex { // future index
		return nil, nil
	}

	i := eh.Queue.Front

	events := make([]*Event, 0)
	var eventIndex uint64

	for {
		e := eh.Queue.Events[i]

		if eventIndex != 0 && eventIndex != e.Index {
			return events, nil
		}

		if strings.HasPrefix(e.Key, prefix) && index <= e.Index { // make sure we bypass the smaller one
			eventIndex = e.Index
			events = append(events, e)
		}

		i = (i + 1) % eh.Queue.Capacity

		if i > eh.Queue.back() {
			if eventIndex == 0 { // find nothing, return and watch from current index
				return nil, nil
			}
			return events, nil
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
		LastTerm:   eh.LastTerm,
		DupCnt:     eh.DupCnt,
	}

}
