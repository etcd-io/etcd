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
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/failpoint"
	"go.etcd.io/etcd/tests/v3/robustness/options"
	"go.etcd.io/etcd/tests/v3/robustness/random"
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

type testScenario struct {
	name      string
	failpoint failpoint.Failpoint
	cluster   e2e.EtcdProcessClusterConfig
	traffic   traffic.Traffic
	profile   traffic.Profile
	watch     client.WatchConfig
}

func exploratoryScenarios(_ *testing.T) []testScenario {
	randomizableOptions := []e2e.EPClusterOption{
		options.WithClusterOptionGroups(
			options.ClusterOptions{options.WithTickMs(29), options.WithElectionMs(271)},
			options.ClusterOptions{options.WithTickMs(101), options.WithElectionMs(521)},
			options.ClusterOptions{options.WithTickMs(100), options.WithElectionMs(2000)}),
	}

	mixedVersionOptionChoices := []random.ChoiceWeight[options.ClusterOptions]{
		// 60% with all members of current version
		{Choice: options.ClusterOptions{options.WithVersion(e2e.CurrentVersion)}, Weight: 60},
		// 10% with 2 members of current version, 1 member last version, leader is current version
		{Choice: options.ClusterOptions{options.WithVersion(e2e.MinorityLastVersion), options.WithInitialLeaderIndex(0)}, Weight: 10},
		// 10% with 2 members of current version, 1 member last version, leader is last version
		{Choice: options.ClusterOptions{options.WithVersion(e2e.MinorityLastVersion), options.WithInitialLeaderIndex(2)}, Weight: 10},
		// 10% with 2 members of last version, 1 member current version, leader is last version
		{Choice: options.ClusterOptions{options.WithVersion(e2e.QuorumLastVersion), options.WithInitialLeaderIndex(0)}, Weight: 10},
		// 10% with 2 members of last version, 1 member current version, leader is current version
		{Choice: options.ClusterOptions{options.WithVersion(e2e.QuorumLastVersion), options.WithInitialLeaderIndex(2)}, Weight: 10},
	}
	mixedVersionOption := options.WithClusterOptionGroups(random.PickRandom[options.ClusterOptions](mixedVersionOptionChoices))

	baseOptions := []e2e.EPClusterOption{
		options.WithSnapshotCount(50, 100, 1000),
		options.WithSubsetOptions(randomizableOptions...),
		e2e.WithGoFailEnabled(true),
		// Set low minimal compaction batch limit to allow for triggering multi batch compaction failpoints.
		options.WithCompactionBatchLimit(10, 100, 1000),
		e2e.WithWatchProcessNotifyInterval(100 * time.Millisecond),
	}

	if e2e.CouldSetSnapshotCatchupEntries(e2e.BinPath.Etcd) {
		baseOptions = append(baseOptions, e2e.WithSnapshotCatchUpEntries(100))
	}
	scenarios := []testScenario{}
	for _, tp := range trafficProfiles {
		name := filepath.Join(tp.Name, "ClusterOfSize1")
		clusterOfSize1Options := baseOptions
		clusterOfSize1Options = append(clusterOfSize1Options, e2e.WithClusterSize(1))
		scenarios = append(scenarios, testScenario{
			name:    name,
			traffic: tp.Traffic,
			profile: tp.Profile,
			cluster: *e2e.NewConfig(clusterOfSize1Options...),
		})
	}

	for _, tp := range trafficProfiles {
		name := filepath.Join(tp.Name, "ClusterOfSize3")
		clusterOfSize3Options := baseOptions
		clusterOfSize3Options = append(clusterOfSize3Options, e2e.WithIsPeerTLS(true))
		clusterOfSize3Options = append(clusterOfSize3Options, e2e.WithPeerProxy(true))
		if fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
			clusterOfSize3Options = append(clusterOfSize3Options, mixedVersionOption)
		}
		scenarios = append(scenarios, testScenario{
			name:    name,
			traffic: tp.Traffic,
			profile: tp.Profile,
			cluster: *e2e.NewConfig(clusterOfSize3Options...),
		})
	}
	if e2e.BinPath.LazyFSAvailable() {
		newScenarios := scenarios
		for _, s := range scenarios {
			// LazyFS increases the load on CPU, so we run it with more lightweight case.
			if s.profile.MinimalQPS <= 100 && s.cluster.ClusterSize == 1 {
				lazyfsCluster := s.cluster
				lazyfsCluster.LazyFSEnabled = true
				newScenarios = append(newScenarios, testScenario{
					name:      filepath.Join(s.name, "LazyFS"),
					failpoint: s.failpoint,
					cluster:   lazyfsCluster,
					traffic:   s.traffic,
					profile:   s.profile.WithoutCompaction(),
					watch:     s.watch,
				})
			}
		}
		scenarios = newScenarios
	}
	return scenarios
}

func regressionScenarios(t *testing.T) []testScenario {
	v, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		t.Fatalf("Failed checking etcd version binary, binary: %q, err: %v", e2e.BinPath.Etcd, err)
	}

	scenarios := []testScenario{}
	scenarios = append(scenarios, testScenario{
		name:      "Issue14370",
		failpoint: failpoint.RaftBeforeSavePanic,
		profile:   traffic.LowTraffic,
		traffic:   traffic.EtcdPutDeleteLease,
		cluster: *e2e.NewConfig(
			e2e.WithClusterSize(1),
			e2e.WithGoFailEnabled(true),
		),
	})
	scenarios = append(scenarios, testScenario{
		name:      "Issue14685",
		failpoint: failpoint.DefragBeforeCopyPanic,
		profile:   traffic.LowTraffic,
		traffic:   traffic.EtcdPutDeleteLease,
		cluster: *e2e.NewConfig(
			e2e.WithClusterSize(1),
			e2e.WithGoFailEnabled(true),
		),
	})
	scenarios = append(scenarios, testScenario{
		name:      "Issue13766",
		failpoint: failpoint.KillFailpoint,
		profile:   traffic.HighTrafficProfile,
		traffic:   traffic.EtcdPut,
		cluster: *e2e.NewConfig(
			e2e.WithSnapshotCount(100),
		),
	})
	scenarios = append(scenarios, testScenario{
		name: "Issue15220",
		watch: client.WatchConfig{
			RequestProgress: true,
		},
		profile:   traffic.LowTraffic,
		traffic:   traffic.EtcdPutDeleteLease,
		failpoint: failpoint.KillFailpoint,
		cluster: *e2e.NewConfig(
			e2e.WithClusterSize(1),
		),
	})
	scenarios = append(scenarios, testScenario{
		name:      "Issue17529",
		profile:   traffic.HighTrafficProfile,
		traffic:   traffic.Kubernetes,
		failpoint: failpoint.SleepBeforeSendWatchResponse,
		cluster: *e2e.NewConfig(
			e2e.WithClusterSize(1),
			e2e.WithGoFailEnabled(true),
			options.WithSnapshotCount(100),
		),
	})
	if v.Compare(version.V3_5) >= 0 {
		opts := []e2e.EPClusterOption{
			e2e.WithSnapshotCount(100),
			e2e.WithPeerProxy(true),
			e2e.WithIsPeerTLS(true),
		}
		if e2e.CouldSetSnapshotCatchupEntries(e2e.BinPath.Etcd) {
			opts = append(opts, e2e.WithSnapshotCatchUpEntries(100))
		}
		scenarios = append(scenarios, testScenario{
			name:      "Issue15271",
			failpoint: failpoint.BlackholeUntilSnapshot,
			profile:   traffic.HighTrafficProfile,
			traffic:   traffic.EtcdPut,
			cluster:   *e2e.NewConfig(opts...),
		})
	}
	return scenarios
}
