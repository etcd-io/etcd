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

	"github.com/coreos/etcd/functional/rpcpb"

	"go.uber.org/zap"
)

const (
	// Wait more when it recovers from slow network, because network layer
	// needs extra time to propagate traffic control (tc command) change.
	// Otherwise, we get different hash values from the previous revision.
	// For more detail, please see https://github.com/coreos/etcd/issues/5121.
	waitRecover = 5 * time.Second
)

func inject_DELAY_PEER_PORT_TX_RX(clus *Cluster, idx int) error {
	clus.lg.Info(
		"injecting delay latency",
		zap.Duration("latency", time.Duration(clus.Tester.UpdatedDelayLatencyMs)*time.Millisecond),
		zap.Duration("latency-rv", time.Duration(clus.Tester.DelayLatencyMsRv)*time.Millisecond),
		zap.String("endpoint", clus.Members[idx].EtcdClientEndpoint),
	)
	return clus.sendOp(idx, rpcpb.Operation_DELAY_PEER_PORT_TX_RX)
}

func recover_DELAY_PEER_PORT_TX_RX(clus *Cluster, idx int) error {
	err := clus.sendOp(idx, rpcpb.Operation_UNDELAY_PEER_PORT_TX_RX)
	time.Sleep(waitRecover)
	return err
}

func new_FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER(clus *Cluster, random bool) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER,
		injectMember:  inject_DELAY_PEER_PORT_TX_RX,
		recoverMember: recover_DELAY_PEER_PORT_TX_RX,
	}
	clus.Tester.UpdatedDelayLatencyMs = clus.Tester.DelayLatencyMs
	if random {
		clus.UpdateDelayLatencyMs()
		ff.failureCase = rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER
	}
	f := &failureFollower{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT(clus *Cluster, random bool) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  inject_DELAY_PEER_PORT_TX_RX,
		recoverMember: recover_DELAY_PEER_PORT_TX_RX,
	}
	clus.Tester.UpdatedDelayLatencyMs = clus.Tester.DelayLatencyMs
	if random {
		clus.UpdateDelayLatencyMs()
		ff.failureCase = rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT
	}
	f := &failureFollower{ff, -1, -1}
	return &failureUntilSnapshot{
		failureCase: ff.failureCase,
		Failure:     f,
	}
}

func new_FailureCase_DELAY_PEER_PORT_TX_RX_LEADER(clus *Cluster, random bool) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_LEADER,
		injectMember:  inject_DELAY_PEER_PORT_TX_RX,
		recoverMember: recover_DELAY_PEER_PORT_TX_RX,
	}
	clus.Tester.UpdatedDelayLatencyMs = clus.Tester.DelayLatencyMs
	if random {
		clus.UpdateDelayLatencyMs()
		ff.failureCase = rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_LEADER
	}
	f := &failureLeader{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT(clus *Cluster, random bool) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		injectMember:  inject_DELAY_PEER_PORT_TX_RX,
		recoverMember: recover_DELAY_PEER_PORT_TX_RX,
	}
	clus.Tester.UpdatedDelayLatencyMs = clus.Tester.DelayLatencyMs
	if random {
		clus.UpdateDelayLatencyMs()
		ff.failureCase = rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT
	}
	f := &failureLeader{ff, -1, -1}
	return &failureUntilSnapshot{
		failureCase: ff.failureCase,
		Failure:     f,
	}
}

func new_FailureCase_DELAY_PEER_PORT_TX_RX_QUORUM(clus *Cluster, random bool) Failure {
	f := &failureQuorum{
		failureByFunc: failureByFunc{
			failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_QUORUM,
			injectMember:  inject_DELAY_PEER_PORT_TX_RX,
			recoverMember: recover_DELAY_PEER_PORT_TX_RX,
		},
		injected: make(map[int]struct{}),
	}
	clus.Tester.UpdatedDelayLatencyMs = clus.Tester.DelayLatencyMs
	if random {
		clus.UpdateDelayLatencyMs()
		f.failureCase = rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_QUORUM
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_DELAY_PEER_PORT_TX_RX_ALL(clus *Cluster, random bool) Failure {
	f := &failureAll{
		failureCase:   rpcpb.FailureCase_DELAY_PEER_PORT_TX_RX_ALL,
		injectMember:  inject_DELAY_PEER_PORT_TX_RX,
		recoverMember: recover_DELAY_PEER_PORT_TX_RX,
	}
	clus.Tester.UpdatedDelayLatencyMs = clus.Tester.DelayLatencyMs
	if random {
		clus.UpdateDelayLatencyMs()
		f.failureCase = rpcpb.FailureCase_RANDOM_DELAY_PEER_PORT_TX_RX_ALL
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}
