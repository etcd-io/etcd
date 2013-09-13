package fileSystem

import (
	"fmt"
	"strings"
	"sync"
	"time"

	etcdErr "github.com/coreos/etcd/error"
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

// interfaces for sorting
func (k KeyValuePair) Len() int {
	return len(k.KVPairs)
}

func (k KeyValuePair) Less(i, j int) bool {
	return k.KVPairs[i].Key < k.KVPairs[j].Key
}

func (k KeyValuePair) Swap(i, j int) {
	k.KVPairs[i], k.KVPairs[j] = k.KVPairs[j], k.KVPairs[i]
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
	Events   []*Event
	Size     int
	Front    int
	Capacity int
}

func (eq *eventQueue) back() int {
	return (eq.Front + eq.Size - 1 + eq.Capacity) % eq.Capacity
}

func (eq *eventQueue) insert(e *Event) {

	index := (eq.back() + 1) % eq.Capacity

	eq.Events[index] = e

	if eq.Size == eq.Capacity { //dequeue
		eq.Front = (index + 1) % eq.Capacity
	} else {
		eq.Size++
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
			Capacity: capacity,
			Events:   make([]*Event, capacity),
		},
	}
}

// addEvent function adds event into the eventHistory
func (eh *EventHistory) addEvent(e *Event) {
	eh.rwl.Lock()
	defer eh.rwl.Unlock()

	eh.Queue.insert(e)

	eh.StartIndex = eh.Queue.Events[eh.Queue.Front].Index
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

	if start >= uint64(eh.Queue.Size) {
		return nil, nil
	}

	i := int((start + uint64(eh.Queue.Front)) % uint64(eh.Queue.Capacity))

	for {
		e := eh.Queue.Events[i]
		if strings.HasPrefix(e.Key, prefix) {
			return e, nil
		}

		i = (i + 1) % eh.Queue.Capacity

		if i == eh.Queue.back() {
			// TODO: Add error type
			return nil, nil
		}
	}

}
