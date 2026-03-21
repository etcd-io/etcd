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
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/etcdserver"
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
		Profile: traffic.Profile{
			KeyValue:   &traffic.KeyValueHigh,
			Watch:      &traffic.WatchDefault,
			Compaction: &traffic.CompactionDefault,
		},
	},
	{
		Name:    "EtcdTrafficDeleteLeases",
		Traffic: traffic.EtcdPutDeleteLease,
		Profile: traffic.Profile{
			KeyValue:   &traffic.KeyValueMedium,
			Watch:      &traffic.WatchDefault,
			Compaction: &traffic.CompactionDefault,
		},
	},
	{
		Name:    "KubernetesHighTraffic",
		Traffic: traffic.Kubernetes,
		Profile: traffic.Profile{
			KeyValue:   &traffic.KeyValueHigh,
			Watch:      &traffic.WatchDefault,
			Compaction: &traffic.CompactionDefault,
		},
	},
	{
		Name:    "KubernetesLowTraffic",
		Traffic: traffic.Kubernetes,
		Profile: traffic.Profile{
			KeyValue:   &traffic.KeyValueMedium,
			Watch:      &traffic.WatchDefault,
			Compaction: &traffic.CompactionDefault,
		},
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

func Exploratory(_ *testing.T) []TestScenario {
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
		// Set a low minimal compaction batch limit to allow for triggering multi batch compaction failpoints.
		options.WithCompactionBatchLimit(10, 100, 1000),
		e2e.WithWatchProcessNotifyInterval(100 * time.Millisecond),
	}

	if addr := os.Getenv("TRACING_SERVER_ADDR"); addr != "" {
		baseOptions = append(baseOptions, e2e.WithEnableDistributedTracing(addr))
	}

	if e2e.CouldSetSnapshotCatchupEntries(e2e.BinPath.Etcd) {
		baseOptions = append(baseOptions, options.WithSnapshotCatchUpEntries(100, etcdserver.DefaultSnapshotCatchUpEntries))
	}
	scenarios := []TestScenario{}
	for _, tp := range trafficProfiles {
		name := filepath.Join(tp.Name, "ClusterOfSize1")
		clusterOfSize1Options := baseOptions
		clusterOfSize1Options = append(clusterOfSize1Options, e2e.WithClusterSize(1))
		scenarios = append(scenarios, TestScenario{
			Name:    name,
			Traffic: tp.Traffic,
			Profile: tp.Profile,
			Cluster: *e2e.NewConfig(clusterOfSize1Options...),
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
		scenarios = append(scenarios, TestScenario{
			Name:    name,
			Traffic: tp.Traffic,
			Profile: tp.Profile,
			Cluster: *e2e.NewConfig(clusterOfSize3Options...),
		})
	}
	if e2e.BinPath.LazyFSAvailable() {
		newScenarios := scenarios
		for _, s := range scenarios {
			// LazyFS increases the load on the CPU, so we run it with a more lightweight case.
			if s.Profile.KeyValue.MinimalQPS <= 100 && s.Cluster.ClusterSize == 1 {
				lazyfsCluster := s.Cluster
				lazyfsCluster.LazyFSEnabled = true
				profileWithoutCompaction := s.Profile
				profileWithoutCompaction.Compaction = nil
				newScenarios = append(newScenarios, TestScenario{
					Name:      filepath.Join(s.Name, "LazyFS"),
					Failpoint: s.Failpoint,
					Cluster:   lazyfsCluster,
					Traffic:   s.Traffic,
					Profile:   profileWithoutCompaction,
					Watch:     s.Watch,
				})
			}
		}
		scenarios = newScenarios
	}
	return scenarios
}

func Regression(t *testing.T) []TestScenario {
	return nil
}
