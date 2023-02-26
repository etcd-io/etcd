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

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

const (
	// waitBetweenFailpointTriggers
	waitBetweenFailpointTriggers = time.Second
)

var (
	LowTraffic = trafficConfig{
		name:        "LowTraffic",
		minimalQPS:  100,
		maximalQPS:  200,
		clientCount: 8,
		traffic: traffic{
			keyCount:     10,
			leaseTTL:     DefaultLeaseTTL,
			largePutSize: 32769,
			writes: []requestChance{
				{operation: Put, chance: 45},
				{operation: LargePut, chance: 5},
				{operation: Delete, chance: 10},
				{operation: MultiOpTxn, chance: 10},
				{operation: PutWithLease, chance: 10},
				{operation: LeaseRevoke, chance: 10},
				{operation: CompareAndSet, chance: 10},
			},
		},
	}
	HighTraffic = trafficConfig{
		name:        "HighTraffic",
		minimalQPS:  200,
		maximalQPS:  1000,
		clientCount: 12,
		traffic: traffic{
			keyCount:     10,
			largePutSize: 32769,
			leaseTTL:     DefaultLeaseTTL,
			writes: []requestChance{
				{operation: Put, chance: 85},
				{operation: MultiOpTxn, chance: 10},
				{operation: LargePut, chance: 5},
			},
		},
	}
	defaultTraffic = LowTraffic
	trafficList    = []trafficConfig{
		LowTraffic, HighTraffic,
	}
)

func TestRobustness(t *testing.T) {
	testRunner.BeforeTest(t)
	type scenario struct {
		name      string
		failpoint Failpoint
		config    e2e.EtcdProcessClusterConfig
		traffic   *trafficConfig
	}
	scenarios := []scenario{}
	for _, traffic := range trafficList {
		scenarios = append(scenarios, scenario{
			name:      "ClusterOfSize1/" + traffic.name,
			failpoint: RandomOneNodeClusterFailpoint,
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
			name:      "ClusterOfSize3/" + traffic.name,
			failpoint: RandomMultiNodeClusterFailpoint,
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
			failpoint: RaftBeforeSavePanic,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithGoFailEnabled(true),
			),
		},
		{
			name:      "Issue14685",
			failpoint: DefragBeforeCopyPanic,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithGoFailEnabled(true),
			),
		},
		{
			name:      "Issue13766",
			failpoint: KillFailpoint,
			traffic:   &HighTraffic,
			config: *e2e.NewConfig(
				e2e.WithSnapshotCount(100),
			),
		},
		{
			name:      "Snapshot",
			failpoint: RandomSnapshotFailpoint,
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
			testRobustness(ctx, t, lg, scenario.config, scenario.traffic, FailpointConfig{
				failpoint:           scenario.failpoint,
				count:               1,
				retries:             3,
				waitBetweenTriggers: waitBetweenFailpointTriggers,
			})
		})
	}
}

func testRobustness(ctx context.Context, t *testing.T, lg *zap.Logger, config e2e.EtcdProcessClusterConfig, traffic *trafficConfig, failpoint FailpointConfig) {
	r := report{lg: lg}
	var err error
	r.clus, err = e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&config))
	if err != nil {
		t.Fatal(err)
	}
	defer r.clus.Close()

	defer func() {
		r.Report(t)
	}()
	r.operations, r.responses = runScenario(ctx, t, lg, r.clus, *traffic, failpoint)
	forcestopCluster(r.clus)

	watchProgressNotifyEnabled := r.clus.Cfg.WatchProcessNotifyInterval != 0
	validateWatchResponses(t, r.responses, watchProgressNotifyEnabled)

	r.events = watchEvents(r.responses)
	validateEventsMatch(t, r.events)

	r.patchedOperations = patchOperationBasedOnWatchEvents(r.operations, longestHistory(r.events))
	r.visualizeHistory = model.ValidateOperationHistoryAndReturnVisualize(t, lg, r.patchedOperations)
}

func runScenario(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, traffic trafficConfig, failpoint FailpointConfig) (operations []porcupine.Operation, responses [][]watchResponse) {
	// Run multiple test components (traffic, failpoints, etc) in parallel and use canceling context to propagate stop signal.
	g := errgroup.Group{}
	trafficCtx, trafficCancel := context.WithCancel(ctx)
	g.Go(func() error {
		triggerFailpoints(ctx, t, lg, clus, failpoint)
		time.Sleep(time.Second)
		trafficCancel()
		return nil
	})
	watchCtx, watchCancel := context.WithCancel(ctx)
	g.Go(func() error {
		operations = simulateTraffic(trafficCtx, t, lg, clus, traffic)
		time.Sleep(time.Second)
		watchCancel()
		return nil
	})
	g.Go(func() error {
		responses = collectClusterWatchEvents(watchCtx, t, lg, clus)
		return nil
	})
	g.Wait()
	return operations, responses
}

// forcestopCluster stops the etcd member with signal kill.
func forcestopCluster(clus *e2e.EtcdProcessCluster) error {
	for _, member := range clus.Procs {
		member.Kill()
	}
	return clus.Stop()
}
