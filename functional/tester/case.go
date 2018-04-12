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

// Case defines failure/test injection interface.
// To add a test case:
//  1. implement "Case" interface
//  2. define fail case name in "rpcpb.Case"
type Case interface {
	// Inject injeccts the failure into the testing cluster at the given
	// round. When calling the function, the cluster should be in health.
	Inject(clus *Cluster) error
	// Recover recovers the injected failure caused by the injection of the
	// given round and wait for the recovery of the testing cluster.
	Recover(clus *Cluster) error
	// Desc returns a description of the failure
	Desc() string
	// TestCase returns "rpcpb.Case" enum type.
	TestCase() rpcpb.Case
}

type injectMemberFunc func(*Cluster, int) error
type recoverMemberFunc func(*Cluster, int) error

type caseByFunc struct {
	desc          string
	rpcpbCase     rpcpb.Case
	injectMember  injectMemberFunc
	recoverMember recoverMemberFunc
}

func (c *caseByFunc) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	return c.rpcpbCase.String()
}

func (c *caseByFunc) TestCase() rpcpb.Case {
	return c.rpcpbCase
}

type caseFollower struct {
	caseByFunc
	last int
	lead int
}

func (c *caseFollower) updateIndex(clus *Cluster) error {
	lead, err := clus.GetLeader()
	if err != nil {
		return err
	}
	c.lead = lead

	n := len(clus.Members)
	if c.last == -1 { // first run
		c.last = clus.rd % n
		if c.last == c.lead {
			c.last = (c.last + 1) % n
		}
	} else {
		c.last = (c.last + 1) % n
		if c.last == c.lead {
			c.last = (c.last + 1) % n
		}
	}
	return nil
}

func (c *caseFollower) Inject(clus *Cluster) error {
	if err := c.updateIndex(clus); err != nil {
		return err
	}
	return c.injectMember(clus, c.last)
}

func (c *caseFollower) Recover(clus *Cluster) error {
	return c.recoverMember(clus, c.last)
}

func (c *caseFollower) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	return c.rpcpbCase.String()
}

func (c *caseFollower) TestCase() rpcpb.Case {
	return c.rpcpbCase
}

type caseLeader struct {
	caseByFunc
	last int
	lead int
}

func (c *caseLeader) updateIndex(clus *Cluster) error {
	lead, err := clus.GetLeader()
	if err != nil {
		return err
	}
	c.lead = lead
	c.last = lead
	return nil
}

func (c *caseLeader) Inject(clus *Cluster) error {
	if err := c.updateIndex(clus); err != nil {
		return err
	}
	return c.injectMember(clus, c.last)
}

func (c *caseLeader) Recover(clus *Cluster) error {
	return c.recoverMember(clus, c.last)
}

func (c *caseLeader) TestCase() rpcpb.Case {
	return c.rpcpbCase
}

type caseQuorum struct {
	caseByFunc
	injected map[int]struct{}
}

func (c *caseQuorum) Inject(clus *Cluster) error {
	c.injected = pickQuorum(len(clus.Members))
	for idx := range c.injected {
		if err := c.injectMember(clus, idx); err != nil {
			return err
		}
	}
	return nil
}

func (c *caseQuorum) Recover(clus *Cluster) error {
	for idx := range c.injected {
		if err := c.recoverMember(clus, idx); err != nil {
			return err
		}
	}
	return nil
}

func (c *caseQuorum) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	return c.rpcpbCase.String()
}

func (c *caseQuorum) TestCase() rpcpb.Case {
	return c.rpcpbCase
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

type caseAll caseByFunc

func (c *caseAll) Inject(clus *Cluster) error {
	for i := range clus.Members {
		if err := c.injectMember(clus, i); err != nil {
			return err
		}
	}
	return nil
}

func (c *caseAll) Recover(clus *Cluster) error {
	for i := range clus.Members {
		if err := c.recoverMember(clus, i); err != nil {
			return err
		}
	}
	return nil
}

func (c *caseAll) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	return c.rpcpbCase.String()
}

func (c *caseAll) TestCase() rpcpb.Case {
	return c.rpcpbCase
}

// caseUntilSnapshot injects a failure/test and waits for a snapshot event
type caseUntilSnapshot struct {
	desc      string
	rpcpbCase rpcpb.Case
	Case
}

// all delay failure cases except the ones failing with latency
// greater than election timeout (trigger leader election and
// cluster keeps operating anyways)
var slowCases = map[rpcpb.Case]bool{
	rpcpb.Case_RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER:                        true,
	rpcpb.Case_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT:        true,
	rpcpb.Case_RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT: true,
	rpcpb.Case_RANDOM_DELAY_PEER_PORT_TX_RX_LEADER:                              true,
	rpcpb.Case_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT:              true,
	rpcpb.Case_RANDOM_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT:       true,
	rpcpb.Case_RANDOM_DELAY_PEER_PORT_TX_RX_QUORUM:                              true,
	rpcpb.Case_RANDOM_DELAY_PEER_PORT_TX_RX_ALL:                                 true,
}

func (c *caseUntilSnapshot) Inject(clus *Cluster) error {
	if err := c.Case.Inject(clus); err != nil {
		return err
	}

	snapshotCount := clus.Members[0].Etcd.SnapshotCount

	now := time.Now()
	clus.lg.Info(
		"trigger snapshot START",
		zap.String("desc", c.Desc()),
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
	if v, ok := slowCases[c.TestCase()]; v && ok {
		// slow network takes more retries
		retries *= 5
	}

	for i := 0; i < retries; i++ {
		lastRev, _ = clus.maxRev()
		// If the number of proposals committed is bigger than snapshot count,
		// a new snapshot should have been created.
		dicc := lastRev - startRev
		if dicc > snapshotCount {
			clus.lg.Info(
				"trigger snapshot PASS",
				zap.Int("retries", i),
				zap.String("desc", c.Desc()),
				zap.Int64("committed-entries", dicc),
				zap.Int64("etcd-snapshot-count", snapshotCount),
				zap.Int64("last-revision", lastRev),
				zap.Duration("took", time.Since(now)),
			)
			return nil
		}

		clus.lg.Info(
			"trigger snapshot PROGRESS",
			zap.Int("retries", i),
			zap.Int64("committed-entries", dicc),
			zap.Int64("etcd-snapshot-count", snapshotCount),
			zap.Int64("last-revision", lastRev),
			zap.Duration("took", time.Since(now)),
		)
		time.Sleep(time.Second)
	}

	return fmt.Errorf("cluster too slow: only %d commits in %d retries", lastRev-startRev, retries)
}

func (c *caseUntilSnapshot) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	if c.rpcpbCase.String() != "" {
		return c.rpcpbCase.String()
	}
	return c.Case.Desc()
}

func (c *caseUntilSnapshot) TestCase() rpcpb.Case {
	return c.rpcpbCase
}
