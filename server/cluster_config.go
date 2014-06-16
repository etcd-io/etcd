package server

import (
	"time"
)

const (
	// DefaultActiveSize is the default number of active followers allowed.
	DefaultActiveSize = 9

	// MinActiveSize is the minimum active size allowed.
	MinActiveSize = 3

	// DefaultRemoveDelay is the default elapsed time before removal.
	DefaultRemoveDelay = float64((30 * time.Minute) / time.Second)

	// MinRemoveDelay is the minimum remove delay allowed.
	MinRemoveDelay = float64((2 * time.Second) / time.Second)

	// DefaultSyncInterval is the default interval for cluster sync.
	DefaultSyncInterval = float64((5 * time.Second) / time.Second)

	// MinSyncInterval is the minimum sync interval allowed.
	MinSyncInterval = float64((1 * time.Second) / time.Second)
)

// ClusterConfig represents cluster-wide configuration settings.
type ClusterConfig struct {
	// ActiveSize is the maximum number of node that can join as Raft followers.
	// Nodes that join the cluster after the limit is reached are standbys.
	ActiveSize int `json:"activeSize"`

	// RemoveDelay is the amount of time, in seconds, after a node is
	// unreachable that it will be swapped out as a standby node.
	RemoveDelay float64 `json:"removeDelay"`

	// SyncInterval is the amount of time, in seconds, between
	// cluster sync when it runs in standby mode.
	SyncInterval float64 `json:"syncInterval"`
}

// NewClusterConfig returns a cluster configuration with default settings.
func NewClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		ActiveSize:   DefaultActiveSize,
		RemoveDelay:  DefaultRemoveDelay,
		SyncInterval: DefaultSyncInterval,
	}
}
