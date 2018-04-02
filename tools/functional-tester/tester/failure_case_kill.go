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

import "github.com/coreos/etcd/tools/functional-tester/rpcpb"

func injectKill(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_KillEtcd)
}

func recoverKill(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_RestartEtcd)
}

func newFailureKillOneFollower() Failure {
	ff := failureByFunc{
		description:   "kill one follower",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
	return &failureFollower{ff, -1, -1}
}

func newFailureKillLeader() Failure {
	ff := failureByFunc{
		description:   "kill leader",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
	return &failureLeader{ff, -1, -1}
}

func newFailureKillQuorum() Failure {
	return &failureQuorum{
		description:   "kill quorum",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
}

func newFailureKillAll() Failure {
	return &failureAll{
		description:   "kill all",
		injectMember:  injectKill,
		recoverMember: recoverKill,
	}
}

func newFailureKillOneFollowerForLongTime() Failure {
	return &failureUntilSnapshot{newFailureKillOneFollower()}
}

func newFailureKillLeaderForLongTime() Failure {
	return &failureUntilSnapshot{newFailureKillLeader()}
}
