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

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

const (
	// waitBetweenFailpointTriggers
	waitBetweenFailpointTriggers = time.Second
)

var (
	LowTraffic = trafficConfig{
		name:            "LowTraffic",
		minimalQPS:      100,
		maximalQPS:      200,
		clientCount:     8,
		requestProgress: false,
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
		name:            "HighTraffic",
		minimalQPS:      200,
		maximalQPS:      1000,
		clientCount:     12,
		requestProgress: false,
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
	ReqProgTraffic = trafficConfig{
		name:            "RequestProgressTraffic",
		minimalQPS:      200,
		maximalQPS:      1000,
		clientCount:     12,
		requestProgress: true,
		traffic: traffic{
			keyCount:     10,
			largePutSize: 8196,
			leaseTTL:     DefaultLeaseTTL,
			writes: []requestChance{
				{operation: Put, chance: 95},
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
	v, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	if err != nil {
		t.Fatalf("Failed checking etcd version binary, binary: %q, err: %v", e2e.BinPath.Etcd, err)
	}
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
			failpoint: RandomFailpoint,
			traffic:   &traffic,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithSnapshotCount(100),
				e2e.WithGoFailEnabled(true),
				e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
				e2e.WithWatchProcessNotifyInterval(100*time.Millisecond),
			),
		})
		clusterOfSize3Options := []e2e.EPClusterOption{
			e2e.WithIsPeerTLS(true),
			e2e.WithSnapshotCount(100),
			e2e.WithPeerProxy(true),
			e2e.WithGoFailEnabled(true),
			e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
			e2e.WithWatchProcessNotifyInterval(100 * time.Millisecond),
		}
		if !v.LessThan(version.V3_6) {
			clusterOfSize3Options = append(clusterOfSize3Options, e2e.WithSnapshotCatchUpEntries(100))
		}
		scenarios = append(scenarios, scenario{
			name:      "ClusterOfSize3/" + traffic.name,
			failpoint: RandomFailpoint,
			traffic:   &traffic,
			config:    *e2e.NewConfig(clusterOfSize3Options...),
		})
	}
	scenarios = append(scenarios, scenario{
		name:      "Issue14370",
		failpoint: RaftBeforeSavePanic,
		config: *e2e.NewConfig(
			e2e.WithClusterSize(1),
			e2e.WithGoFailEnabled(true),
		),
	})
	scenarios = append(scenarios, scenario{
		name:      "Issue14685",
		failpoint: DefragBeforeCopyPanic,
		config: *e2e.NewConfig(
			e2e.WithClusterSize(1),
			e2e.WithGoFailEnabled(true),
		),
	})
	scenarios = append(scenarios, scenario{
		name:      "Issue13766",
		failpoint: KillFailpoint,
		traffic:   &HighTraffic,
		config: *e2e.NewConfig(
			e2e.WithSnapshotCount(100),
		),
	})
	scenarios = append(scenarios, scenario{
		name:      "Issue15220",
		failpoint: RandomFailpoint,
		traffic:   &ReqProgTraffic,
		config: *e2e.NewConfig(
			e2e.WithClusterSize(1),
		),
	})
	if v.Compare(version.V3_5) >= 0 {
		scenarios = append(scenarios, scenario{
			name:      "Issue15271",
			failpoint: BlackholeUntilSnapshot,
			traffic:   &HighTraffic,
			config: *e2e.NewConfig(
				e2e.WithSnapshotCount(100),
				e2e.WithPeerProxy(true),
				e2e.WithIsPeerTLS(true),
			),
		})
	}
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
	validateWatchResponses(t, r.clus, r.responses, traffic.requestProgress || watchProgressNotifyEnabled)

	r.events = watchEvents(r.responses)
	validateEventsMatch(t, r.events)

	r.patchedOperations = patchOperationBasedOnWatchEvents(r.operations, longestHistory(r.events))
	r.visualizeHistory = model.ValidateOperationHistoryAndReturnVisualize(t, lg, r.patchedOperations)
}

func runScenario(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, traffic trafficConfig, failpoint FailpointConfig) (operations []porcupine.Operation, responses [][]watchResponse) {
	g := errgroup.Group{}
	finishTraffic := make(chan struct{})

	g.Go(func() error {
		defer close(finishTraffic)
		injectFailpoints(ctx, t, lg, clus, failpoint)
		time.Sleep(time.Second)
		return nil
	})
	maxRevisionChan := make(chan int64, 1)
	g.Go(func() error {
		defer close(maxRevisionChan)
		operations = simulateTraffic(ctx, t, lg, clus, traffic, finishTraffic)
		maxRevisionChan <- operationsMaxRevision(operations)
		return nil
	})
	g.Go(func() error {
		responses = collectClusterWatchEvents(ctx, t, clus, maxRevisionChan, traffic.requestProgress)
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
	return clus.ConcurrentStop()
}
