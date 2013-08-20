package main

import (
	"math"
	"time"

	"github.com/coreos/go-raft"
)

type raftServerStats struct {
	State                string
	StartTime            time.Time
	Leader               string
	leaderStartTime      time.Time
	LeaderUptime         time.Duration
	RecvAppendRequestCnt uint64
	SendAppendRequestCnt uint64
}

func (ss *raftServerStats) RecvAppendReq(leaderName string) {
	ss.State = raft.Follower
	if leaderName != ss.Leader {
		ss.Leader = leaderName
		ss.leaderStartTime = time.Now()
	}

	ss.RecvAppendRequestCnt++
}

func (ss *raftServerStats) SendAppendReq() {
	if ss.State != raft.Leader {
		ss.State = raft.Leader
		ss.Leader = r.Name()
		ss.leaderStartTime = time.Now()
	}

	ss.SendAppendRequestCnt++
}

type raftPeerStats struct {
	Latency          float64 `json:"latency"`
	AvgLatency       float64 `json:"averageLatency"`
	avgLatencySquare float64
	SdvLatency       float64 `json:"sdvLatency"`
	MinLatency       float64 `json:"minLatency"`
	MaxLatency       float64 `json:"maxLatency"`
	FailCnt          uint64  `json:"failsCount"`
	SuccCnt          uint64  `json:"successCount"`
}

func (ps *raftPeerStats) Fail() {
	ps.FailCnt++
}

func (ps *raftPeerStats) Succ(d time.Duration) {

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
