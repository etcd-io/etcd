package config

import "time"

const (
	// The amount of time (in ms) to elapse without a heartbeat before becoming a candidate
	DefaultElectionTimeout = 200

	// The frequency (in ms) by which heartbeats are sent to followers.
	DefaultHeartbeatInterval = 50

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

	DefaultReadTimeout  = float64((5 * time.Minute) / time.Second)
	DefaultWriteTimeout = float64((5 * time.Minute) / time.Second)
)
