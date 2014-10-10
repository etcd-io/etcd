package stats

import (
	"math"
	"time"
)

// LeaderStats is used by the leader in an etcd cluster, and encapsulates
// statistics about communication with its followers
type LeaderStats struct {
	// TODO(jonboulle): clarify that these are IDs, not names
	Leader    string                    `json:"leader"`
	Followers map[string]*FollowerStats `json:"followers"`
}

// NewLeaderStats generates a new LeaderStats with the given id as leader
func NewLeaderStats(id string) *LeaderStats {
	return &LeaderStats{
		Leader:    id,
		Followers: make(map[string]*FollowerStats),
	}
}

// FollowerStats encapsulates various statistics about a follower in an etcd cluster
type FollowerStats struct {
	Latency struct {
		Current           float64 `json:"current"`
		Average           float64 `json:"average"`
		averageSquare     float64
		StandardDeviation float64 `json:"standardDeviation"`
		Minimum           float64 `json:"minimum"`
		Maximum           float64 `json:"maximum"`
	} `json:"latency"`

	Counts struct {
		Fail    uint64 `json:"fail"`
		Success uint64 `json:"success"`
	} `json:"counts"`
}

// Succ updates the FollowerStats with a successful send
func (fs *FollowerStats) Succ(d time.Duration) {
	total := float64(fs.Counts.Success) * fs.Latency.Average
	totalSquare := float64(fs.Counts.Success) * fs.Latency.averageSquare

	fs.Counts.Success++

	fs.Latency.Current = float64(d) / (1000000.0)

	if fs.Latency.Current > fs.Latency.Maximum {
		fs.Latency.Maximum = fs.Latency.Current
	}

	if fs.Latency.Current < fs.Latency.Minimum {
		fs.Latency.Minimum = fs.Latency.Current
	}

	fs.Latency.Average = (total + fs.Latency.Current) / float64(fs.Counts.Success)
	fs.Latency.averageSquare = (totalSquare + fs.Latency.Current*fs.Latency.Current) / float64(fs.Counts.Success)

	// sdv = sqrt(avg(x^2) - avg(x)^2)
	fs.Latency.StandardDeviation = math.Sqrt(fs.Latency.averageSquare - fs.Latency.Average*fs.Latency.Average)
}

// Fail updates the FollowerStats with an unsuccessful send
func (fs *FollowerStats) Fail() {
	fs.Counts.Fail++
}
