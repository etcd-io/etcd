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
	"flag"
	"os"
	"sync"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	robustnessrand "go.etcd.io/etcd/tests/v3/robustness/random"
	"go.etcd.io/etcd/tests/v3/robustness/report"
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

const (
	defaultetcd0 = "etcd0:2379"
	defaultetcd1 = "etcd1:2379"
	defaultetcd2 = "etcd0:2379"

	localetcd0 = "127.0.0.1:12379"
	localetcd1 = "127.0.0.1:22379"
	localetcd2 = "127.0.0.1:32379"
)

func main() {
	local := flag.Bool("local", false, "run tests locally and connect to etcd instances via localhost")
	flag.Parse()
	hosts := []string{defaultetcd0, defaultetcd1, defaultetcd2}
	if *local {
		hosts = []string{localetcd0, localetcd1, localetcd2}
	}

	ctx := context.Background()
	baseTime := time.Now()
	duration := time.Duration(robustnessrand.RandRange(5, 60) * int64(time.Second))
	testRobustness(ctx, hosts, baseTime, duration)
}

func testRobustness(ctx context.Context, hosts []string, baseTime time.Time, duration time.Duration) {
	lg, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	lg.Info("Start traffic generation")
	reports := runTraffic(ctx, lg, hosts, baseTime, duration)
	lg.Info("Completed traffic generation")

	validateConfig := validate.Config{ExpectRevisionUnique: traffic.EtcdAntithesis.ExpectUniqueRevision()}
	result := validate.ValidateAndReturnVisualize(lg, validateConfig, reports, nil, 5*time.Minute)
	err = result.Linearization.Visualize(lg, "history.html")
	if err != nil {
		lg.Error("Failed to save visualization", zap.Error(result.Error))
	}
	if result.Error != nil {
		lg.Error("Robustness validation failed", zap.Error(result.Error))
		assert.Unreachable("Robustness validation failed", map[string]any{"error": result.Error})
		return
	}
	lg.Info("Completed robustness validation")
	assert.Reachable("Completed robustness validation", nil)
}

func runTraffic(ctx context.Context, lg *zap.Logger, hosts []string, baseTime time.Time, duration time.Duration) []report.ClientReport {
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), profile.BurstableQPS)
	finish := closeAfter(ctx, duration)
	ids := identity.NewIDProvider()
	storage := identity.NewLeaseIDStorage()
	concurrencyLimiter := traffic.NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)

	r, err := traffic.CheckEmptyDatabaseAtStart(ctx, lg, hosts, ids, baseTime)
	if err != nil {
		lg.Fatal("Failed empty database at start check", zap.Error(err))
	}

	reports := []report.ClientReport{r}
	var mux sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < profile.ClientCount; i++ {
		c := connect([]string{hosts[i%len(hosts)]}, ids, baseTime)
		defer c.Close()
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
	return reports
}

func connect(endpoints []string, ids identity.Provider, baseTime time.Time) *client.RecordingClient {
	cli, err := client.NewRecordingClient(endpoints, ids, baseTime)
	if err != nil {
		// Antithesis Assertion: client should always be able to connect to an etcd host
		assert.Unreachable("Client failed to connect to an etcd host", map[string]any{"endpoints": endpoints, "error": err})
		os.Exit(1)
	}
	return cli
}

func closeAfter(ctx context.Context, t time.Duration) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		select {
		case <-time.After(t):
		case <-ctx.Done():
		}
		close(out)
	}()
	return out
}
