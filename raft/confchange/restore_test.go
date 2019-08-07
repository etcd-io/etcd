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

package confchange

import (
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"testing/quick"

	pb "go.etcd.io/etcd/raft/raftpb"
	"go.etcd.io/etcd/raft/tracker"
)

type rndConfChange pb.ConfState

// Generate creates a random (valid) ConfState for use with quickcheck.
func (rndConfChange) Generate(rand *rand.Rand, _ int) reflect.Value {
	conv := func(sl []int) []uint64 {
		// We want IDs but the incoming slice is zero-indexed, so add one to
		// each.
		out := make([]uint64, len(sl))
		for i := range sl {
			out[i] = uint64(sl[i] + 1)
		}
		return out
	}
	var cs pb.ConfState
	// NB: never generate the empty ConfState, that one should be unit tested.
	nVoters := 1 + rand.Intn(5)

	nLearners := rand.Intn(5)
	// The number of voters that are in the outgoing config but not in the
	// incoming one. (We'll additionally retain a random number of the
	// incoming voters below).
	nRemovedVoters := rand.Intn(3)

	// Voters, learners, and removed voters must not overlap. A "removed voter"
	// is one that we have in the outgoing config but not the incoming one.
	ids := conv(rand.Perm(2 * (nVoters + nLearners + nRemovedVoters)))

	cs.Voters = ids[:nVoters]
	ids = ids[nVoters:]

	if nLearners > 0 {
		cs.Learners = ids[:nLearners]
		ids = ids[nLearners:]
	}

	// Roll the dice on how many of the incoming voters we decide were also
	// previously voters.
	//
	// NB: this code avoids creating non-nil empty slices (here and below).
	nOutgoingRetainedVoters := rand.Intn(nVoters + 1)
	if nOutgoingRetainedVoters > 0 || nRemovedVoters > 0 {
		cs.VotersOutgoing = append([]uint64(nil), cs.Voters[:nOutgoingRetainedVoters]...)
		cs.VotersOutgoing = append(cs.VotersOutgoing, ids[:nRemovedVoters]...)
	}
	// Only outgoing voters that are not also incoming voters can be in
	// LearnersNext (they represent demotions).
	if nRemovedVoters > 0 {
		if nLearnersNext := rand.Intn(nRemovedVoters + 1); nLearnersNext > 0 {
			cs.LearnersNext = ids[:nLearnersNext]
		}
	}

	cs.AutoLeave = len(cs.VotersOutgoing) > 0 && rand.Intn(2) == 1
	return reflect.ValueOf(rndConfChange(cs))
}

func TestRestore(t *testing.T) {
	cfg := quick.Config{MaxCount: 1000}

	f := func(cs pb.ConfState) bool {
		chg := Changer{
			Tracker:   tracker.MakeProgressTracker(20),
			LastIndex: 10,
		}
		cfg, prs, err := Restore(chg, cs)
		if err != nil {
			t.Error(err)
			return false
		}
		chg.Tracker.Config = cfg
		chg.Tracker.Progress = prs

		for _, sl := range [][]uint64{
			cs.Voters,
			cs.Learners,
			cs.VotersOutgoing,
			cs.LearnersNext,
		} {
			sort.Slice(sl, func(i, j int) bool { return sl[i] < sl[j] })
		}

		cs2 := chg.Tracker.ConfState()
		// NB: cs.Equivalent does the same "sorting" dance internally, but let's
		// test it a bit here instead of relying on it.
		if reflect.DeepEqual(cs, cs2) && cs.Equivalent(cs2) == nil && cs2.Equivalent(cs) == nil {
			return true // success
		}
		t.Errorf(`
before: %+#v
after:  %+#v`, cs, cs2)
		return false
	}

	ids := func(sl ...uint64) []uint64 {
		return sl
	}

	// Unit tests.
	for _, cs := range []pb.ConfState{
		{},
		{Voters: ids(1, 2, 3)},
		{Voters: ids(1, 2, 3), Learners: ids(4, 5, 6)},
		{Voters: ids(1, 2, 3), Learners: ids(5), VotersOutgoing: ids(1, 2, 4, 6), LearnersNext: ids(4)},
	} {
		if !f(cs) {
			t.FailNow() // f() already logged a nice t.Error()
		}
	}

	if err := quick.Check(func(cs rndConfChange) bool {
		return f(pb.ConfState(cs))
	}, &cfg); err != nil {
		t.Error(err)
	}
}
