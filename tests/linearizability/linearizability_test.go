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
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"go.etcd.io/etcd/tests/v3/linearizability/identity"
	"go.etcd.io/etcd/tests/v3/linearizability/model"
)

const (
	// waitBetweenFailpointTriggers
	waitBetweenFailpointTriggers = time.Second
)

var (
	DefaultTrafficConfig = trafficConfig{
		minimalQPS:  100,
		maximalQPS:  200,
		clientCount: 8,
		traffic:     DefaultTraffic,
	}
	HighTrafficConfig = trafficConfig{
		minimalQPS:  200,
		maximalQPS:  1000,
		clientCount: 12,
		traffic:     DefaultTraffic,
	}
	AuthTrafficConfig = trafficConfig{
		minimalQPS:  100,
		maximalQPS:  200,
		clientCount: 8,
		traffic:     DefaultTrafficWithAuth,
	}
)

func TestLinearizability(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name        string
		failpoint   Failpoint
		config      e2e.EtcdProcessClusterConfig
		traffic     *trafficConfig
		clientCount int
	}{
		{
			name:      "ClusterOfSize1",
			failpoint: RandomFailpoint,
			traffic:   &HighTrafficConfig,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(1),
				e2e.WithSnapshotCount(100),
				e2e.WithPeerProxy(true),
				e2e.WithGoFailEnabled(true),
				e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
			),
		},
		{
			name:      "ClusterOfSize3",
			failpoint: RandomFailpoint,
			traffic:   &HighTrafficConfig,
			config: *e2e.NewConfig(
				e2e.WithSnapshotCount(100),
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
		{
			name:      "Issue13766",
			failpoint: KillFailpoint,
			traffic:   &HighTrafficConfig,
			config: *e2e.NewConfig(
				e2e.WithSnapshotCount(100),
			),
		},
		{
			name:      "Issue14571",
			failpoint: EnableAuthKillFailpoint,
			traffic:   &AuthTrafficConfig,
			config: *e2e.NewConfig(
				e2e.WithClusterSize(3),
				e2e.WithSnapshotCount(2),
				e2e.WithSnapshotCatchUpEntries(1),
			),
			clientCount: 3, // actual client count = 9; 3 + (2 test user * 3 endpoints) = 9 clients
		},
	}
	for _, tc := range tcs {
		if tc.traffic == nil {
			tc.traffic = &DefaultTrafficConfig
		}
		if tc.clientCount == 0 {
			tc.clientCount = 8
		}

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			clus, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&tc.config))
			if err != nil {
				t.Fatal(err)
			}
			defer clus.Close()
			lg := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())).Named(tc.name)
			operations, events := testLinearizability(ctx, t, clus, FailpointConfig{
				failpoint:           tc.failpoint,
				count:               1,
				retries:             3,
				waitBetweenTriggers: waitBetweenFailpointTriggers,
			}, *tc.traffic, lg)
			longestHistory, remainingEvents := pickLongestHistory(events)
			validateEventsMatch(t, longestHistory, remainingEvents)
			operations = patchOperationBasedOnWatchEvents(operations, longestHistory)
			checkOperationsAndPersistResults(t, operations, clus, lg)
		})
	}
}

func testLinearizability(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, failpoint FailpointConfig, traffic trafficConfig, lg *zap.Logger) (operations []porcupine.Operation, events [][]watchEvent) {
	// Run multiple test components (traffic, failpoints, etc) in parallel and use canceling context to propagate stop signal.
	g := errgroup.Group{}
	trafficCtx, trafficCancel := context.WithCancel(ctx)
	g.Go(func() error {
		triggerFailpoints(ctx, t, clus, failpoint, lg)
		time.Sleep(time.Second)
		trafficCancel()
		return nil
	})
	watchCtx, watchCancel := context.WithCancel(ctx)
	g.Go(func() error {
		operations = simulateTraffic(trafficCtx, t, clus, traffic, lg)
		time.Sleep(time.Second)
		watchCancel()
		return nil
	})
	g.Go(func() error {
		events = collectClusterWatchEvents(watchCtx, t, clus, traffic.traffic.AuthEnabled())
		return nil
	})
	g.Wait()
	return operations, events
}

func patchOperationBasedOnWatchEvents(operations []porcupine.Operation, watchEvents []watchEvent) []porcupine.Operation {
	newOperations := make([]porcupine.Operation, 0, len(operations))
	persisted := map[model.EtcdOperation]watchEvent{}
	for _, op := range watchEvents {
		persisted[op.Op] = op
	}
	lastObservedOperation := lastOperationObservedInWatch(operations, persisted)

	for _, op := range operations {
		resp := op.Output.(model.EtcdResponse)
		if resp.Err == nil || op.Call > lastObservedOperation.Call {
			// No need to patch successfully requests and cannot patch requests outside observed window.
			newOperations = append(newOperations, op)
			continue
		}
		event, hasUniqueWriteOperation := matchWatchEvent(op, persisted)
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
		if hasWriteOperation(op) && !hasUniqueWriteOperation {
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
		event, _ := matchWatchEvent(op, watchEvents)
		if event != nil && op.Call > maxCallTime {
			maxCallTime = op.Call
			lastOperation = op
		}
	}
	return lastOperation
}

func matchWatchEvent(op porcupine.Operation, watchEvents map[model.EtcdOperation]watchEvent) (event *watchEvent, hasUniqueWriteOperation bool) {
	request := op.Input.(model.EtcdRequest)
	for _, etcdOp := range request.Ops {
		if model.IsWrite(etcdOp.Type) && model.IsUnique(etcdOp.Type) {
			// We expect all put to be unique as they write unique value.
			hasUniqueWriteOperation = true
			opType := etcdOp.Type
			if opType == model.PutWithLease {
				opType = model.Put
			}
			event, ok := watchEvents[model.EtcdOperation{
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
	request := op.Input.(model.EtcdRequest)
	for _, etcdOp := range request.Ops {
		if model.IsWrite(etcdOp.Type) {
			return true
		}
	}
	return false
}

func triggerFailpoints(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, config FailpointConfig, lg *zap.Logger) {
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
		err = config.failpoint.Trigger(t, ctx, clus, lg)
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

func simulateTraffic(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, config trafficConfig, lg *zap.Logger) []porcupine.Operation {
	require.NoError(t, config.traffic.PreRun(ctx, clus.Client(), lg))

	mux := sync.Mutex{}
	endpoints := clus.EndpointsV3()

	ids := identity.NewIdProvider()
	lm := identity.NewLeaseIdStorage()
	h := &model.History{}
	limiter := rate.NewLimiter(rate.Limit(config.maximalQPS), 200)

	startTime := time.Now()
	wg := sync.WaitGroup{}

	for i := 0; i < config.clientCount; i++ {
		i := i

		wg.Add(1)
		endpoints := []string{endpoints[i%len(endpoints)]}
		c, err := NewClient(endpoints, ids, clientOption(config.traffic.AuthEnabled()))
		if err != nil {
			t.Fatal(err)
		}
		go func(c *recordingClient, clientId int) {
			defer wg.Done()
			defer c.Close()

			config.traffic.Run(ctx, clientId, c, limiter, ids, lm)
			mux.Lock()
			h.Merge(c.history.History)
			mux.Unlock()
		}(c, i)
	}

	simulatePostFailpointTraffic(ctx, &wg, endpoints, config.clientCount, ids, h, &mux, config, limiter, lm)

	wg.Wait()
	endTime := time.Now()
	operations := h.Operations()
	lg.Info("Recorded operations", zap.Int("num-of-operation", len(operations)))

	qps := float64(len(operations)) / float64(endTime.Sub(startTime)) * float64(time.Second)
	lg.Info("Average traffic", zap.Float64("qps", qps))
	if qps < config.minimalQPS {
		t.Errorf("Requiring minimal %f qps for test results to be reliable, got %f qps", config.minimalQPS, qps)
	}
	return operations
}

func simulatePostFailpointTraffic(ctx context.Context, wg *sync.WaitGroup, endpoints []string, clientId int, ids identity.Provider, h *model.History, mux *sync.Mutex, config trafficConfig, limiter *rate.Limiter, lm identity.LeaseIdStorage) {
	if !config.traffic.AuthEnabled() {
		return
	}

	// each endpoint has a client with each auth user
	for _, ep := range endpoints {
		eps := []string{ep}
		for _, user := range users {
			user := user
			wg.Add(1)
			i := clientId + 1
			go func(clientId int) {
				defer wg.Done()
				select {
				case <-ctx.Done():
					// Triggering failpoint is finished, start the traffic
				}
				c, err := NewClient(eps, ids, integration.WithAuth(user.userName, user.userPassword))
				if err != nil {
					panic(err)
				}
				defer c.Close()
				cctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				config.traffic.Run(cctx, clientId, c, limiter, ids, lm)
				mux.Lock()
				h.Merge(c.history.History)
				mux.Unlock()
			}(i)

			clientId++
		}
	}
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

func checkOperationsAndPersistResults(t *testing.T, operations []porcupine.Operation, clus *e2e.EtcdProcessCluster, lg *zap.Logger) {
	path, err := testResultsDirectory(t)
	if err != nil {
		t.Error(err)
	}

	lg.Info("start evaluating operations", zap.Int("num-of-operations", len(operations)))
	start := time.Now()
	linearizable, info := porcupine.CheckOperationsVerbose(model.Etcd, operations, time.Minute)
	if linearizable == porcupine.Illegal {
		t.Error("Model is not linearizable")
	}
	if linearizable == porcupine.Unknown {
		t.Error("Linearization timed out")
	}
	if linearizable != porcupine.Ok {
		persistOperationHistory(t, path, operations)
		persistMemberDataDir(t, clus, path)
	} else {
		lg.Info("operations is evaluated. Model is linearizable", zap.Duration("took", time.Since(start)))
	}

	visualizationPath := filepath.Join(path, "history.html")
	t.Logf("saving visualization to %q", visualizationPath)
	err = porcupine.VisualizePath(model.Etcd, info, visualizationPath)
	if err != nil {
		t.Errorf("Failed to visualize, err: %v", err)
	}
}

func persistOperationHistory(t *testing.T, path string, operations []porcupine.Operation) {
	historyFilePath := filepath.Join(path, "history.json")
	t.Logf("saving operation history to %q", historyFilePath)
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

func persistMemberDataDir(t *testing.T, clus *e2e.EtcdProcessCluster, path string) {
	for _, member := range clus.Procs {
		memberDataDir := filepath.Join(path, member.Config().Name)
		// TODO it conflicts with embed.Etcd Close verification tries to open db file which caused panic
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
