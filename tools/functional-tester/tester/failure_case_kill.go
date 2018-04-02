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

	"github.com/coreos/etcd/tools/functional-tester/rpcpb"
)

const snapshotCount = 10000

func injectKill(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_KillEtcd)
}

func recoverKill(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_RestartEtcd)
}

func newFailureKillAll() Failure {
	return &failureAll{
		description:   "kill all members",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
}

func newFailureKillQuorum() Failure {
	return &failureQuorum{
		description:   "kill quorum of the cluster",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
}

func newFailureKillOne() Failure {
	return &failureOne{
		description:   "kill one random member",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
}

func newFailureKillLeader() Failure {
	ff := failureByFunc{
		description:   "kill leader member",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
	return &failureLeader{ff, 0}
}

func newFailureKillOneForLongTime() Failure {
	return &failureUntilSnapshot{newFailureKillOne()}
}

func newFailureKillLeaderForLongTime() Failure {
	return &failureUntilSnapshot{newFailureKillLeader()}
}

type description string

func (d description) Desc() string { return string(d) }

type injectMemberFunc func(*Cluster, int) error
type recoverMemberFunc func(*Cluster, int) error

type failureByFunc struct {
	description
	injectMember  injectMemberFunc
	recoverMember recoverMemberFunc
}

// TODO: support kill follower
type failureOne failureByFunc
type failureAll failureByFunc
type failureQuorum failureByFunc

type failureLeader struct {
	failureByFunc
	idx int
}

// failureUntilSnapshot injects a failure and waits for a snapshot event
type failureUntilSnapshot struct{ Failure }

func (f *failureOne) Inject(clus *Cluster) error {
	return f.injectMember(clus, clus.rd%len(clus.Members))
}

func (f *failureOne) Recover(clus *Cluster) error {
	if err := f.recoverMember(clus, clus.rd%len(clus.Members)); err != nil {
		return err
	}
	clus.logger.Info("wait health after recovering failureOne")
	return clus.WaitHealth()
}

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
	clus.logger.Info("wait health after recovering failureAll")
	return clus.WaitHealth()
}

func (f *failureQuorum) Inject(clus *Cluster) error {
	for i := range killMap(len(clus.Members), clus.rd) {
		if err := f.injectMember(clus, i); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureQuorum) Recover(clus *Cluster) error {
	for i := range killMap(len(clus.Members), clus.rd) {
		if err := f.recoverMember(clus, i); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureLeader) Inject(clus *Cluster) error {
	idx, err := clus.GetLeader()
	if err != nil {
		return err
	}
	f.idx = idx
	return f.injectMember(clus, idx)
}

func (f *failureLeader) Recover(clus *Cluster) error {
	if err := f.recoverMember(clus, f.idx); err != nil {
		return err
	}
	clus.logger.Info("wait health after recovering failureLeader")
	return clus.WaitHealth()
}

func (f *failureUntilSnapshot) Inject(clus *Cluster) error {
	if err := f.Failure.Inject(clus); err != nil {
		return err
	}
	if len(clus.Members) < 3 {
		return nil
	}
	// maxRev may fail since failure just injected, retry if failed.
	startRev, err := clus.maxRev()
	for i := 0; i < 10 && startRev == 0; i++ {
		startRev, err = clus.maxRev()
	}
	if startRev == 0 {
		return err
	}
	lastRev := startRev
	// Normal healthy cluster could accept 1000req/s at least.
	// Give it 3-times time to create a new snapshot.
	retry := snapshotCount / 1000 * 3
	for j := 0; j < retry; j++ {
		lastRev, _ = clus.maxRev()
		// If the number of proposals committed is bigger than snapshot count,
		// a new snapshot should have been created.
		if lastRev-startRev > snapshotCount {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("cluster too slow: only commit %d requests in %ds", lastRev-startRev, retry)
}

func (f *failureUntilSnapshot) Desc() string {
	return f.Failure.Desc() + " for a long time and expect it to recover from an incoming snapshot"
}

func killMap(size int, seed int) map[int]bool {
	m := make(map[int]bool)
	r := rand.New(rand.NewSource(int64(seed)))
	majority := size/2 + 1
	for {
		m[r.Intn(size)] = true
		if len(m) >= majority {
			return m
		}
	}
}
