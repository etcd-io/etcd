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
	desc
	failureCase   rpcpb.FailureCase
	injectMember  injectMemberFunc
	recoverMember recoverMemberFunc
}

func (f *failureByFunc) Desc() string {
	if string(f.desc) != "" {
		return string(f.desc)
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

func (f *failureFollower) FailureCase() rpcpb.FailureCase { return f.failureCase }

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

type failureQuorum failureByFunc

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

func (f *failureQuorum) FailureCase() rpcpb.FailureCase { return f.failureCase }

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

func (f *failureAll) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

// failureUntilSnapshot injects a failure and waits for a snapshot event
type failureUntilSnapshot struct {
	desc        desc
	failureCase rpcpb.FailureCase

	Failure
}

const snapshotCount = 10000

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
	if f.desc.Desc() != "" {
		return f.desc.Desc()
	}
	return f.failureCase.String()
}

func (f *failureUntilSnapshot) FailureCase() rpcpb.FailureCase {
	return f.failureCase
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

type desc string

func (d desc) Desc() string { return string(d) }
