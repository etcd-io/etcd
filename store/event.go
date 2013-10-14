package store

import (
	"time"
)

const (
	Get        = "get"
	Create     = "create"
	Update     = "update"
	Delete     = "delete"
	TestAndSet = "testAndSet"
	Expire     = "expire"
)

const (
	UndefIndex = 0
	UndefTerm  = 0
)

type Event struct {
	Action     string     `json:"action"`
	Key        string     `json:"key, omitempty"`
	Dir        bool       `json:"dir,omitempty"`
	PrevValue  string     `json:"prevValue,omitempty"`
	Value      string     `json:"value,omitempty"`
	KVPairs    kvPairs    `json:"kvs,omitempty"`
	Expiration *time.Time `json:"expiration,omitempty"`
	TTL        int64      `json:"ttl,omitempty"` // Time to live in second
	// The command index of the raft machine when the command is executed
	Index uint64 `json:"index"`
	Term  uint64 `json:"term"`
}

func newEvent(action string, key string, index uint64, term uint64) *Event {
	return &Event{
		Action: action,
		Key:    key,
		Index:  index,
		Term:   term,
	}
}

// Converts an event object into a response object.
func (event *Event) Response() interface{} {
	if !event.Dir {
		response := &Response{
			Action:     event.Action,
			Key:        event.Key,
			Value:      event.Value,
			PrevValue:  event.PrevValue,
			Index:      event.Index,
			TTL:        event.TTL,
			Expiration: event.Expiration,
		}

		if response.Action == Create || response.Action == Update {
			response.Action = "set"
			if response.PrevValue == "" {
				response.NewKey = true
			}
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
				Index:  event.Index,
			}
		}
		return responses
	}
}
