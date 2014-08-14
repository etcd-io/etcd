package conf

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

func (c *ClusterConfig) Sanitize() {
	if c.ActiveSize < MinActiveSize {
		c.ActiveSize = MinActiveSize
	}
	if c.RemoveDelay < MinRemoveDelay {
		c.RemoveDelay = MinRemoveDelay
	}
	if c.SyncInterval < MinSyncInterval {
		c.SyncInterval = MinSyncInterval
	}
}
