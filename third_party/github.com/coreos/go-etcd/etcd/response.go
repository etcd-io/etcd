package etcd

import (
	"time"
)

// The response object from the server.
type Response struct {
	Action    string `json:"action"`
	Key       string `json:"key"`
	Dir       bool   `json:"dir,omitempty"`
	PrevValue string `json:"prevValue,omitempty"`
	Value     string `json:"value,omitempty"`

	// If the key did not exist before the action,
	// this field should be set to true
	NewKey bool `json:"newKey,omitempty"`

	Expiration *time.Time `json:"expiration,omitempty"`

	// Time to live in second
	TTL int64 `json:"ttl,omitempty"`

	// The command index of the raft machine when the command is executed
	Index uint64 `json:"index"`
}
