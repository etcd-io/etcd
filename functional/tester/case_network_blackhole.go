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

import "github.com/coreos/etcd/functional/rpcpb"

func inject_BLACKHOLE_PEER_PORT_TX_RX(clus *Cluster, idx int) error {
	return clus.sendOp(idx, rpcpb.Operation_BLACKHOLE_PEER_PORT_TX_RX)
}

func recover_BLACKHOLE_PEER_PORT_TX_RX(clus *Cluster, idx int) error {
	return clus.sendOp(idx, rpcpb.Operation_UNBLACKHOLE_PEER_PORT_TX_RX)
}

func new_Case_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER(clus *Cluster) Case {
	cc := caseByFunc{
		rpcpbCase:     rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	c := &caseFollower{cc, -1, -1}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}

func new_Case_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT() Case {
	cc := caseByFunc{
		rpcpbCase:     rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	c := &caseFollower{cc, -1, -1}
	return &caseUntilSnapshot{
		rpcpbCase: rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		Case:      c,
	}
}

func new_Case_BLACKHOLE_PEER_PORT_TX_RX_LEADER(clus *Cluster) Case {
	cc := caseByFunc{
		rpcpbCase:     rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_LEADER,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	c := &caseLeader{cc, -1, -1}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}

func new_Case_BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT() Case {
	cc := caseByFunc{
		rpcpbCase:     rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	c := &caseLeader{cc, -1, -1}
	return &caseUntilSnapshot{
		rpcpbCase: rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		Case:      c,
	}
}

func new_Case_BLACKHOLE_PEER_PORT_TX_RX_QUORUM(clus *Cluster) Case {
	c := &caseQuorum{
		caseByFunc: caseByFunc{
			rpcpbCase:     rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_QUORUM,
			injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
			recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
		},
		injected: make(map[int]struct{}),
	}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}

func new_Case_BLACKHOLE_PEER_PORT_TX_RX_ALL(clus *Cluster) Case {
	c := &caseAll{
		rpcpbCase:     rpcpb.Case_BLACKHOLE_PEER_PORT_TX_RX_ALL,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}
