package main

import (
	"math"
	"time"
)

type peerStats struct {
	Latency          float64 `json:"latency"`
	AvgLatency       float64 `json:"averageLatency"`
	avgLatencySquare float64
	SdvLatency       float64 `json:"sdvLatency"`
	MinLatency       float64 `json:"minLatency"`
	MaxLatency       float64 `json:"maxLatency"`
	FailCnt          uint64  `json:"failsCount"`
	SuccCnt          uint64  `json:"successCount"`
}

func (ps *peerStats) Fail() {
	ps.FailCnt++
}

func (ps *peerStats) Succ(d time.Duration) {

	total := float64(ps.SuccCnt) * ps.AvgLatency
	totalSquare := float64(ps.SuccCnt) * ps.avgLatencySquare

	ps.SuccCnt++

	ps.Latency = float64(d) / (1000000.0)

	if ps.Latency > ps.MaxLatency {
		ps.MaxLatency = ps.Latency
	}

	if ps.Latency < ps.MinLatency {
		ps.MinLatency = ps.Latency
	}

	ps.AvgLatency = (total + ps.Latency) / float64(ps.SuccCnt)
	ps.avgLatencySquare = (totalSquare + ps.Latency*ps.Latency) / float64(ps.SuccCnt)

	// sdv = sqrt(avg(x^2) - avg(x)^2)
	ps.SdvLatency = math.Sqrt(ps.avgLatencySquare - ps.AvgLatency*ps.AvgLatency)
}
