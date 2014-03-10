package server

import (
	"time"
)

const (
	// DefaultActiveSize is the default number of active followers allowed.
	DefaultActiveSize = 9

	// MinActiveSize is the minimum active size allowed.
	MinActiveSize = 3

	// DefaultPromoteDelay is the default elapsed time before promotion.
	DefaultPromoteDelay = int((30 * time.Minute) / time.Second)

	// MinPromoteDelay is the minimum promote delay allowed.
	MinPromoteDelay = int((2 * time.Second) / time.Second)
)

// ClusterConfig represents cluster-wide configuration settings.
// These settings can only be changed through Raft.
type ClusterConfig struct {
	// ActiveSize is the maximum number of node that can join as Raft followers.
	// Nodes that join the cluster after the limit is reached are proxies.
	ActiveSize int `json:"activeSize"`

	// PromoteDelay is the amount of time, in seconds, after a node is
	// unreachable that it will be swapped out for a proxy node, if available.
	PromoteDelay int `json:"promoteDelay"`
}

// NewClusterConfig returns a cluster configuration with default settings.
func NewClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		ActiveSize:   DefaultActiveSize,
		PromoteDelay: DefaultPromoteDelay,
	}
}
