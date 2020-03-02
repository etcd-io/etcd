// Copyright 2019 The etcd Authors
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

package raft

import (
	"testing"

	"go.etcd.io/etcd/raft/tracker"
)

func next_delegate_and_targets(groups *Groups) (uint64, []uint64) {
	var delegate uint64
	targets := make([]uint64, 0)
	for k, v := range groups.BcastTargets {
		delegate = k
		targets = v
	}
	return delegate, targets
}

func TestGroupSystem(t *testing.T) {
	groups := NewGroups([]Group{
		{
			ID:      1,
			Members: []uint64{1, 2},
		}, {
			ID:      2,
			Members: []uint64{3, 4, 5},
		}, {
			ID:      3,
			Members: []uint64{6},
		},
	})
	if len(groups.unresolvedPeers) != 6 {
		t.Fatalf("expect unsolved peers length: %d, but got: %d", 6, len(groups.unresolvedPeers))
	}
	groups.LeaderGroupID = 1
	prs := tracker.MakeProgressTracker(256)
	for i := 1; i <= 6; i++ {
		prs.Progress[uint64(i)] = &tracker.Progress{
			Match:     99,
			Next:      100,
			Inflights: tracker.NewInflights(100),
		}
	}
	groups.ResolveDelegates(prs.Progress)
	if len(groups.BcastTargets) != 1 || groups.IsDelegated(1) || groups.IsDelegated(6) {
		t.Fatalf("only the delegate of group 2 should be picked")
	}

	// Remove a delegate which doesn't exists
	groups.RemoveDelegate(6)
	if len(groups.unresolvedPeers) != 0 {
		t.Fatalf("All the peers should be resolved")
	}

	delegate, targets := next_delegate_and_targets(&groups)
	if len(targets) != 2 {
		t.Fatalf("Unexpected number of peers delegated")
	}

	// Remove a delegated peer but not a delegate
	var to_be_removed uint64
	switch delegate {
	case 3:
		to_be_removed = 4
	case 4:
		to_be_removed = 5
	case 5:
		to_be_removed = 3
	default:
		t.Fatalf("Unexpected delegate")

	}
	groups.RemoveDelegate(to_be_removed)
	if len(groups.unresolvedPeers) != 0 {
		t.Fatalf("All the peers should be resolved")
	}

	// Remove a delegate from group system
	groups.RemoveDelegate(delegate)
	if len(groups.unresolvedPeers) != 2 {
		t.Fatalf("Peers in group 2 should become unsolved after the delegate being removed")
	}
	groups.ResolveDelegates(prs.Progress)
	_, targets = next_delegate_and_targets(&groups)
	if len(targets) != 1 {
		t.Fatalf("The delegate must be picked even if there're only 2 peers in group2")
	}

	// Add the removed peer back, but without group id
	readded := delegate
	groups.UpdateGroup(readded, None)
	if len(groups.unresolvedPeers) != 0 {
		t.Fatalf("UpdateGroup with a invalid group id will be ignored")
	}

	// Add the removed peer back with group id
	groups.UpdateGroup(readded, 2)
	delegate, targets = next_delegate_and_targets(&groups)
	if len(targets) != 2 {
		t.Fatalf("The peer should be included in groups system after invalid UpdateGroup")
	}
	if delegate == readded {
		t.Fatalf("A new added peer should never changes the delegate in current group")
	}

	// Remove peer from group system if it's join in again with a invalid Group id
	groups.UpdateGroup(readded, None)
	_, targets = next_delegate_and_targets(&groups)
	if len(targets) != 1 {
		t.Fatalf("An already delegated peer can be removed")
	}

	// Move delegate peer to group 3
	move_to_new_group := delegate
	if !groups.UpdateGroup(move_to_new_group, 3) && len(groups.BcastTargets) != 0 {
		t.Fatalf("The delegated peers should become unsolved if the delegate's group is updated")
	}
}
