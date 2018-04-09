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

func inject_SIGTERM_ETCD(clus *Cluster, idx int) error {
	return clus.sendOp(idx, rpcpb.Operation_SIGTERM_ETCD)
}

func recover_SIGTERM_ETCD(clus *Cluster, idx int) error {
	return clus.sendOp(idx, rpcpb.Operation_RESTART_ETCD)
}

func new_FailureCase_SIGTERM_ONE_FOLLOWER(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_SIGTERM_ONE_FOLLOWER,
		injectMember:  inject_SIGTERM_ETCD,
		recoverMember: recover_SIGTERM_ETCD,
	}
	f := &failureFollower{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_SIGTERM_LEADER(clus *Cluster) Failure {
	ff := failureByFunc{
		failureCase:   rpcpb.FailureCase_SIGTERM_LEADER,
		injectMember:  inject_SIGTERM_ETCD,
		recoverMember: recover_SIGTERM_ETCD,
	}
	f := &failureLeader{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_SIGTERM_QUORUM(clus *Cluster) Failure {
	f := &failureQuorum{
		failureCase:   rpcpb.FailureCase_SIGTERM_QUORUM,
		injectMember:  inject_SIGTERM_ETCD,
		recoverMember: recover_SIGTERM_ETCD,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_SIGTERM_ALL(clus *Cluster) Failure {
	f := &failureAll{
		failureCase:   rpcpb.FailureCase_SIGTERM_ALL,
		injectMember:  inject_SIGTERM_ETCD,
		recoverMember: recover_SIGTERM_ETCD,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

func new_FailureCase_SIGTERM_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT(clus *Cluster) Failure {
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_SIGTERM_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     new_FailureCase_SIGTERM_ONE_FOLLOWER(clus),
	}
}

func new_FailureCase_SIGTERM_LEADER_UNTIL_TRIGGER_SNAPSHOT(clus *Cluster) Failure {
	return &failureUntilSnapshot{
		failureCase: rpcpb.FailureCase_SIGTERM_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		Failure:     new_FailureCase_SIGTERM_LEADER(clus),
	}
}
