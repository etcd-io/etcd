package server

import (
	"math"
	"time"
)

type raftFollowersStats struct {
	Leader    string                        `json:"leader"`
	Followers map[string]*raftFollowerStats `json:"followers"`
}

func NewRaftFollowersStats(name string) *raftFollowersStats {
	return &raftFollowersStats{
		Leader:    name,
		Followers: make(map[string]*raftFollowerStats),
	}
}

type raftFollowerStats struct {
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

// Succ function update the raftFollowerStats with a successful send
func (ps *raftFollowerStats) Succ(d time.Duration) {
	total := float64(ps.Counts.Success) * ps.Latency.Average
	totalSquare := float64(ps.Counts.Success) * ps.Latency.averageSquare

	ps.Counts.Success++

	ps.Latency.Current = float64(d) / (1000000.0)

	if ps.Latency.Current > ps.Latency.Maximum {
		ps.Latency.Maximum = ps.Latency.Current
	}

	if ps.Latency.Current < ps.Latency.Minimum {
		ps.Latency.Minimum = ps.Latency.Current
	}

	ps.Latency.Average = (total + ps.Latency.Current) / float64(ps.Counts.Success)
	ps.Latency.averageSquare = (totalSquare + ps.Latency.Current*ps.Latency.Current) / float64(ps.Counts.Success)

	// sdv = sqrt(avg(x^2) - avg(x)^2)
	ps.Latency.StandardDeviation = math.Sqrt(ps.Latency.averageSquare - ps.Latency.Average*ps.Latency.Average)
}

// Fail function update the raftFollowerStats with a unsuccessful send
func (ps *raftFollowerStats) Fail() {
	ps.Counts.Fail++
}
