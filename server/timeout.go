package server

// import (
// 	"time"
// )

const (
	// The amount of time to elapse without a heartbeat before becoming a candidate.
	// ElectionTimeout = 200 * time.Millisecond
	ElectionTimeout = 200

	// The frequency by which heartbeats are sent to followers.
	//HeartbeatTimeout = 50 * time.Millisecond
	HeartbeatTimeout = 50

	RetryInterval = 10
)
