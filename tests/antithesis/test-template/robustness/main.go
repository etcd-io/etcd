// Copyright 2025 The etcd Authors
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

//go:build cgo && amd64

package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	robustnessrand "go.etcd.io/etcd/tests/v3/robustness/random"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/scenarios"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
	"go.etcd.io/etcd/tests/v3/robustness/validate"
)

var profile = traffic.Profile{
	MinimalQPS:                     100,
	MaximalQPS:                     1000,
	BurstableQPS:                   1000,
	ClientCount:                    3,
	MaxNonUniqueRequestConcurrency: 3,
}

func main() {
	ctx := context.Background()
	baseTime := time.Now()
	duration := time.Duration(robustnessrand.RandRange(5, 60) * int64(time.Second))
	testRobustness(ctx, baseTime, duration)
}

func testRobustness(ctx context.Context, baseTime time.Time, duration time.Duration) {
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), profile.BurstableQPS)
	finish := closeAfter(ctx, duration)
	ids := identity.NewIDProvider()
	storage := identity.NewLeaseIDStorage()
	concurrencyLimiter := traffic.NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)
	hosts := []string{"etcd0:2379", "etcd1:2379", "etcd2:2379"}

	reports := []report.ClientReport{}
	var mux sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < profile.ClientCount; i++ {
		c := connect(hosts[i%len(hosts)], ids, baseTime)
		wg.Add(1)
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()

			traffic.EtcdAntithesis.RunTrafficLoop(ctx, c, limiter,
				ids,
				storage,
				concurrencyLimiter,
				finish,
			)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
		}(c)
	}
	wg.Wait()
	fmt.Println("Completed robustness traffic generation")
	assert.Reachable("Completed robustness traffic generation", nil)
}

func validateReport(ctx context.Context, lg *zap.Logger, c *e2e.EtcdProcessCluster, s scenarios.TestScenario, t *testing.T) {
	r := report.TestReport{Logger: lg, Cluster: c}
	persistedRequests, err := report.PersistedRequestsCluster(lg, c)
	require.NoError(t, err)
	validateConfig := validate.Config{ExpectRevisionUnique: s.Traffic.ExpectUniqueRevision()}
	r.Visualize = validate.ValidateAndReturnVisualize(t, lg, validateConfig, r.Client, persistedRequests, 5*time.Minute).Visualize
}

func connect(endpoint string, ids identity.Provider, baseTime time.Time) *client.RecordingClient {
	cli, err := client.NewRecordingClient([]string{endpoint}, ids, baseTime)
	if err != nil {
		// Antithesis Assertion: client should always be able to connect to an etcd host
		assert.Unreachable("Client failed to connect to an etcd host", map[string]any{"host": endpoint, "error": err})
		os.Exit(1)
	}
	return cli
}

func closeAfter(ctx context.Context, t time.Duration) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		for {
			select {
			case <-time.After(t):
			case <-ctx.Done():
			}
			close(out)
		}
	}()
	return out
}
