package stats

type Stats interface {
	// SelfStats returns the struct representing statistics of this server
	SelfStats() []byte
	// LeaderStats returns the statistics of all followers in the cluster
	// if this server is leader. Otherwise, nil is returned.
	LeaderStats() []byte
	// StoreStats returns statistics of the store backing this EtcdServer
	StoreStats() []byte
}
