package fileSystem

import (
	"fmt"
	etcdErr "github.com/coreos/etcd/error"
	"strings"
	"sync"
	"time"
)

const (
	Get        = "get"
	Create     = "create"
	Update     = "update"
	Delete     = "delete"
	TestAndSet = "testAndSet"
)

type Event struct {
	Action     string         `json:"action"`
	Key        string         `json:"key, omitempty"`
	Dir        bool           `json:"dir,omitempty"`
	PrevValue  string         `json:"prevValue,omitempty"`
	Value      string         `json:"value,omitempty"`
	KVPairs    []KeyValuePair `json:"kvs,omitempty"`
	Expiration *time.Time     `json:"expiration,omitempty"`
	TTL        int64          `json:"ttl,omitempty"` // Time to live in second
	// The command index of the raft machine when the command is executed
	Index uint64 `json:"index"`
	Term  uint64 `json:"term"`
}

// When user list a directory, we add all the node into key-value pair slice
type KeyValuePair struct {
	Key     string         `json:"key, omitempty"`
	Value   string         `json:"value,omitempty"`
	Dir     bool           `json:"dir,omitempty"`
	KVPairs []KeyValuePair `json:"kvs,omitempty"`
}

func newEvent(action string, key string, index uint64, term uint64) *Event {
	return &Event{
		Action: action,
		Key:    key,
		Index:  index,
		Term:   term,
	}
}

type eventQueue struct {
	events   []*Event
	size     int
	front    int
	capacity int
}

func (eq *eventQueue) back() int {
	return (eq.front + eq.size - 1 + eq.capacity) % eq.capacity
}

func (eq *eventQueue) insert(e *Event) {

	index := (eq.back() + 1) % eq.capacity

	eq.events[index] = e

	if eq.size == eq.capacity { //dequeue
		eq.front = (index + 1) % eq.capacity
	} else {
		eq.size++
	}

}

type EventHistory struct {
	Queue      eventQueue
	StartIndex uint64
	rwl        sync.RWMutex
}

func newEventHistory(capacity int) *EventHistory {
	return &EventHistory{
		Queue: eventQueue{
			capacity: capacity,
			events:   make([]*Event, capacity),
		},
	}
}

// addEvent function adds event into the eventHistory
func (eh *EventHistory) addEvent(e *Event) {
	eh.rwl.Lock()
	defer eh.rwl.Unlock()

	eh.Queue.insert(e)

	eh.StartIndex = eh.Queue.events[eh.Queue.front].Index
}

// scan function is enumerating events from the index in history and
// stops till the first point where the key has identified prefix
func (eh *EventHistory) scan(prefix string, index uint64) (*Event, error) {
	eh.rwl.RLock()
	defer eh.rwl.RUnlock()

	start := index - eh.StartIndex

	// the index should locate after the event history's StartIndex
	// and before its size

	if start < 0 {
		// TODO: Add error type
		return nil,
			etcdErr.NewError(etcdErr.EcodeEventIndexCleared,
				fmt.Sprintf("prefix:%v index:%v", prefix, index),
			)
	}

	if start >= uint64(eh.Queue.size) {
		return nil, nil
	}

	i := int((start + uint64(eh.Queue.front)) % uint64(eh.Queue.capacity))

	for {
		e := eh.Queue.events[i]
		if strings.HasPrefix(e.Key, prefix) {
			return e, nil
		}

		i = (i + 1) % eh.Queue.capacity

		if i == eh.Queue.back() {
			// TODO: Add error type
			return nil, nil
		}
	}

}
