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

package common

import (
	"testing"

	"go.etcd.io/etcd/tests/v3/framework"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

var testRunner framework.TestRunner

var clusterTestCases = []testCase{
	{
		name:   "NoTLS",
		config: config.NewClusterConfig(config.WithClusterSize(1)),
	},
	{
		name:   "PeerTLS",
		config: config.NewClusterConfig(config.WithPeerTLS(config.ManualTLS)),
	},
	{
		name:   "PeerAutoTLS",
		config: config.NewClusterConfig(config.WithPeerTLS(config.AutoTLS)),
	},
	{
		name:   "ClientTLS",
		config: config.NewClusterConfig(config.WithClusterSize(1), config.WithClientTLS(config.ManualTLS)),
	},
	{
		name:   "ClientAutoTLS",
		config: config.NewClusterConfig(config.WithClusterSize(1), config.WithClientTLS(config.AutoTLS)),
	},
}

func TestMain(m *testing.M) {
	testRunner.TestMain(m)
}

type testCase struct {
	name   string
	config config.ClusterConfig
}
