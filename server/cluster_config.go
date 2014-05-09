package server

import (
	"time"
)

const (
	// DefaultActiveSize is the default number of active followers allowed.
	DefaultActiveSize = 9

	// MinActiveSize is the minimum active size allowed.
	MinActiveSize = 3

	// DefaultRemoveDelay is the default elapsed time before promotion.
	DefaultRemoveDelay = int((30 * time.Minute) / time.Second)

	// MinRemoveDelay is the minimum promote delay allowed.
	MinRemoveDelay = int((2 * time.Second) / time.Second)

	// DefaultSyncInterval is the default interval for cluster sync.
	DefaultSyncInterval = int((30 * time.Minute) / time.Second)

	// MinSyncInterval is the minimum sync interval allowed.
	MinSyncInterval = int((1 * time.Second) / time.Second)
)

// ClusterConfig represents cluster-wide configuration settings.
// These settings can only be changed through Raft.
type ClusterConfig struct {
	// ActiveSize is the maximum number of node that can join as Raft followers.
	// Nodes that join the cluster after the limit is reached are standbys.
	ActiveSize int `json:"activeSize"`

	// RemoveDelay is the amount of time, in seconds, after a node is
	// unreachable that it will be swapped out as a standby node.
	RemoveDelay int `json:"removeDelay"`

	// SyncInterval is the amount of time, in seconds, between
	// cluster sync when it runs in standby mode.
	SyncInterval int `json:"syncInterval"`
}

// NewClusterConfig returns a cluster configuration with default settings.
func NewClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		ActiveSize:   DefaultActiveSize,
		RemoveDelay:  DefaultRemoveDelay,
		SyncInterval: DefaultSyncInterval,
	}
}
