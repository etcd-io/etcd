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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"golang.org/x/time/rate"
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
				e2e.WithGoFailEnabled(true),
				e2e.WithCompactionBatchLimit(100), // required for compactBeforeCommitBatch and compactAfterCommitBatch failpoints
			),
		},
		{
			name:      "ClusterOfSize3",
			failpoint: RandomFailpoint,
			config: *e2e.NewConfig(
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
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			failpoint := FailpointConfig{
				failpoint:           tc.failpoint,
				count:               1,
				retries:             3,
				waitBetweenTriggers: waitBetweenFailpointTriggers,
			}
			traffic := trafficConfig{
				minimalQPS:  minimalQPS,
				maximalQPS:  maximalQPS,
				clientCount: 8,
				traffic:     DefaultTraffic,
			}
			testLinearizability(context.Background(), t, tc.config, failpoint, traffic)
		})
	}
}

func testLinearizability(ctx context.Context, t *testing.T, config e2e.EtcdProcessClusterConfig, failpoint FailpointConfig, traffic trafficConfig) {
	clus, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&config))
	if err != nil {
		t.Fatal(err)
	}
	defer clus.Close()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		err := triggerFailpoints(ctx, t, clus, failpoint)
		if err != nil {
			t.Error(err)
		}
	}()
	operations := simulateTraffic(ctx, t, clus, traffic)
	err = clus.Stop()
	if err != nil {
		t.Error(err)
	}
	checkOperationsAndPersistResults(t, operations, clus)
}

func triggerFailpoints(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, config FailpointConfig) error {
	var err error
	successes := 0
	failures := 0
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
		return fmt.Errorf("failed to trigger failpoints enough times, err: %v", err)
	}
	time.Sleep(config.waitBetweenTriggers)
	return nil
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

			config.traffic.Run(ctx, c, limiter, ids)
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
