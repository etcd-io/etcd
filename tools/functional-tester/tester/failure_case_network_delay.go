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
	"time"

	"github.com/coreos/etcd/tools/functional-tester/rpcpb"
)

const (
	// Wait more when it recovers from slow network, because network layer
	// needs extra time to propagate traffic control (tc command) change.
	// Otherwise, we get different hash values from the previous revision.
	// For more detail, please see https://github.com/coreos/etcd/issues/5121.
	waitRecover = 5 * time.Second
)

func injectDelayPeerPortTxRx(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_DelayPeerPortTxRx)
}

func recoverDelayPeerPortTxRx(clus *Cluster, idx int) error {
	err := clus.sendOperation(idx, rpcpb.Operation_UndelayPeerPortTxRx)
	time.Sleep(waitRecover)
	return err
}

func newFailureDelayPeerPortTxRxOneFollower(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER,
		injectMember:  injectDelayPeerPortTxRx,
		recoverMember: recoverDelayPeerPortTxRx,
	}
	f := &failureFollower{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func newFailureDelayPeerPortTxRxOneFollowerUntilTriggerSnapshot() Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  injectDelayPeerPortTxRx,
		recoverMember: recoverDelayPeerPortTxRx,
	}
	f := &failureFollower{ff, -1, -1}
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     f,
	}
}

func newFailureDelayPeerPortTxRxLeader(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_LEADER,
		injectMember:  injectDelayPeerPortTxRx,
		recoverMember: recoverDelayPeerPortTxRx,
	}
	f := &failureLeader{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func newFailureDelayPeerPortTxRxLeaderUntilTriggerSnapshot() Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  injectDelayPeerPortTxRx,
		recoverMember: recoverDelayPeerPortTxRx,
	}
	f := &failureLeader{ff, -1, -1}
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     f,
	}
}

func newFailureDelayPeerPortTxRxQuorum(clus *Cluster) Failure {
	f := &failureQuorum{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_QUORUM,
		injectMember:  injectDelayPeerPortTxRx,
		recoverMember: recoverDelayPeerPortTxRx,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func newFailureDelayPeerPortTxRxAll(clus *Cluster) Failure {
	f := &failureAll{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ALL,
		injectMember:  injectDelayPeerPortTxRx,
		recoverMember: recoverDelayPeerPortTxRx,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}
