package main

import (
	"math"
	"sync"
	"time"

	"github.com/coreos/go-raft"
)

const (
	queueCapacity = 200
)

// packageStats represent the stats we need for a package.
// It has sending time and the size of the package.
type packageStats struct {
	sendingTime time.Time
	size        int
}

// NewPackageStats creates a pacakgeStats and return the pointer to it.
func NewPackageStats(now time.Time, size int) *packageStats {
	return &packageStats{
		sendingTime: now,
		size:        size,
	}
}

// Time return the sending time of the package.
func (ps *packageStats) Time() time.Time {
	return ps.sendingTime
}

type raftServerStats struct {
	State        string    `json:"state"`
	StartTime    time.Time `json:"startTime"`
	Leader       string    `json:"leader"`
	LeaderUptime string    `json:"leaderUptime"`

	RecvAppendRequestCnt uint64  `json:"recvAppendRequestCnt,"`
	RecvingPkgRate       float64 `json:"recvPkgRate,omitempty"`
	RecvingBandwidthRate float64 `json:"recvBandwidthRate,omitempty"`

	SendAppendRequestCnt uint64  `json:"sendAppendRequestCnt"`
	SendingPkgRate       float64 `json:"sendPkgRate,omitempty"`
	SendingBandwidthRate float64 `json:"sendBandwidthRate,omitempty"`

	leaderStartTime time.Time
	sendRateQueue   *statsQueue
	recvRateQueue   *statsQueue
}

func (ss *raftServerStats) RecvAppendReq(leaderName string, pkgSize int) {
	ss.State = raft.Follower
	if leaderName != ss.Leader {
		ss.Leader = leaderName
		ss.leaderStartTime = time.Now()
	}

	ss.recvRateQueue.Insert(NewPackageStats(time.Now(), pkgSize))
	ss.RecvAppendRequestCnt++
}

func (ss *raftServerStats) SendAppendReq(pkgSize int) {
	now := time.Now()
	if ss.State != raft.Leader {
		ss.State = raft.Leader
		ss.Leader = r.Name()
		ss.leaderStartTime = now
	}

	ss.sendRateQueue.Insert(NewPackageStats(time.Now(), pkgSize))

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

// Succ function update the raftPeerStats with a successful send
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

// Fail function update the raftPeerStats with a unsuccessful send
func (ps *raftPeerStats) Fail() {
	ps.FailCnt++
}

type statsQueue struct {
	items        [queueCapacity]*packageStats
	size         int
	front        int
	back         int
	totalPkgSize int
	rwl          sync.RWMutex
}

func (q *statsQueue) Len() int {
	return q.size
}

func (q *statsQueue) PkgSize() int {
	return q.totalPkgSize
}

// FrontAndBack gets the front and back elements in the queue
// We must grab front and back together with the protection of the lock
func (q *statsQueue) frontAndBack() (*packageStats, *packageStats) {
	q.rwl.RLock()
	defer q.rwl.RUnlock()
	if q.size != 0 {
		return q.items[q.front], q.items[q.back]
	}
	return nil, nil
}

// Insert function insert a packageStats into the queue and update the records
func (q *statsQueue) Insert(p *packageStats) {
	q.rwl.Lock()
	defer q.rwl.Unlock()

	q.back = (q.back + 1) % queueCapacity

	if q.size == queueCapacity { //dequeue
		q.totalPkgSize -= q.items[q.front].size
		q.front = (q.back + 1) % queueCapacity
	} else {
		q.size++
	}

	q.items[q.back] = p
	q.totalPkgSize += q.items[q.back].size

}

// Rate function returns the package rate and byte rate
func (q *statsQueue) Rate() (float64, float64) {
	front, back := q.frontAndBack()

	if front == nil || back == nil {
		return 0, 0
	}

	if time.Now().Sub(back.Time()) > time.Second {
		q.Clear()
		return 0, 0
	}

	sampleDuration := back.Time().Sub(front.Time())

	pr := float64(q.Len()) / float64(sampleDuration) * float64(time.Second)

	br := float64(q.PkgSize()) / float64(sampleDuration) * float64(time.Second)

	return pr, br
}

// Clear function clear up the statsQueue
func (q *statsQueue) Clear() {
	q.rwl.Lock()
	defer q.rwl.Unlock()
	q.back = -1
	q.front = 0
	q.size = 0
	q.totalPkgSize = 0
}
