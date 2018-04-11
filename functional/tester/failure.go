// Copyright 2018 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tester

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/coreos/etcd/functional/rpcpb"

	"go.uber.org/zap"
)

// Failure defines failure injection interface.
// To add a fail case:
//  1. implement "Failure" interface
//  2. define fail case name in "rpcpb.FailureCase"
type Failure interface {
	// Inject injeccts the failure into the testing cluster at the given
	// round. When calling the function, the cluster should be in health.
	Inject(clus *Cluster) error
	// Recover recovers the injected failure caused by the injection of the
	// given round and wait for the recovery of the testing cluster.
	Recover(clus *Cluster) error
	// Desc returns a description of the failure
	Desc() string
	// FailureCase returns "rpcpb.FailureCase" enum type.
	FailureCase() rpcpb.FailureCase
}

type injectMemberFunc func(*Cluster, int) error
type recoverMemberFunc func(*Cluster, int) error

type failureByFunc struct {
	desc          string
	failureCase   rpcpb.FailureCase
	injectMember  injectMemberFunc
	recoverMember recoverMemberFunc
}

func (f *failureByFunc) Desc() string {
	if f.desc != "" {
		return f.desc
	}
	return f.failureCase.String()
}

func (f *failureByFunc) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

type failureFollower struct {
	failureByFunc
	last int
	lead int
}

func (f *failureFollower) updateIndex(clus *Cluster) error {
	idx, err := clus.GetLeader()
	if err != nil {
		return err
	}
	f.lead = idx

	n := len(clus.Members)
	if f.last == -1 { // first run
		f.last = clus.rd % n
		if f.last == f.lead {
			f.last = (f.last + 1) % n
		}
	} else {
		f.last = (f.last + 1) % n
		if f.last == f.lead {
			f.last = (f.last + 1) % n
		}
	}
	return nil
}

func (f *failureFollower) Inject(clus *Cluster) error {
	if err := f.updateIndex(clus); err != nil {
		return err
	}
	return f.injectMember(clus, f.last)
}

func (f *failureFollower) Recover(clus *Cluster) error {
	return f.recoverMember(clus, f.last)
}

func (f *failureFollower) Desc() string {
	if f.desc != "" {
		return f.desc
	}
	return f.failureCase.String()
}

func (f *failureFollower) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

type failureLeader struct {
	failureByFunc
	last int
	lead int
}

func (f *failureLeader) updateIndex(clus *Cluster) error {
	idx, err := clus.GetLeader()
	if err != nil {
		return err
	}
	f.lead = idx
	f.last = idx
	return nil
}

func (f *failureLeader) Inject(clus *Cluster) error {
	if err := f.updateIndex(clus); err != nil {
		return err
	}
	return f.injectMember(clus, f.last)
}

func (f *failureLeader) Recover(clus *Cluster) error {
	return f.recoverMember(clus, f.last)
}

func (f *failureLeader) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

type failureQuorum struct {
	failureByFunc
	injected map[int]struct{}
}

func (f *failureQuorum) Inject(clus *Cluster) error {
	f.injected = pickQuorum(len(clus.Members))
	for idx := range f.injected {
		if err := f.injectMember(clus, idx); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureQuorum) Recover(clus *Cluster) error {
	for idx := range f.injected {
		if err := f.recoverMember(clus, idx); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureQuorum) Desc() string {
	if f.desc != "" {
		return f.desc
	}
	return f.failureCase.String()
}

func (f *failureQuorum) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

func pickQuorum(size int) (picked map[int]struct{}) {
	picked = make(map[int]struct{})
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	quorum := size/2 + 1
	for len(picked) < quorum {
		idx := r.Intn(size)
		picked[idx] = struct{}{}
	}
	return picked
}

type failureAll failureByFunc

func (f *failureAll) Inject(clus *Cluster) error {
	for i := range clus.Members {
		if err := f.injectMember(clus, i); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureAll) Recover(clus *Cluster) error {
	for i := range clus.Members {
		if err := f.recoverMember(clus, i); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureAll) Desc() string {
	if f.desc != "" {
		return f.desc
	}
	return f.failureCase.String()
}

func (f *failureAll) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

// failureUntilSnapshot injects a failure and waits for a snapshot event
type failureUntilSnapshot struct {
	desc        string
	failureCase rpcpb.FailureCase
	Failure
}

// all delay failure cases except the ones failing with latency
// greater than election timeout (trigger leader election and
// cluster keeps operating anyways)
var slowCases = map[rpcpb.FailureCase]bool{
	rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER:                        true,
	rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT:        true,
	rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT: true,
	rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_LEADER:                              true,
	rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT:              true,
	rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT:       true,
	rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_QUORUM:                              true,
	rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_ALL:                                 true,
}

func (f *failureUntilSnapshot) Inject(clus *Cluster) error {
	if err := f.Failure.Inject(clus); err != nil {
		return err
	}

	snapshotCount := clus.Members[0].Etcd.SnapshotCount

	now := time.Now()
	clus.lg.Info(
		"trigger snapshot START",
		zap.String("desc", f.Desc()),
		zap.Int64("etcd-snapshot-count", snapshotCount),
	)

	// maxRev may fail since failure just injected, retry if failed.
	startRev, err := clus.maxRev()
	for i := 0; i < 10 && startRev == 0; i++ {
		startRev, err = clus.maxRev()
	}
	if startRev == 0 {
		return err
	}
	lastRev := startRev

	// healthy cluster could accept 1000 req/sec at least.
	// 3x time to trigger snapshot.
	retries := int(snapshotCount) / 1000 * 3
	if v, ok := slowCases[f.FailureCase()]; v && ok {
		// slow network takes more retries
		retries *= 5
	}

	for i := 0; i < retries; i++ {
		lastRev, _ = clus.maxRev()
		// If the number of proposals committed is bigger than snapshot count,
		// a new snapshot should have been created.
		diff := lastRev - startRev
		if diff > snapshotCount {
			clus.lg.Info(
				"trigger snapshot PASS",
				zap.Int("retries", i),
				zap.String("desc", f.Desc()),
				zap.Int64("committed-entries", diff),
				zap.Int64("etcd-snapshot-count", snapshotCount),
				zap.Int64("last-revision", lastRev),
				zap.Duration("took", time.Since(now)),
			)
			return nil
		}

		clus.lg.Info(
			"trigger snapshot PROGRESS",
			zap.Int("retries", i),
			zap.Int64("committed-entries", diff),
			zap.Int64("etcd-snapshot-count", snapshotCount),
			zap.Int64("last-revision", lastRev),
			zap.Duration("took", time.Since(now)),
		)
		time.Sleep(time.Second)
	}

	return fmt.Errorf("cluster too slow: only %d commits in %d retries", lastRev-startRev, retries)
}

func (f *failureUntilSnapshot) Desc() string {
	if f.desc != "" {
		return f.desc
	}
	if f.failureCase.String() != "" {
		return f.failureCase.String()
	}
	return f.Failure.Desc()
}

func (f *failureUntilSnapshot) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}
