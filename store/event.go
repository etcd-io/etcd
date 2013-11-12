package store

import (
	"time"
)

const (
	Get            = "get"
	Create         = "create"
	Set            = "set"
	Update         = "update"
	Delete         = "delete"
	CompareAndSwap = "compareAndSwap"
	Expire         = "expire"
)

type Event struct {
	Action        string     `json:"action"`
	Key           string     `json:"key, omitempty"`
	Dir           bool       `json:"dir,omitempty"`
	PrevValue     string     `json:"prevValue,omitempty"`
	Value         string     `json:"value,omitempty"`
	KVPairs       kvPairs    `json:"kvs,omitempty"`
	Expiration    *time.Time `json:"expiration,omitempty"`
	TTL           int64      `json:"ttl,omitempty"` // Time to live in second
	ModifiedIndex uint64     `json:"modifiedIndex"`
}

func newEvent(action string, key string, index uint64) *Event {
	return &Event{
		Action:        action,
		Key:           key,
		ModifiedIndex: index,
	}
}

func (e *Event) IsCreated() bool {
	if e.Action == Create {
		return true
	}

	if e.Action == Set && e.PrevValue == "" {
		return true
	}

	return false
}

func (e *Event) Index() uint64 {
	return e.ModifiedIndex
}

// Converts an event object into a response object.
func (event *Event) Response() interface{} {
	if !event.Dir {
		response := &Response{
			Action:     event.Action,
			Key:        event.Key,
			Value:      event.Value,
			PrevValue:  event.PrevValue,
			Index:      event.ModifiedIndex,
			TTL:        event.TTL,
			Expiration: event.Expiration,
		}

		if response.Action == Set {
			if response.PrevValue == "" {
				response.NewKey = true
			}
		}

		if response.Action == CompareAndSwap || response.Action == Create {
			response.Action = "testAndSet"
		}

		return response
	} else {
		responses := make([]*Response, len(event.KVPairs))

		for i, kv := range event.KVPairs {
			responses[i] = &Response{
				Action: event.Action,
				Key:    kv.Key,
				Value:  kv.Value,
				Dir:    kv.Dir,
				Index:  event.ModifiedIndex,
			}
		}
		return responses
	}
}
