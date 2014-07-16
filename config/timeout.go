package config

const (
	// The amount of time (in ms) to elapse without a heartbeat before becoming a candidate
	defaultElectionTimeout = 1200

	// The frequency (in ms) by which heartbeats are sent to followers.
	defaultHeartbeatInterval = 50
)
