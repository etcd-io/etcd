package raft

import (
	"math/rand"
	"time"
)

// Waits for a random time between two durations and sends the current time on
// the returned channel.
func afterBetween(min time.Duration, max time.Duration) <-chan time.Time {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	d, delta := min, (max - min)
	if delta > 0 {
		d += time.Duration(rand.Int63n(int64(delta)))
	}
	return time.After(d)
}
