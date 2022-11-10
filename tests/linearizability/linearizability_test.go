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
	// failpointTriggersCount
	failpointTriggersCount = 60
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
			name:      "ClusterOfSize3",
			failpoint: RandomFailpoint,
			config: e2e.EtcdProcessClusterConfig{
				ClusterSize:   3,
				GoFailEnabled: true,
				DataDirPath:   "/Users/wachao/tmp/etcd/test/etcd",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			failpoint := FailpointConfig{
				failpoint:           tc.failpoint,
				count:               10,
				waitBetweenTriggers: waitBetweenFailpointTriggers,
			}
			traffic := trafficConfig{
				minimalQPS:  minimalQPS,
				maximalQPS:  maximalQPS,
				clientCount: 10,
				traffic:     PutGetTraffic,
			}
			testLinearizability(context.Background(), t, tc.config, failpoint, traffic)
		})
	}
}

func testLinearizability(ctx context.Context, t *testing.T, config e2e.EtcdProcessClusterConfig, failpoint FailpointConfig, traffic trafficConfig) {
	clus, err := e2e.NewEtcdProcessCluster(ctx, t, &config)
	if err != nil {
		t.Fatal(err)
	}
	defer clus.Stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		err := triggerFailpoints(ctx, t, clus, failpoint)
		if err != nil {
			t.Error(err)
		}
	}()
	operations := simulateTraffic(ctx, t, clus, traffic)
	time.Sleep(10 * time.Second)

	linearizable, info := porcupine.CheckOperationsVerbose(etcdModel, operations, 0)
	if linearizable != porcupine.Ok {
		t.Error("###### Model is not linearizable")
	}

	path, err := filepath.Abs(filepath.Join("/Users/wachao/tmp/etcd/test/", strings.Replace(t.Name(), "/", "_", -1)+".html"))
	if err != nil {
		t.Errorf("###### failed to call filepath.Abs: %v", err)
	}
	err = porcupine.VisualizePath(etcdModel, info, path)
	if err != nil {
		t.Errorf("###### Failed to visualize, err: %v", err)
	}
	t.Logf("saving visualization to %q", path)
}

func triggerFailpoints(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, config FailpointConfig) error {
	var err error
	successes := 0
	failures := 0
	time.Sleep(config.waitBetweenTriggers)
	t.Logf("###### config.count: %d", config.count)
	for successes+failures < config.count {
		t.Logf("###### config.count: %d, successes: %d, failures: %d", config.count, successes, failures)
		err = config.failpoint.Trigger(ctx, clus)
		if err != nil {
			t.Logf("Failed to trigger failpoint %q, err: %v\n", config.failpoint.Name(), err)
			failures++
			continue
		}
		successes++
		time.Sleep(config.waitBetweenTriggers)
	}
	if successes < config.count || failures >= config.count {
		//return fmt.Errorf("failed to trigger failpoints enough times, err: %v", err)
	}
	return nil
}

type FailpointConfig struct {
	failpoint           Failpoint
	count               int
	waitBetweenTriggers time.Duration
}

func simulateTraffic(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, config trafficConfig) (operations []porcupine.Operation) {
	mux := sync.Mutex{}
	endpoints := clus.EndpointsV3()

	limiter := rate.NewLimiter(rate.Limit(config.maximalQPS), 200)

	startTime := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < config.clientCount; i++ {
		wg.Add(1)
		endpoints := []string{endpoints[i%len(endpoints)]}
		c, err := NewClient(endpoints, i)
		if err != nil {
			t.Fatal(err)
		}
		go func(c *recordingClient) {
			defer wg.Done()
			defer c.Close()

			config.traffic.Run(ctx, c, limiter)
			mux.Lock()
			operations = append(operations, c.operations...)
			mux.Unlock()
		}(c)
	}
	wg.Wait()
	endTime := time.Now()
	t.Logf("Recorded %d operations", len(operations))

	qps := float64(len(operations)) / float64(endTime.Sub(startTime)) * float64(time.Second)
	t.Logf("Average traffic: %f qps", qps)
	if qps < config.minimalQPS {
		t.Logf("Requiring minimal %f qps for test results to be reliable, got %f qps", config.minimalQPS, qps)
	}
	return operations
}

type trafficConfig struct {
	minimalQPS  float64
	maximalQPS  float64
	clientCount int
	traffic     Traffic
}
