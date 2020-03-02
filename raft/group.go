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
	"go.etcd.io/etcd/raft/tracker"
)

type Group struct {
	ID      uint64
	Members []uint64
}

func (g *Group) IsValid() bool {
	return g.ID != None && len(g.Members) > 0
}

type MemberInfo struct {
	Delegate uint64
	Group    uint64
}

type Groups struct {
	// LeaderGroupID is used for ensuring the peers in the group will not be delegated
	LeaderGroupID uint64
	// BcastTargets contains the pair: group delegate => other members in the same group
	// The peers in the map should be mutually exclusive with `unresolvedPeers`
	BcastTargets map[uint64][]uint64
	// Indexes is a map containing node id => group info pairs
	Indexes map[uint64]*MemberInfo
	// All the peers without delegates
	unresolvedPeers []uint64
}

// NewGroups creates a new `Groups` from a collection of `Group`s
func NewGroups(configs []Group) Groups {
	groups := Groups{
		Indexes:         make(map[uint64]*MemberInfo),
		BcastTargets:    make(map[uint64][]uint64),
		unresolvedPeers: make([]uint64, 0),
		LeaderGroupID:   None,
	}
	for _, c := range configs {
		if c.IsValid() {
			for _, m := range c.Members {
				groups.Indexes[m] = &MemberInfo{
					Delegate: None,
					Group:    c.ID,
				}
				groups.unresolvedPeers = append(groups.unresolvedPeers, m)
			}
		}
	}
	return groups
}

func (g *Groups) GetMemberInfo(member uint64) *MemberInfo {
	return g.Indexes[member]
}

// GetDelegate returns the delegate of given `peer`. The delegate could be the `peer` itself
func (g *Groups) GetDelegate(peer uint64) uint64 {
	if info, ok := g.Indexes[peer]; ok {
		return info.Delegate
	}
	return None
}

// pickDelegate picks a delegate who has the biggest `match` among the same group of given `to`
func (g *Groups) pickDelegate(to uint64, prs tracker.ProgressMap) {
	groupID := None
	if info, ok := g.Indexes[to]; ok {
		if info.Delegate != None {
			// The delegate have already been picked
			return
		}
		if info.Group == g.LeaderGroupID {
			// Never pick delegate for the member in the leader's group
			return
		}
		groupID = info.Group
	} else {
		// The peer is not included in Groups system, just ignore it
		return
	}

	chosen, match, bcastTargets := None, None, make([]uint64, 0)
	for _, member := range g.candidateDelegates(groupID) {
		if pr := prs[member]; pr != nil {
			if pr.Match > match {
				if chosen != None {
					// The old `chosen` must be delegated
					bcastTargets = append(bcastTargets, member)
				}
				chosen = member
				match = pr.Match
			} else {
				bcastTargets = append(bcastTargets, member)
			}
		}
	}
	// If there is only one member in the group, it remains `unresolvedPeers`
	if chosen != None && len(bcastTargets) > 0 {
		g.Indexes[chosen].Delegate = chosen
		for _, peer := range bcastTargets {
			g.Indexes[peer].Delegate = chosen
		}
		g.BcastTargets[chosen] = bcastTargets
	}
}

// IsDelegated returns whether the given peer has a delegate.
// If `peer` is a delegate, returns false
func (g *Groups) IsDelegated(peer uint64) bool {
	if info, ok := g.Indexes[peer]; ok {
		return info.Delegate != None && info.Delegate != peer
	}
	return false
}

// UpdateGroup updates given `peer`'s group ID and returns true if any peers are unresolved
func (g *Groups) UpdateGroup(peer uint64, group uint64) bool {
	if group == None {
		g.unmarkPeer(peer)
	} else if info, ok := g.Indexes[peer]; ok {
		if info.Group == group {
			// No need to update the same group
			return false
		}
		// Not a same group, update the peer
		g.unmarkPeer(peer)
		g.markPeer(peer, group)
	} else {
		g.markPeer(peer, group)
	}
	return len(g.unresolvedPeers) > 0
}

// unmarkPeer just removes the peer from Groups system.
// If `peer` is a delegate, this only removes the delegate itself
func (g *Groups) unmarkPeer(peer uint64) {
	if info, ok := g.Indexes[peer]; ok {
		if info.Delegate == peer {
			g.RemoveDelegate(peer)
			return
		}
		// The peer is not a delegate, exclude it from the delegated members
		targets := g.BcastTargets[info.Delegate]
		for i, p := range targets {
			if p == peer {
				g.BcastTargets[info.Delegate] = append(targets[:i], targets[i+1:]...)
			}
		}
	}
	delete(g.Indexes, peer)
}

func (g *Groups) markPeer(peer uint64, group uint64) {
	found, delegate := false, None
	for _, info := range g.Indexes {
		if info.Group == group {
			found = true
			// The delegate could still be `None` here
			delegate = info.Delegate
			break
		}
	}
	if _, ok := g.Indexes[peer]; ok {
		panic("Peer should never exist in indexes before marking")
	}
	g.Indexes[peer] = &MemberInfo{
		Delegate: delegate,
		Group:    group,
	}
	if delegate != None {
		g.BcastTargets[delegate] = append(g.BcastTargets[delegate], peer)
	} else if found {
		// No delegate in given `group` has been picked, add `peer` into unresolved peers
		g.unresolvedPeers = append(g.unresolvedPeers, peer)
	}
}

// ResolveDelegates picks delegates for all unresolved peers
func (g *Groups) ResolveDelegates(prs tracker.ProgressMap) {
	for _, peer := range g.unresolvedPeers {
		g.pickDelegate(peer, prs)
	}
	g.unresolvedPeers = g.unresolvedPeers[:0]
}

// RemoveDelegate removes the given `delegate` peer from Groups system.
// This should be called only when the delegate peer is detected to be unreachable
func (g *Groups) RemoveDelegate(delegate uint64) {
	if _, ok := g.BcastTargets[delegate]; ok {
		// Since `delegate` is unreachable, remove it from `Indexes`
		delete(g.Indexes, delegate)
		for peer, info := range g.Indexes {
			if info.Delegate == delegate {
				info.Delegate = None
				g.unresolvedPeers = append(g.unresolvedPeers, peer)
			}
		}
		delete(g.BcastTargets, delegate)
	}
}

// candidateDelegates returns all the members in given group
func (g *Groups) candidateDelegates(groupID uint64) []uint64 {
	res := make([]uint64, 0)
	for p, info := range g.Indexes {
		if groupID == info.Group {
			res = append(res, p)
		}
	}
	return res
}
