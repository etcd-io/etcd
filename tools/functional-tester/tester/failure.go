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

// Failure defines failure injection interface.
// To add a fail case:
//  1. implement "Failure" interface
//  2. define fail case name in "rpcpb.FailureCase"
type Failure interface {
	// Inject injeccts the failure into the testing cluster at the given
	// round. When calling the function, the cluster should be in health.
	Inject(clus *Cluster) error
	// Recover recovers the injected failure caused by the injection of the
	// given round and wait for the recovery of the testing cluster.
	Recover(clus *Cluster) error
	// Desc returns a description of the failure
	Desc() string
}
