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

package robustness

import (
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/failpoint"
	"go.etcd.io/etcd/tests/v3/robustness/options"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

type TrafficProfile struct {
	Traffic traffic.Traffic
	Profile traffic.Profile
}

var trafficProfiles = []TrafficProfile{
	{
		Traffic: traffic.Kubernetes,
		Profile: traffic.LowTraffic,
	},
}

type testScenario struct {
	name      string
	failpoint failpoint.Failpoint
	cluster   e2e.EtcdProcessClusterConfig
	traffic   traffic.Traffic
	profile   traffic.Profile
	watch     watchConfig
}

func scenarios(t *testing.T) []testScenario {
	v, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		t.Fatalf("Failed checking etcd version binary, binary: %q, err: %v", e2e.BinPath.Etcd, err)
	}
	randomizableOptions := []e2e.EPClusterOption{
		options.WithClusterOptionGroups(
			options.ClusterOptions{options.WithTickMs(29), options.WithElectionMs(271)},
			options.ClusterOptions{options.WithTickMs(101), options.WithElectionMs(521)},
			options.ClusterOptions{options.WithTickMs(100), options.WithElectionMs(2000)}),
	}

	baseOptions := []e2e.EPClusterOption{
		options.WithSnapshotCount(50),
		options.WithSubsetOptions(randomizableOptions...),
		e2e.WithGoFailEnabled(true),
		e2e.WithCompactionBatchLimit(100),
		e2e.WithWatchProcessNotifyInterval(100 * time.Millisecond),
	}
	scenarios := []testScenario{}

	for _, tp := range trafficProfiles {
		name := filepath.Join(tp.Traffic.Name(), tp.Profile.Name, "ClusterOfSize3")
		clusterOfSize3Options := baseOptions
		clusterOfSize3Options = append(clusterOfSize3Options, e2e.WithIsPeerTLS(true))
		clusterOfSize3Options = append(clusterOfSize3Options, e2e.WithPeerProxy(true))
		if !v.LessThan(version.V3_6) {
			clusterOfSize3Options = append(clusterOfSize3Options, e2e.WithSnapshotCatchUpEntries(100))
		}
		scenarios = append(scenarios, testScenario{
			name:    name,
			traffic: tp.Traffic,
			profile: tp.Profile,
			cluster: *e2e.NewConfig(clusterOfSize3Options...),
		})
	}
	return scenarios
}
