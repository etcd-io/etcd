package main

import (
	"encoding/json"
)

type peerStats struct {
	Latency    float64 `json:"latency"`
	AvgLatency float64 `json:"averageLatency"`
	FailCnt    uint64  `json:"failsCount"`
	SuccCnt    uint64  `json:"successCount"`
}

func (r *raftServer) Stats() []byte {
	b, _ := json.Marshal(r.peersStats)
	return b
}
