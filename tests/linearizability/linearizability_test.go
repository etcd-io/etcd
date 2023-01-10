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

package linearizability

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	// minimalQPS is used to validate if enough traffic is send to make tests accurate.
	minimalQPS = 100.0
	// maximalQPS limits number of requests send to etcd to avoid linearizability analysis taking too long.
	maximalQPS = 200.0
	// waitBetweenFailpointTriggers
	waitBetweenFailpointTriggers = time.Second
)

func TestLinearizability(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name      string
		failpoint Failpoint
		config    e2e.EtcdProcessClusterConfig
	}{
		{
			name:      "ClusterOfSize1",
			failpoint: RandomFailpoint,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithPeerProxy(true),
				e2e.WithGoFailEnabled(true),
				e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
			),
		},
		{
			name:      "ClusterOfSize3",
			failpoint: RandomFailpoint,
			config: *e2e.NewConfig(
				e2e.WithPeerProxy(true),
				e2e.WithGoFailEnabled(true),
				e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
			),
		},
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
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			clus, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&tc.config))
			if err != nil {
				t.Fatal(err)
			}
			defer clus.Close()
			operations, events := testLinearizability(ctx, t, clus, FailpointConfig{
				failpoint:           tc.failpoint,
				count:               1,
				retries:             3,
				waitBetweenTriggers: waitBetweenFailpointTriggers,
			}, trafficConfig{
				minimalQPS:  minimalQPS,
				maximalQPS:  maximalQPS,
				clientCount: 8,
				traffic:     DefaultTraffic,
			})
			longestHistory, remainingEvents := pickLongestHistory(events)
			validateEventsMatch(t, longestHistory, remainingEvents)
			operations = patchOperationBasedOnWatchEvents(operations, longestHistory)
			checkOperationsAndPersistResults(t, operations, clus)
		})
	}
}

func testLinearizability(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, failpoint FailpointConfig, traffic trafficConfig) (operations []porcupine.Operation, events [][]watchEvent) {
	// Run multiple test components (traffic, failpoints, etc) in parallel and use canceling context to propagate stop signal.
	g := errgroup.Group{}
	trafficCtx, trafficCancel := context.WithCancel(ctx)
	g.Go(func() error {
		triggerFailpoints(ctx, t, clus, failpoint)
		time.Sleep(time.Second)
		trafficCancel()
		return nil
	})
	watchCtx, watchCancel := context.WithCancel(ctx)
	g.Go(func() error {
		operations = simulateTraffic(trafficCtx, t, clus, traffic)
		time.Sleep(time.Second)
		watchCancel()
		return nil
	})
	g.Go(func() error {
		events = collectClusterWatchEvents(watchCtx, t, clus)
		return nil
	})
	g.Wait()
	return operations, events
}

func patchOperationBasedOnWatchEvents(operations []porcupine.Operation, watchEvents []watchEvent) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))
	persisted := map[EtcdOperation]watchEvent{}
	for _, op := range watchEvents {
		persisted[op.Op] = op
	}
	lastObservedEventTime := watchEvents[len(watchEvents)-1].Time

	for _, op := range operations {
		resp := op.Output.(EtcdResponse)
		if resp.Err == nil || op.Call > lastObservedEventTime.UnixNano() {
			// No need to patch successfully requests and cannot patch requests outside observed window.
			newOperations = append(newOperations, op)
			continue
		}
		event, hasUniqueWriteOperation := matchWatchEvent(op, persisted)
		if event != nil {
			// Set revision and time based on watchEvent.
			op.Return = event.Time.UnixNano()
			op.Output = EtcdResponse{
				Revision:      event.Revision,
				ResultUnknown: true,
			}
			newOperations = append(newOperations, op)
			continue
		}
		if hasWriteOperation(op) && !hasUniqueWriteOperation {
			// Leave operation as it is as we cannot match non-unique operations to watch events.
			newOperations = append(newOperations, op)
			continue
		}
		// Remove non persisted operations
	}
	return newOperations
}

func matchWatchEvent(op porcupine.Operation, watchEvents map[EtcdOperation]watchEvent) (event *watchEvent, hasUniqueWriteOperation bool) {
	request := op.Input.(EtcdRequest)
	for _, etcdOp := range request.Ops {
		if isWrite(etcdOp.Type) && inUnique(etcdOp.Type) {
			// We expect all put to be unique as they write unique value.
			hasUniqueWriteOperation = true
			opType := etcdOp.Type
			if opType == PutWithLease {
				opType = Put
			}
			event, ok := watchEvents[EtcdOperation{
				Type:  opType,
				Key:   etcdOp.Key,
				Value: etcdOp.Value,
			}]
			if ok {
				return &event, hasUniqueWriteOperation
			}
		}
	}
	return nil, hasUniqueWriteOperation
}

func hasWriteOperation(op porcupine.Operation) bool {
	request := op.Input.(EtcdRequest)
	for _, etcdOp := range request.Ops {
		if isWrite(etcdOp.Type) {
			return true
		}
	}
	return false
}

func triggerFailpoints(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, config FailpointConfig) {
	var err error
	successes := 0
	failures := 0
	for _, proc := range clus.Procs {
		if !config.failpoint.Available(proc) {
			t.Errorf("Failpoint %q not available on %s", config.failpoint.Name(), proc.Config().Name)
			return
		}
	}
	for successes < config.count && failures < config.retries {
		time.Sleep(config.waitBetweenTriggers)
		err = config.failpoint.Trigger(t, ctx, clus)
		if err != nil {
			t.Logf("Failed to trigger failpoint %q, err: %v\n", config.failpoint.Name(), err)
			failures++
			continue
		}
		successes++
	}
	if successes < config.count || failures >= config.retries {
		t.Errorf("failed to trigger failpoints enough times, err: %v", err)
	}
}

type FailpointConfig struct {
	failpoint           Failpoint
	count               int
	retries             int
	waitBetweenTriggers time.Duration
}

func simulateTraffic(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, config trafficConfig) []porcupine.Operation {
	mux := sync.Mutex{}
	endpoints := clus.EndpointsV3()

	ids := newIdProvider()
	lm := newClientId2LeaseIdMapper()
	h := history{}
	limiter := rate.NewLimiter(rate.Limit(config.maximalQPS), 200)

	startTime := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < config.clientCount; i++ {
		wg.Add(1)
		endpoints := []string{endpoints[i%len(endpoints)]}
		c, err := NewClient(endpoints, ids)
		if err != nil {
			t.Fatal(err)
		}
		go func(c *recordingClient) {
			defer wg.Done()
			defer c.Close()

			config.traffic.Run(ctx, c, limiter, ids, lm)
			mux.Lock()
			h = h.Merge(c.history.history)
			mux.Unlock()
		}(c)
	}
	wg.Wait()
	endTime := time.Now()
	operations := h.Operations()
	t.Logf("Recorded %d operations", len(operations))

	qps := float64(len(operations)) / float64(endTime.Sub(startTime)) * float64(time.Second)
	t.Logf("Average traffic: %f qps", qps)
	if qps < config.minimalQPS {
		t.Errorf("Requiring minimal %f qps for test results to be reliable, got %f qps", config.minimalQPS, qps)
	}
	return operations
}

type trafficConfig struct {
	minimalQPS  float64
	maximalQPS  float64
	clientCount int
	traffic     Traffic
}

func pickLongestHistory(ops [][]watchEvent) (longest []watchEvent, rest [][]watchEvent) {
	sort.Slice(ops, func(i, j int) bool {
		return len(ops[i]) > len(ops[j])
	})
	return ops[0], ops[1:]
}

func validateEventsMatch(t *testing.T, longestHistory []watchEvent, other [][]watchEvent) {
	for i := 0; i < len(other); i++ {
		length := len(other[i])
		// We compare prefix of watch events, as we are not guaranteed to collect all events from each node.
		if diff := cmp.Diff(longestHistory[:length], other[i][:length], cmpopts.IgnoreFields(watchEvent{}, "Time")); diff != "" {
			t.Errorf("Events in watches do not match, %s", diff)
		}
	}
}

func checkOperationsAndPersistResults(t *testing.T, operations []porcupine.Operation, clus *e2e.EtcdProcessCluster) {
	path, err := testResultsDirectory(t)
	if err != nil {
		t.Error(err)
	}

	linearizable, info := porcupine.CheckOperationsVerbose(etcdModel, operations, 0)
	if linearizable != porcupine.Ok {
		t.Error("Model is not linearizable")
		persistMemberDataDir(t, clus, path)
	}

	visualizationPath := filepath.Join(path, "history.html")
	t.Logf("saving visualization to %q", visualizationPath)
	err = porcupine.VisualizePath(etcdModel, info, visualizationPath)
	if err != nil {
		t.Errorf("Failed to visualize, err: %v", err)
	}
}

func persistMemberDataDir(t *testing.T, clus *e2e.EtcdProcessCluster, path string) {
	for _, member := range clus.Procs {
		memberDataDir := filepath.Join(path, member.Config().Name)
		err := os.RemoveAll(memberDataDir)
		if err != nil {
			t.Error(err)
		}
		t.Logf("saving %s data dir to %q", member.Config().Name, memberDataDir)
		err = os.Rename(member.Config().DataDirPath, memberDataDir)
		if err != nil {
			t.Error(err)
		}
	}
}

func testResultsDirectory(t *testing.T) (string, error) {
	path, err := filepath.Abs(filepath.Join(resultsDirectory, strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		return path, err
	}
	err = os.MkdirAll(path, 0700)
	if err != nil {
		return path, err
	}
	return path, nil
}
