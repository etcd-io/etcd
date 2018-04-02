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

func injectBlackholePeerPortTxRx(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_BlackholePeerPortTxRx)
}

func recoverBlackholePeerPortTxRx(clus *Cluster, idx int) error {
	return clus.sendOperation(idx, rpcpb.Operation_UnblackholePeerPortTxRx)
}

func newFailureBlackholePeerPortTxRxOneFollower() Failure {
	ff := failureByFunc{
		description:   "blackhole peer port on one follower",
		injectMember:  injectBlackholePeerPortTxRx,
		recoverMember: recoverBlackholePeerPortTxRx,
	}
	f := &failureFollower{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: triggerElectionDur,
	}
}

func newFailureBlackholePeerPortTxRxLeader() Failure {
	ff := failureByFunc{
		description:   "blackhole peer port on leader",
		injectMember:  injectBlackholePeerPortTxRx,
		recoverMember: recoverBlackholePeerPortTxRx,
	}
	f := &failureLeader{ff, -1, -1}
	return &failureDelay{
		Failure:       f,
		delayDuration: triggerElectionDur,
	}
}

func newFailureBlackholePeerPortTxRxAll() Failure {
	f := &failureAll{
		description:   "blackhole peer port on all",
		injectMember:  injectBlackholePeerPortTxRx,
		recoverMember: recoverBlackholePeerPortTxRx,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: triggerElectionDur,
	}
}
