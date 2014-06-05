package server

import (
	"sync"
	"time"

	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

type raftServerStats struct {
	Name      string    `json:"name"`
	State     string    `json:"state"`
	StartTime time.Time `json:"startTime"`

	LeaderInfo struct {
		Name      string    `json:"leader"`
		Uptime    string    `json:"uptime"`
		StartTime time.Time `json:"startTime"`
	} `json:"leaderInfo"`

	RecvAppendRequestCnt uint64  `json:"recvAppendRequestCnt,"`
	RecvingPkgRate       float64 `json:"recvPkgRate,omitempty"`
	RecvingBandwidthRate float64 `json:"recvBandwidthRate,omitempty"`

	SendAppendRequestCnt uint64  `json:"sendAppendRequestCnt"`
	SendingPkgRate       float64 `json:"sendPkgRate,omitempty"`
	SendingBandwidthRate float64 `json:"sendBandwidthRate,omitempty"`

	sendRateQueue *statsQueue
	recvRateQueue *statsQueue

	sync.Mutex
}

func NewRaftServerStats(name string) *raftServerStats {
	stats := &raftServerStats{
		Name:      name,
		StartTime: time.Now(),
		sendRateQueue: &statsQueue{
			back: -1,
		},
		recvRateQueue: &statsQueue{
			back: -1,
		},
	}
	stats.LeaderInfo.StartTime = time.Now()
	return stats
}

func (ss *raftServerStats) RecvAppendReq(leaderName string, pkgSize int) {
	ss.Lock()
	defer ss.Unlock()

	ss.State = raft.Follower
	if leaderName != ss.LeaderInfo.Name {
		ss.LeaderInfo.Name = leaderName
		ss.LeaderInfo.StartTime = time.Now()
	}

	ss.recvRateQueue.Insert(NewPackageStats(time.Now(), pkgSize))
	ss.RecvAppendRequestCnt++
}

func (ss *raftServerStats) SendAppendReq(pkgSize int) {
	ss.Lock()
	defer ss.Unlock()

	now := time.Now()

	if ss.State != raft.Leader {
		ss.State = raft.Leader
		ss.LeaderInfo.Name = ss.Name
		ss.LeaderInfo.StartTime = now
	}

	ss.sendRateQueue.Insert(NewPackageStats(now, pkgSize))

	ss.SendAppendRequestCnt++
}
