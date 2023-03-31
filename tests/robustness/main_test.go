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
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"go.etcd.io/etcd/tests/v3/framework"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/robustness"
)

var testRunner = framework.E2eTestRunner

func TestMain(m *testing.M) {
	testRunner.TestMain(m)
}

const (
	// waitBetweenFailpointTriggers
	waitBetweenFailpointTriggers = time.Second
)

var (
	LowTraffic = robustness.TrafficConfig{
		Name:        "LowTraffic",
		MinimalQPS:  100,
		MaximalQPS:  200,
		ClientCount: 8,
		Traffic: robustness.Traffic{
			KeyCount:     10,
			LeaseTTL:     robustness.DefaultLeaseTTL,
			LargePutSize: 32769,
			Writes: []robustness.RequestChance{
				{Operation: robustness.Put, Chance: 45},
				{Operation: robustness.LargePut, Chance: 5},
				{Operation: robustness.Delete, Chance: 10},
				{Operation: robustness.MultiOpTxn, Chance: 10},
				{Operation: robustness.PutWithLease, Chance: 10},
				{Operation: robustness.LeaseRevoke, Chance: 10},
				{Operation: robustness.CompareAndSet, Chance: 10},
			},
		},
	}
	HighTraffic = robustness.TrafficConfig{
		Name:        "HighTraffic",
		MinimalQPS:  200,
		MaximalQPS:  1000,
		ClientCount: 12,
		Traffic: robustness.Traffic{
			KeyCount:     10,
			LargePutSize: 32769,
			LeaseTTL:     robustness.DefaultLeaseTTL,
			Writes: []robustness.RequestChance{
				{Operation: robustness.Put, Chance: 85},
				{Operation: robustness.MultiOpTxn, Chance: 10},
				{Operation: robustness.LargePut, Chance: 5},
			},
		},
	}
	defaultTraffic = LowTraffic
	trafficList    = []robustness.TrafficConfig{
		LowTraffic, HighTraffic,
	}
)

func TestRobustness(t *testing.T) {
	testRunner.BeforeTest(t)
	type scenario struct {
		name      string
		failpoint robustness.Failpoint
		config    e2e.EtcdProcessClusterConfig
		traffic   *robustness.TrafficConfig
	}
	scenarios := []scenario{}
	for _, traffic := range trafficList {
		scenarios = append(scenarios, scenario{
			name:      "ClusterOfSize1/" + traffic.Name,
			failpoint: robustness.RandomOneNodeClusterFailpoint,
			traffic:   &traffic,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithSnapshotCount(100),
				e2e.WithGoFailEnabled(true),
				e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
				e2e.WithWatchProcessNotifyInterval(100*time.Millisecond),
			),
		})
		scenarios = append(scenarios, scenario{
			name:      "ClusterOfSize3/" + traffic.Name,
			failpoint: robustness.RandomMultiNodeClusterFailpoint,
			traffic:   &traffic,
			config: *e2e.NewConfig(
				e2e.WithSnapshotCount(100),
				e2e.WithPeerProxy(true),
				e2e.WithGoFailEnabled(true),
				e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
				e2e.WithWatchProcessNotifyInterval(100*time.Millisecond),
			),
		})
	}
	scenarios = append(scenarios, []scenario{
		{
			name:      "Issue14370",
			failpoint: robustness.RaftBeforeSavePanic,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithGoFailEnabled(true),
			),
		},
		{
			name:      "Issue14685",
			failpoint: robustness.DefragBeforeCopyPanic,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithGoFailEnabled(true),
			),
		},
		{
			name:      "Issue13766",
			failpoint: robustness.KillFailpoint,
			traffic:   &HighTraffic,
			config: *e2e.NewConfig(
				e2e.WithSnapshotCount(100),
			),
		},
		{
			name:      "Snapshot",
			failpoint: robustness.RandomSnapshotFailpoint,
			traffic:   &HighTraffic,
			config: *e2e.NewConfig(
				e2e.WithGoFailEnabled(true),
				e2e.WithSnapshotCount(100),
				e2e.WithSnapshotCatchUpEntries(100),
				e2e.WithPeerProxy(true),
			),
		},
	}...)
	for _, scenario := range scenarios {
		if scenario.traffic == nil {
			scenario.traffic = &defaultTraffic
		}

		t.Run(scenario.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			scenario.config.Logger = lg
			ctx := context.Background()
			testRobustness(ctx, t, lg, scenario.config, scenario.traffic, robustness.FailpointConfig{
				Failpoint:           scenario.failpoint,
				Count:               1,
				Retries:             3,
				WaitBetweenTriggers: waitBetweenFailpointTriggers,
			})
		})
	}
}

func testRobustness(ctx context.Context, t *testing.T, lg *zap.Logger, config e2e.EtcdProcessClusterConfig, traffic *robustness.TrafficConfig, failpoint robustness.FailpointConfig) {
	r := robustness.NewReport(lg)
	var err error
	r.Cluster, err = e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&config))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Cluster.Close()

	defer func() {
		r.Report(t)
	}()
	r.Operations, r.Responses = runScenario(ctx, t, lg, r.Cluster, *traffic, failpoint)
	forcestopCluster(r.Cluster)

	watchProgressNotifyEnabled := r.Cluster.Cfg.WatchProcessNotifyInterval != 0
	robustness.ValidateWatchResponses(t, r.Responses, watchProgressNotifyEnabled)

	r.Events = robustness.WatchEvents(r.Responses)
	robustness.ValidateEventsMatch(t, r.Events)

	r.PatchedOperations = robustness.PatchOperationBasedOnWatchEvents(r.Operations, robustness.LongestHistory(r.Events))
	r.VisualizeHistory = model.ValidateOperationHistoryAndReturnVisualize(t, lg, r.PatchedOperations)
}

func runScenario(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, traffic robustness.TrafficConfig, failpoint robustness.FailpointConfig) (operations []porcupine.Operation, responses [][]robustness.WatchResponse) {
	// Run multiple test components (Traffic, failpoints, etc) in parallel and use canceling context to propagate stop signal.
	g := errgroup.Group{}
	trafficCtx, trafficCancel := context.WithCancel(ctx)
	g.Go(func() error {
		robustness.TriggerFailpoints(ctx, t, lg, clus, failpoint)
		time.Sleep(time.Second)
		trafficCancel()
		return nil
	})
	maxRevisionChan := make(chan int64, 1)
	g.Go(func() error {
		operations = robustness.SimulateTraffic(trafficCtx, t, lg, clus, traffic)
		time.Sleep(time.Second)
		maxRevisionChan <- operationsMaxRevision(operations)
		return nil
	})
	g.Go(func() error {
		responses = robustness.CollectClusterWatchEvents(ctx, t, clus, maxRevisionChan)
		return nil
	})
	g.Wait()
	return operations, responses
}

func operationsMaxRevision(operations []porcupine.Operation) int64 {
	var maxRevision int64
	for _, op := range operations {
		revision := op.Output.(model.EtcdResponse).Revision
		if revision > maxRevision {
			maxRevision = revision
		}
	}
	return maxRevision
}

// forcestopCluster stops the etcd member with signal kill.
func forcestopCluster(clus *e2e.EtcdProcessCluster) error {
	for _, member := range clus.Procs {
		member.Kill()
	}
	return clus.Stop()
}
