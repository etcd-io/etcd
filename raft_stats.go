/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
		ss.LeaderInfo.Name = r.Name()
		ss.LeaderInfo.startTime = now
	}

	ss.sendRateQueue.Insert(NewPackageStats(now, pkgSize))

	ss.SendAppendRequestCnt++
}

type raftFollowersStats struct {
	Leader    string                        `json:"leader"`
	Followers map[string]*raftFollowerStats `json:"followers"`
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
