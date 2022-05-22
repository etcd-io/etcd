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

var testRunner = framework.UnitTestRunner
var clusterTestCases = []testCase{
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

func TestMain(m *testing.M) {
	testRunner.TestMain(m)
}

type testCase struct {
	name   string
	config config.ClusterConfig
}
