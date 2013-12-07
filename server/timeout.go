package server

const (
	// The amount of time (ms) to elapse without a heartbeat before becoming a candidate.
	ElectionTimeout = 200

	// The frequency by which heartbeats are sent to followers.
	HeartbeatTimeout = 50

	RetryInterval = 10
)
