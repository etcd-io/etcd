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

package tracker

import (
	"fmt"
	"sort"

	"go.etcd.io/etcd/raft/quorum"
)

// ProgressTracker tracks the currently active configuration and the information
// known about the nodes and learners in it. In particular, it tracks the match
// index for each peer which in turn allows reasoning about the committed index.
type ProgressTracker struct {
	Voters   quorum.JointConfig
	Learners map[uint64]struct{}

	Progress map[uint64]*Progress

	Votes map[uint64]bool

	MaxInflight int
}

// MakeProgressTracker initializes a ProgressTracker.
func MakeProgressTracker(maxInflight int) ProgressTracker {
	p := ProgressTracker{
		MaxInflight: maxInflight,
		Voters: quorum.JointConfig{
			quorum.MajorityConfig{},
			quorum.MajorityConfig{},
		},
		Learners: map[uint64]struct{}{},
		Votes:    map[uint64]bool{},
		Progress: map[uint64]*Progress{},
	}
	return p
}

// IsSingleton returns true if (and only if) there is only one voting member
// (i.e. the leader) in the current configuration.
func (p *ProgressTracker) IsSingleton() bool {
	return len(p.Voters[0]) == 1 && len(p.Voters[1]) == 0
}

type matchAckIndexer map[uint64]*Progress

var _ quorum.AckedIndexer = matchAckIndexer(nil)

// AckedIndex implements IndexLookuper.
func (l matchAckIndexer) AckedIndex(id uint64) (quorum.Index, bool) {
	pr, ok := l[id]
	if !ok {
		return 0, false
	}
	return quorum.Index(pr.Match), true
}

// Committed returns the largest log index known to be committed based on what
// the voting members of the group have acknowledged.
func (p *ProgressTracker) Committed() uint64 {
	return uint64(p.Voters.CommittedIndex(matchAckIndexer(p.Progress)))
}

// RemoveAny removes this peer, which *must* be tracked as a voter or learner,
// from the tracker.
func (p *ProgressTracker) RemoveAny(id uint64) {
	_, okPR := p.Progress[id]
	_, okV1 := p.Voters[0][id]
	_, okV2 := p.Voters[1][id]
	_, okL := p.Learners[id]

	okV := okV1 || okV2

	if !okPR {
		panic("attempting to remove unknown peer %x")
	} else if !okV && !okL {
		panic("attempting to remove unknown peer %x")
	} else if okV && okL {
		panic(fmt.Sprintf("peer %x is both voter and learner", id))
	}

	delete(p.Voters[0], id)
	delete(p.Voters[1], id)
	delete(p.Learners, id)
	delete(p.Progress, id)
}

// InitProgress initializes a new progress for the given node or learner. The
// node may not exist yet in either form or a panic will ensue.
func (p *ProgressTracker) InitProgress(id, match, next uint64, isLearner bool) {
	if pr := p.Progress[id]; pr != nil {
		panic(fmt.Sprintf("peer %x already tracked as node %v", id, pr))
	}
	if !isLearner {
		p.Voters[0][id] = struct{}{}
	} else {
		p.Learners[id] = struct{}{}
	}
	p.Progress[id] = &Progress{Next: next, Match: match, Inflights: NewInflights(p.MaxInflight), IsLearner: isLearner}
}

// Visit invokes the supplied closure for all tracked progresses.
func (p *ProgressTracker) Visit(f func(id uint64, pr *Progress)) {
	for id, pr := range p.Progress {
		f(id, pr)
	}
}

// QuorumActive returns true if the quorum is active from the view of the local
// raft state machine. Otherwise, it returns false.
func (p *ProgressTracker) QuorumActive() bool {
	votes := map[uint64]bool{}
	p.Visit(func(id uint64, pr *Progress) {
		if pr.IsLearner {
			return
		}
		votes[id] = pr.RecentActive
	})

	return p.Voters.VoteResult(votes) == quorum.VoteWon
}

// VoterNodes returns a sorted slice of voters.
func (p *ProgressTracker) VoterNodes() []uint64 {
	m := p.Voters.IDs()
	nodes := make([]uint64, 0, len(m))
	for id := range m {
		nodes = append(nodes, id)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	return nodes
}

// LearnerNodes returns a sorted slice of learners.
func (p *ProgressTracker) LearnerNodes() []uint64 {
	nodes := make([]uint64, 0, len(p.Learners))
	for id := range p.Learners {
		nodes = append(nodes, id)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	return nodes
}

// ResetVotes prepares for a new round of vote counting via recordVote.
func (p *ProgressTracker) ResetVotes() {
	p.Votes = map[uint64]bool{}
}

// RecordVote records that the node with the given id voted for this Raft
// instance if v == true (and declined it otherwise).
func (p *ProgressTracker) RecordVote(id uint64, v bool) {
	_, ok := p.Votes[id]
	if !ok {
		p.Votes[id] = v
	}
}

// TallyVotes returns the number of granted and rejected Votes, and whether the
// election outcome is known.
func (p *ProgressTracker) TallyVotes() (granted int, rejected int, _ quorum.VoteResult) {
	// Make sure to populate granted/rejected correctly even if the Votes slice
	// contains members no longer part of the configuration. This doesn't really
	// matter in the way the numbers are used (they're informational), but might
	// as well get it right.
	for id, pr := range p.Progress {
		if pr.IsLearner {
			continue
		}
		v, voted := p.Votes[id]
		if !voted {
			continue
		}
		if v {
			granted++
		} else {
			rejected++
		}
	}
	result := p.Voters.VoteResult(p.Votes)
	return granted, rejected, result
}
