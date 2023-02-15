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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/linearizability/identity"
	"go.etcd.io/etcd/tests/v3/linearizability/model"
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
			keyCount:     4,
			leaseTTL:     DefaultLeaseTTL,
			largePutSize: 32769,
			writes: []requestChance{
				{operation: Put, chance: 50},
				{operation: LargePut, chance: 5},
				{operation: Delete, chance: 10},
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
			keyCount:     4,
			largePutSize: 32769,
			leaseTTL:     DefaultLeaseTTL,
			writes: []requestChance{
				{operation: Put, chance: 90},
				{operation: LargePut, chance: 5},
			},
		},
	}
	defaultTraffic = LowTraffic
	trafficList    = []trafficConfig{
		LowTraffic, HighTraffic,
	}
)

func TestLinearizability(t *testing.T) {
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
			clus, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&scenario.config))
			if err != nil {
				t.Fatal(err)
			}
			defer clus.Close()
			operations, watchResponses := testLinearizability(ctx, t, lg, clus, FailpointConfig{
				failpoint:           scenario.failpoint,
				count:               1,
				retries:             3,
				waitBetweenTriggers: waitBetweenFailpointTriggers,
			}, *scenario.traffic)
			forcestopCluster(clus)
			watchProgressNotifyEnabled := clus.Cfg.WatchProcessNotifyInterval != 0
			validateWatchResponses(t, watchResponses, watchProgressNotifyEnabled)
			watchEvents := watchEvents(watchResponses)
			validateEventsMatch(t, watchEvents)
			patchedOperations := patchOperationBasedOnWatchEvents(operations, longestHistory(watchEvents))
			checkOperationsAndPersistResults(t, lg, patchedOperations, clus)
		})
	}
}

func testLinearizability(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, failpoint FailpointConfig, traffic trafficConfig) (operations []porcupine.Operation, responses [][]watchResponse) {
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

func patchOperationBasedOnWatchEvents(operations []porcupine.Operation, watchEvents []watchEvent) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))
	persisted := map[model.EtcdOperation]watchEvent{}
	for _, op := range watchEvents {
		persisted[op.Op] = op
	}
	lastObservedOperation := lastOperationObservedInWatch(operations, persisted)

	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		resp := op.Output.(model.EtcdResponse)
		if resp.Err == nil || op.Call > lastObservedOperation.Call || request.Type != model.Txn {
			// Cannot patch those requests.
			newOperations = append(newOperations, op)
			continue
		}
		event := matchWatchEvent(request.Txn, persisted)
		if event != nil {
			// Set revision and time based on watchEvent.
			op.Return = event.Time.UnixNano()
			op.Output = model.EtcdResponse{
				Revision:      event.Revision,
				ResultUnknown: true,
			}
			newOperations = append(newOperations, op)
			continue
		}
		if hasNonUniqueWriteOperation(request.Txn) && !hasUniqueWriteOperation(request.Txn) {
			// Leave operation as it is as we cannot match non-unique operations to watch events.
			newOperations = append(newOperations, op)
			continue
		}
		// Remove non persisted operations
	}
	return newOperations
}

func lastOperationObservedInWatch(operations []porcupine.Operation, watchEvents map[model.EtcdOperation]watchEvent) porcupine.Operation {
	var maxCallTime int64
	var lastOperation porcupine.Operation
	for _, op := range operations {
		request := op.Input.(model.EtcdRequest)
		if request.Type != model.Txn {
			continue
		}
		event := matchWatchEvent(request.Txn, watchEvents)
		if event != nil && op.Call > maxCallTime {
			maxCallTime = op.Call
			lastOperation = op
		}
	}
	return lastOperation
}

func matchWatchEvent(request *model.TxnRequest, watchEvents map[model.EtcdOperation]watchEvent) *watchEvent {
	for _, etcdOp := range request.Ops {
		if etcdOp.Type == model.Put {
			// Remove LeaseID which is not exposed in watch.
			event, ok := watchEvents[model.EtcdOperation{
				Type:  etcdOp.Type,
				Key:   etcdOp.Key,
				Value: etcdOp.Value,
			}]
			if ok {
				return &event
			}
		}
	}
	return nil
}

func hasNonUniqueWriteOperation(request *model.TxnRequest) bool {
	for _, etcdOp := range request.Ops {
		if etcdOp.Type == model.Put || etcdOp.Type == model.Delete {
			return true
		}
	}
	return false
}

func hasUniqueWriteOperation(request *model.TxnRequest) bool {
	for _, etcdOp := range request.Ops {
		if etcdOp.Type == model.Put {
			return true
		}
	}
	return false
}

func triggerFailpoints(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, config FailpointConfig) {
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
		lg.Info("Triggering failpoint\n", zap.String("failpoint", config.failpoint.Name()))
		err = config.failpoint.Trigger(ctx, t, lg, clus)
		if err != nil {
			lg.Info("Failed to trigger failpoint", zap.String("failpoint", config.failpoint.Name()), zap.Error(err))
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

func simulateTraffic(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, config trafficConfig) []porcupine.Operation {
	mux := sync.Mutex{}
	endpoints := clus.EndpointsV3()

	ids := identity.NewIdProvider()
	lm := identity.NewLeaseIdStorage()
	h := model.History{}
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
		go func(c *recordingClient, clientId int) {
			defer wg.Done()
			defer c.Close()

			config.traffic.Run(ctx, clientId, c, limiter, ids, lm)
			mux.Lock()
			h = h.Merge(c.history.History)
			mux.Unlock()
		}(c, i)
	}
	wg.Wait()
	endTime := time.Now()
	operations := h.Operations()
	lg.Info("Recorded operations", zap.Int("count", len(operations)))

	qps := float64(len(operations)) / float64(endTime.Sub(startTime)) * float64(time.Second)
	lg.Info("Average traffic", zap.Float64("qps", qps))
	if qps < config.minimalQPS {
		t.Errorf("Requiring minimal %f qps for test results to be reliable, got %f qps", config.minimalQPS, qps)
	}
	return operations
}

type trafficConfig struct {
	name        string
	minimalQPS  float64
	maximalQPS  float64
	clientCount int
	traffic     Traffic
}

func watchEvents(responses [][]watchResponse) [][]watchEvent {
	ops := make([][]watchEvent, len(responses))
	for i, resps := range responses {
		ops[i] = toWatchEvents(resps)
	}
	return ops
}

func validateEventsMatch(t *testing.T, histories [][]watchEvent) {
	longestHistory := longestHistory(histories)
	for i := 0; i < len(histories); i++ {
		length := len(histories[i])
		// We compare prefix of watch events, as we are not guaranteed to collect all events from each node.
		if diff := cmp.Diff(longestHistory[:length], histories[i][:length], cmpopts.IgnoreFields(watchEvent{}, "Time")); diff != "" {
			t.Errorf("Events in watches do not match, %s", diff)
		}
	}
}

func longestHistory(histories [][]watchEvent) []watchEvent {
	longestIndex := 0
	for i, history := range histories {
		if len(history) > len(histories[longestIndex]) {
			longestIndex = i
		}
	}
	return histories[longestIndex]
}

func checkOperationsAndPersistResults(t *testing.T, lg *zap.Logger, operations []porcupine.Operation, clus *e2e.EtcdProcessCluster) {
	path := testResultsDirectory(t)

	linearizable, info := porcupine.CheckOperationsVerbose(model.Etcd, operations, 5*time.Minute)
	if linearizable == porcupine.Illegal {
		t.Error("Model is not linearizable")
	}
	if linearizable == porcupine.Unknown {
		t.Error("Linearization timed out")
	}
	if linearizable != porcupine.Ok {
		persistOperationHistory(t, lg, path, operations)
		for _, member := range clus.Procs {
			persistMemberDataDir(t, lg, member, filepath.Join(path, member.Config().Name))
		}
	}

	visualizationPath := filepath.Join(path, "history.html")
	lg.Info("Saving visualization", zap.String("path", visualizationPath))
	err := porcupine.VisualizePath(model.Etcd, info, visualizationPath)
	if err != nil {
		t.Errorf("Failed to visualize, err: %v", err)
	}
}

func persistOperationHistory(t *testing.T, lg *zap.Logger, path string, operations []porcupine.Operation) {
	historyFilePath := filepath.Join(path, "history.json")
	lg.Info("Saving operation history", zap.String("path", historyFilePath))
	file, err := os.OpenFile(historyFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		t.Errorf("Failed to save operation history: %v", err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, op := range operations {
		err := encoder.Encode(op)
		if err != nil {
			t.Errorf("Failed to encode operation: %v", err)
		}
	}
}

func persistMemberDataDir(t *testing.T, lg *zap.Logger, member e2e.EtcdProcess, path string) {
	lg.Info("Saving member data dir", zap.String("member", member.Config().Name), zap.String("path", path))
	err := os.Rename(member.Config().DataDirPath, path)
	if err != nil {
		t.Fatal(err)
	}
}

func testResultsDirectory(t *testing.T) string {
	path, err := filepath.Abs(filepath.Join(resultsDirectory, strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll(path)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(path, 0700)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// forcestopCluster stops the etcd member with signal kill.
func forcestopCluster(clus *e2e.EtcdProcessCluster) error {
	for _, member := range clus.Procs {
		member.Kill()
	}
	return clus.Stop()
}
