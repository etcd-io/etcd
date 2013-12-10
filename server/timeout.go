package server

import (
	"time"
)

const (
	// The amount of time to elapse without a heartbeat before becoming a candidate
	defaultElectionTimeout = 200 * time.Millisecond

	// The frequency by which heartbeats are sent to followers.
	defaultHeartbeatTimeout = 50 * time.Millisecond
)
