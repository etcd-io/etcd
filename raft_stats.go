package main

import (
	"time"
)

type peerStats struct {
	Latency    float64 `json:"latency"`
	AvgLatency float64 `json:"averageLatency"`
	FailCnt    uint64  `json:"failsCount"`
	SuccCnt    uint64  `json:"successCount"`
}

func (ps *peerStats) Fail() {
	ps.FailCnt++
}

func (ps *peerStats) Succ(d time.Duration) {
	total := float64(ps.SuccCnt) * ps.AvgLatency
	ps.SuccCnt++
	ps.Latency = float64(d) / (1000000.0)
	ps.AvgLatency = (total + ps.Latency) / float64(ps.SuccCnt)
}
