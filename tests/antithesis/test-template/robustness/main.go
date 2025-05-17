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
	"fmt"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/anishathalye/porcupine"
	"github.com/antithesishq/antithesis-sdk-go/assert"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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
	defaultetcd2 = "etcd2:2379"

	localetcd0 = "127.0.0.1:12379"
	localetcd1 = "127.0.0.1:22379"
	localetcd2 = "127.0.0.1:32379"
	// mounted by the client in docker compose
	defaultEtcdDataPath = "/var/etcddata%d"
	// used by default when running the client locally
	defaultLocalEtcdDataPath = "/tmp/etcddata%d"
	localEtcdDataPathEnv     = "ETCD_ROBUSTNESS_DATA_PATH"

	defaultReportPath = "/var/report/"
	localReportPath   = "report"
)

func main() {
	local := flag.Bool("local", false, "run tests locally and connect to etcd instances via localhost")
	flag.Parse()

	etcdDataPath := defaultEtcdDataPath
	hosts := []string{defaultetcd0, defaultetcd1, defaultetcd2}
	reportPath := defaultReportPath
	if *local {
		hosts = []string{localetcd0, localetcd1, localetcd2}
		etcdDataPath = defaultLocalEtcdDataPath
		localpath := os.Getenv(localEtcdDataPathEnv)
		if localpath != "" {
			etcdDataPath = localpath
		}
		reportPath = localReportPath
	}

	persistedRequestdirs := []string{
		fmt.Sprintf(etcdDataPath, 0),
		fmt.Sprintf(etcdDataPath, 1),
		fmt.Sprintf(etcdDataPath, 2),
	}

	ctx := context.Background()
	baseTime := time.Now()
	duration := time.Duration(robustnessrand.RandRange(5, 15) * int64(time.Second))

	lg, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	r := report.TestReport{Logger: lg, ServersDataPath: map[string]string{}}
	for i := range len(persistedRequestdirs) {
		r.ServersDataPath[fmt.Sprintf("etcd%d", i)] = persistedRequestdirs[i]
	}
	defer func() {
		if err := r.Report(reportPath); err != nil {
			lg.Error("Failed to save validation report", zap.Error(err))
		}
	}()
	testRobustness(ctx, lg, &r, hosts, baseTime, duration)
}

func testRobustness(ctx context.Context, lg *zap.Logger, r *report.TestReport, hosts []string, baseTime time.Time, duration time.Duration) {
	lg.Info("Start traffic generation", zap.Duration("duration", duration))
	var err error
	r.Client, err = runTraffic(ctx, lg, hosts, baseTime, duration)
	if err != nil {
		lg.Error("Failed to generate traffic")
		panic(err)
	}
	lg.Info("Completed traffic generation")

	persistedRequests, err := report.PersistedRequests(lg, slices.Collect(maps.Values(r.ServersDataPath)))
	assert.Always(err == nil, "Loaded persisted requests", map[string]any{"error": err})
	validateConfig := validate.Config{ExpectRevisionUnique: traffic.EtcdAntithesis.ExpectUniqueRevision()}
	result := validate.ValidateAndReturnVisualize(lg, validateConfig, r.Client, persistedRequests, 5*time.Minute)
	assert.Always(result.Assumptions == nil, "Validation assumptions fulfilled", map[string]any{"error": result.Assumptions})
	if result.Linearization.Linearizable == porcupine.Unknown {
		assert.Unreachable("Linearization timeout", nil)
	} else {
		assert.Always(result.Linearization.Linearizable == porcupine.Ok, "Linearization validation passes", nil)
	}
	r.Visualize = result.Linearization.Visualize
	assert.Always(result.WatchError == nil, "Watch validation passes", map[string]any{"error": result.WatchError})
	assert.Always(result.SerializableError == nil, "Serializable validation passes", map[string]any{"error": result.SerializableError})
	lg.Info("Completed robustness validation")
}

func runTraffic(ctx context.Context, lg *zap.Logger, hosts []string, baseTime time.Time, duration time.Duration) ([]report.ClientReport, error) {
	ids := identity.NewIDProvider()
	r, err := traffic.CheckEmptyDatabaseAtStart(ctx, lg, hosts, ids, baseTime)
	if err != nil {
		lg.Fatal("Failed empty database at start check", zap.Error(err))
	}
	reports := []report.ClientReport{r}
	watchReport := []report.ClientReport{}
	maxRevisionChan := make(chan int64, 1)
	watchConfig := client.WatchConfig{
		RequestProgress: true,
	}
	g := errgroup.Group{}
	g.Go(func() error {
		defer close(maxRevisionChan)
		reports = slices.Concat(reports, simulateTraffic(ctx, hosts, ids, baseTime, duration))
		maxRevision := report.OperationsMaxRevision(reports)
		maxRevisionChan <- maxRevision
		lg.Info("Finished simulating Traffic", zap.Int64("max-revision", maxRevision))
		return nil
	})
	g.Go(func() error {
		var watchErr error
		watchReport, _, watchErr = client.CollectClusterWatchEvents(ctx, lg, hosts, maxRevisionChan, watchConfig, baseTime, ids)
		return watchErr
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return slices.Concat(reports, watchReport), nil
}

func simulateTraffic(ctx context.Context, hosts []string, ids identity.Provider, baseTime time.Time, duration time.Duration) []report.ClientReport {
	var mux sync.Mutex
	var wg sync.WaitGroup
	storage := identity.NewLeaseIDStorage()
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), profile.BurstableQPS)
	concurrencyLimiter := traffic.NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)
	finish := closeAfter(ctx, duration)
	reports := []report.ClientReport{}
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
