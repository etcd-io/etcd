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

func new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	f := &failureFollower{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT() Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	f := &failureFollower{ff, -1, -1}
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     f,
	}
}

func new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_LEADER(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_LEADER,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	f := &failureLeader{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT() Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	f := &failureLeader{ff, -1, -1}
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     f,
	}
}

func new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_QUORUM(clus *Cluster) Failure {
	f := &failureQuorum{
		failureCase:   rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_QUORUM,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ALL(clus *Cluster) Failure {
	f := &failureAll{
		failureCase:   rpcpb.FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ALL,
		injectMember:  inject_BLACKHOLE_PEER_PORT_TX_RX,
		recoverMember: recover_BLACKHOLE_PEER_PORT_TX_RX,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}
