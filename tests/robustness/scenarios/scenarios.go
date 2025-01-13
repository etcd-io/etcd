// Copyright 2023 The etcd Authors
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

package scenarios

import (
	"testing"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/failpoint"
	"go.etcd.io/etcd/tests/v3/robustness/options"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

type TrafficProfile struct {
	Name    string
	Traffic traffic.Traffic
	Profile traffic.Profile
}

var trafficProfiles = []TrafficProfile{
	{
		Name:    "EtcdHighTraffic",
		Traffic: traffic.EtcdPut,
		Profile: traffic.HighTrafficProfile,
	},
	{
		Name:    "EtcdTrafficDeleteLeases",
		Traffic: traffic.EtcdPutDeleteLease,
		Profile: traffic.LowTraffic,
	},
	{
		Name:    "KubernetesHighTraffic",
		Traffic: traffic.Kubernetes,
		Profile: traffic.HighTrafficProfile,
	},
	{
		Name:    "KubernetesLowTraffic",
		Traffic: traffic.Kubernetes,
		Profile: traffic.LowTraffic,
	},
}

type TestScenario struct {
	Name      string
	Failpoint failpoint.Failpoint
	Cluster   e2e.EtcdProcessClusterConfig
	Traffic   traffic.Traffic
	Profile   traffic.Profile
	Watch     client.WatchConfig
}

func Exploratory(_ *testing.T) (scenarios []TestScenario) {
	return scenarios
}

func Regression(t *testing.T) (scenarios []TestScenario) {
	for i := 0; i < 20; i++ {
		scenarios = append(scenarios, TestScenario{
			Name:      "Issue17529",
			Profile:   traffic.HighTrafficProfile,
			Traffic:   traffic.Kubernetes,
			Failpoint: failpoint.SleepBeforeSendWatchResponse,
			Cluster: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithGoFailEnabled(true),
				options.WithSnapshotCount(100),
			),
		})
	}
	return scenarios
}
