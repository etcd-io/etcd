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

func injectKill(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_KillEtcd)
}

func recoverKill(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_RestartEtcd)
}

func newFailureKillOneFollower(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_KILL_ONE_FOLLOWER,
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
	f := &failureFollower{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func newFailureKillLeader(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_KILL_LEADER,
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
	f := &failureLeader{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func newFailureKillQuorum(clus *Cluster) Failure {
	f := &failureQuorum{
		failureCase:   rpcpb.FailureCase_KILL_QUORUM,
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func newFailureKillAll(clus *Cluster) Failure {
	f := &failureAll{
		failureCase:   rpcpb.FailureCase_KILL_ALL,
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func newFailureKillOneFollowerUntilTriggerSnapshot(clus *Cluster) Failure {
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_KILL_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     newFailureKillOneFollower(clus),
	}
}

func newFailureKillLeaderUntilTriggerSnapshot(clus *Cluster) Failure {
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_KILL_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     newFailureKillLeader(clus),
	}
}
