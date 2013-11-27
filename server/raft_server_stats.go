package server

import (
	"time"

	"github.com/coreos/raft"
)

type raftServerStats struct {
	Name      string    `json:"name"`
	State     string    `json:"state"`
	StartTime time.Time `json:"startTime"`

	LeaderInfo struct {
		Name      string `json:"leader"`
		Uptime    string `json:"uptime"`
		startTime time.Time
	} `json:"leaderInfo"`

	RecvAppendRequestCnt uint64  `json:"recvAppendRequestCnt,"`
	RecvingPkgRate       float64 `json:"recvPkgRate,omitempty"`
	RecvingBandwidthRate float64 `json:"recvBandwidthRate,omitempty"`

	SendAppendRequestCnt uint64  `json:"sendAppendRequestCnt"`
	SendingPkgRate       float64 `json:"sendPkgRate,omitempty"`
	SendingBandwidthRate float64 `json:"sendBandwidthRate,omitempty"`

	sendRateQueue *statsQueue
	recvRateQueue *statsQueue
}

func (ss *raftServerStats) RecvAppendReq(leaderName string, pkgSize int) {
	ss.State = raft.Follower
	if leaderName != ss.LeaderInfo.Name {
		ss.LeaderInfo.Name = leaderName
		ss.LeaderInfo.startTime = time.Now()
	}

	ss.recvRateQueue.Insert(NewPackageStats(time.Now(), pkgSize))
	ss.RecvAppendRequestCnt++
}

func (ss *raftServerStats) SendAppendReq(pkgSize int) {
	now := time.Now()

	if ss.State != raft.Leader {
		ss.State = raft.Leader
		ss.LeaderInfo.Name = ss.Name
		ss.LeaderInfo.startTime = now
	}

	ss.sendRateQueue.Insert(NewPackageStats(now, pkgSize))

	ss.SendAppendRequestCnt++
}
