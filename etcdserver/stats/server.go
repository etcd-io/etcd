package stats

import (
	"sync"
	"time"

	"github.com/coreos/etcd/raft"
)

// ServerStats encapsulates various statistics about an EtcdServer and its
// communication with other members of the cluster
type ServerStats struct {
	Name string `json:"name"`
	// TODO(jonboulle): use ID instead of name?
	ID        string         `json:"id"`
	State     raft.StateType `json:"state"`
	StartTime time.Time      `json:"startTime"`

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

// Initialize clears the statistics of ServerStats and resets its start time
func (ss *ServerStats) Initialize() {
	if ss == nil {
		return
	}
	now := time.Now()
	ss.StartTime = now
	ss.LeaderInfo.StartTime = now
	ss.sendRateQueue = &statsQueue{
		back: -1,
	}
	ss.recvRateQueue = &statsQueue{
		back: -1,
	}
}

// RecvRates calculates and returns the rate of received append requests
func (ss *ServerStats) RecvRates() (float64, float64) {
	return ss.recvRateQueue.Rate()
}

// SendRates calculates and returns the rate of sent append requests
func (ss *ServerStats) SendRates() (float64, float64) {
	return ss.sendRateQueue.Rate()
}

// RecvAppendReq updates the ServerStats in response to an AppendRequest
// from the given leader being received
func (ss *ServerStats) RecvAppendReq(leader string, reqSize int) {
	ss.Lock()
	defer ss.Unlock()

	now := time.Now()

	ss.State = raft.StateFollower
	if leader != ss.LeaderInfo.Name {
		ss.LeaderInfo.Name = leader
		ss.LeaderInfo.StartTime = now
	}

	ss.recvRateQueue.Insert(
		&RequestStats{
			SendingTime: now,
			Size:        reqSize,
		},
	)
	ss.RecvAppendRequestCnt++
}

// SendAppendReq updates the ServerStats in response to an AppendRequest
// being sent by this server
func (ss *ServerStats) SendAppendReq(reqSize int) {
	ss.Lock()
	defer ss.Unlock()

	now := time.Now()

	if ss.State != raft.StateLeader {
		ss.State = raft.StateLeader
		ss.LeaderInfo.Name = ss.ID
		ss.LeaderInfo.StartTime = now
	}

	ss.sendRateQueue.Insert(
		&RequestStats{
			SendingTime: now,
			Size:        reqSize,
		},
	)

	ss.SendAppendRequestCnt++
}
