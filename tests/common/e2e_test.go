// Copyright 2022 The etcd Authors
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

//go:build e2e

package common

import (
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/framework"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func init() {
	testRunner = framework.E2eTestRunner
	clusterTestCases = e2eClusterTestCases
}

func e2eClusterTestCases() []testCase {
	tcs := []testCase{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}

	if fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		tcs = append(tcs, testCase{
			name: "MinorityLastVersion",
			config: config.ClusterConfig{
				ClusterSize: 3,
				ClusterContext: &e2e.ClusterContext{
					Version: e2e.MinorityLastVersion,
				},
			},
		}, testCase{
			name: "QuorumLastVersion",
			config: config.ClusterConfig{
				ClusterSize: 3,
				ClusterContext: &e2e.ClusterContext{
					Version: e2e.QuorumLastVersion,
				},
			},
		})
	}
	return tcs
}

func WithAuth(userName, password string) config.ClientOption {
	return e2e.WithAuth(userName, password)
}

func WithEndpoints(endpoints []string) config.ClientOption {
	return e2e.WithEndpoints(endpoints)
}
