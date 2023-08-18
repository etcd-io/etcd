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

package robustness

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/robustness/model"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"go.etcd.io/etcd/tests/v3/robustness/report"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
	"go.etcd.io/etcd/tests/v3/robustness/validate"
)

type TrafficProfile struct {
	Traffic traffic.Traffic
	Profile traffic.Profile
}

var trafficProfiles = []TrafficProfile{
	{
		Traffic: traffic.EtcdPut,
		Profile: traffic.HighTrafficProfile,
	},
	{
		Traffic: traffic.EtcdPutDeleteLease,
		Profile: traffic.LowTraffic,
	},
	{
		Traffic: traffic.Kubernetes,
		Profile: traffic.HighTrafficProfile,
	},
	{
		Traffic: traffic.Kubernetes,
		Profile: traffic.LowTraffic,
	},
}

func TestRobustness(t *testing.T) {
	testRunner.BeforeTest(t)
	v, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		t.Fatalf("Failed checking etcd version binary, binary: %q, err: %v", e2e.BinPath.Etcd, err)
	}
	enableLazyFS := e2e.BinPath.LazyFSAvailable()
	baseOptions := []e2e.EPClusterOption{
		e2e.WithSnapshotCount(100),
		e2e.WithGoFailEnabled(true),
		e2e.WithCompactionBatchLimit(100),
		e2e.WithWatchProcessNotifyInterval(100 * time.Millisecond),
	}
	scenarios := []testScenario{}
	for _, tp := range trafficProfiles {
		name := filepath.Join(tp.Traffic.Name(), tp.Profile.Name, "ClusterOfSize1")
		clusterOfSize1Options := baseOptions
		clusterOfSize1Options = append(clusterOfSize1Options, e2e.WithClusterSize(1))
		// Add LazyFS only for traffic with lower QPS as it uses a lot of CPU lowering minimal QPS.
		if enableLazyFS && tp.Profile.MinimalQPS <= 100 {
			clusterOfSize1Options = append(clusterOfSize1Options, e2e.WithLazyFSEnabled(true))
			name = filepath.Join(name, "LazyFS")
		}
		scenarios = append(scenarios, testScenario{
			name:    name,
			traffic: tp.Traffic,
			cluster: *e2e.NewConfig(clusterOfSize1Options...),
		})
	}

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
			cluster: *e2e.NewConfig(clusterOfSize3Options...),
		})
	}
	scenarios = append(scenarios, testScenario{
		name:      "Issue14370",
		failpoint: RaftBeforeSavePanic,
		cluster: *e2e.NewConfig(
			e2e.WithClusterSize(1),
			e2e.WithGoFailEnabled(true),
		),
	})
	scenarios = append(scenarios, testScenario{
		name:      "Issue14685",
		failpoint: DefragBeforeCopyPanic,
		cluster: *e2e.NewConfig(
			e2e.WithClusterSize(1),
			e2e.WithGoFailEnabled(true),
		),
	})
	scenarios = append(scenarios, testScenario{
		name:      "Issue13766",
		failpoint: KillFailpoint,
		profile:   traffic.HighTrafficProfile,
		traffic:   traffic.EtcdPut,
		cluster: *e2e.NewConfig(
			e2e.WithSnapshotCount(100),
		),
	})
	scenarios = append(scenarios, testScenario{
		name: "Issue15220",
		watch: watchConfig{
			requestProgress: true,
		},
		cluster: *e2e.NewConfig(
			e2e.WithClusterSize(1),
		),
	})
	// TODO: Deflake waiting for waiting until snapshot for etcd versions that don't support setting snapshot catchup entries.
	if v.Compare(version.V3_6) >= 0 {
		scenarios = append(scenarios, testScenario{
			name:      "Issue15271",
			failpoint: BlackholeUntilSnapshot,
			profile:   traffic.HighTrafficProfile,
			traffic:   traffic.EtcdPut,
			cluster: *e2e.NewConfig(
				e2e.WithSnapshotCatchUpEntries(100),
				e2e.WithSnapshotCount(100),
				e2e.WithPeerProxy(true),
				e2e.WithIsPeerTLS(true),
			),
		})
	}
	for _, scenario := range scenarios {
		if scenario.traffic == nil {
			scenario.traffic = traffic.EtcdPutDeleteLease
		}
		if scenario.profile == (traffic.Profile{}) {
			scenario.profile = traffic.LowTraffic
		}

		t.Run(scenario.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			scenario.cluster.Logger = lg
			ctx := context.Background()
			testRobustness(ctx, t, lg, scenario)
		})
	}
}

type testScenario struct {
	name      string
	failpoint Failpoint
	cluster   e2e.EtcdProcessClusterConfig
	traffic   traffic.Traffic
	profile   traffic.Profile
	watch     watchConfig
}

func testRobustness(ctx context.Context, t *testing.T, lg *zap.Logger, s testScenario) {
	report := report.TestReport{Logger: lg}
	var err error
	report.Cluster, err = e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&s.cluster))
	if err != nil {
		t.Fatal(err)
	}
	defer report.Cluster.Close()

	if s.failpoint == nil {
		s.failpoint = pickRandomFailpoint(t, report.Cluster)
	} else {
		err = validateFailpoint(report.Cluster, s.failpoint)
		if err != nil {
			t.Fatal(err)
		}
	}

	// t.Failed() returns false during panicking. We need to forcibly
	// save data on panicking.
	// Refer to: https://github.com/golang/go/issues/49929
	panicked := true
	defer func() {
		report.Report(t, panicked)
	}()
	report.Client = s.run(ctx, t, lg, report.Cluster)
	forcestopCluster(report.Cluster)

	watchProgressNotifyEnabled := report.Cluster.Cfg.WatchProcessNotifyInterval != 0
	validateGotAtLeastOneProgressNotify(t, report.Client, s.watch.requestProgress || watchProgressNotifyEnabled)
	validateConfig := validate.Config{ExpectRevisionUnique: s.traffic.ExpectUniqueRevision()}
	report.Visualize = validate.ValidateAndReturnVisualize(t, lg, validateConfig, report.Client)

	panicked = false
}

func (s testScenario) run(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) (reports []report.ClientReport) {
	g := errgroup.Group{}
	var operationReport, watchReport []report.ClientReport
	finishTraffic := make(chan struct{})

	// using baseTime time-measuring operation to get monotonic clock reading
	// see https://github.com/golang/go/blob/master/src/time/time.go#L17
	baseTime := time.Now()
	ids := identity.NewIdProvider()
	g.Go(func() error {
		defer close(finishTraffic)
		injectFailpoints(ctx, t, lg, clus, s.failpoint)
		time.Sleep(time.Second)
		return nil
	})
	maxRevisionChan := make(chan int64, 1)
	g.Go(func() error {
		defer close(maxRevisionChan)
		operationReport = traffic.SimulateTraffic(ctx, t, lg, clus, s.profile, s.traffic, finishTraffic, baseTime, ids)
		maxRevisionChan <- operationsMaxRevision(operationReport)
		return nil
	})
	g.Go(func() error {
		watchReport = collectClusterWatchEvents(ctx, t, clus, maxRevisionChan, s.watch, baseTime, ids)
		return nil
	})
	g.Wait()
	return append(operationReport, watchReport...)
}

func operationsMaxRevision(reports []report.ClientReport) int64 {
	var maxRevision int64
	for _, r := range reports {
		for _, op := range r.KeyValue {
			resp := op.Output.(model.MaybeEtcdResponse)
			if resp.Revision > maxRevision {
				maxRevision = resp.Revision
			}
		}
	}
	return maxRevision
}

// forcestopCluster stops the etcd member with signal kill.
func forcestopCluster(clus *e2e.EtcdProcessCluster) error {
	for _, member := range clus.Procs {
		member.Kill()
	}
	return clus.ConcurrentStop()
}
