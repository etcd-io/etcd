package store

import (
	"time"
)

// When user list a directory, we add all the node into key-value pair slice
type KeyValuePair struct {
	Key           string     `json:"key, omitempty"`
	Value         string     `json:"value,omitempty"`
	Dir           bool       `json:"dir,omitempty"`
	Expiration    *time.Time `json:"expiration,omitempty"`
	TTL           int64      `json:"ttl,omitempty"` // Time to live in second
	KVPairs       kvPairs    `json:"kvs,omitempty"`
	ModifiedIndex uint64     `json:"modifiedIndex,omitempty"`
}

type kvPairs []KeyValuePair

// interfaces for sorting
func (kvs kvPairs) Len() int {
	return len(kvs)
}

func (kvs kvPairs) Less(i, j int) bool {
	return kvs[i].Key < kvs[j].Key
}

func (kvs kvPairs) Swap(i, j int) {
	kvs[i], kvs[j] = kvs[j], kvs[i]
}
