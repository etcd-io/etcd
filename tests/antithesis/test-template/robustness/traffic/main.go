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
	"math/rand/v2"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/antithesis/test-template/robustness/common"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	robustnessrand "go.etcd.io/etcd/tests/v3/robustness/random"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

var (
	profile = traffic.Profile{
		MinimalQPS:                     100,
		MaximalQPS:                     1000,
		BurstableQPS:                   1000,
		ClientCount:                    3,
		MaxNonUniqueRequestConcurrency: 3,
	}
	trafficNames = []string{
		"etcd",
		"kubernetes",
	}
	traffics = []traffic.Traffic{
		traffic.EtcdPutDeleteLease,
		traffic.Kubernetes,
	}
)

func main() {
	local := flag.Bool("local", false, "run tests locally and connect to etcd instances via localhost")
	flag.Parse()

	reportPath, hosts, etcdetcdDataPaths := common.DefaultPaths()
	if *local {
		reportPath, hosts, etcdetcdDataPaths = common.LocalPaths()
	}

	ctx := context.Background()
	baseTime := time.Now()
	duration := time.Duration(robustnessrand.RandRange(5, 15) * int64(time.Second))

	lg, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	choice := rand.IntN(len(traffics))
	tf := traffics[choice]
	lg.Info("Traffic", zap.String("Type", trafficNames[choice]))
	r := report.TestReport{Logger: lg, ServersDataPath: etcdetcdDataPaths, Traffic: &report.TrafficDetail{ExpectUniqueRevision: tf.ExpectUniqueRevision()}}
	defer func() {
		if err = r.Report(reportPath); err != nil {
			lg.Error("Failed to save traffic generation report", zap.Error(err))
		}
	}()

	lg.Info("Start traffic generation", zap.Duration("duration", duration))
	r.Client, err = runTraffic(ctx, lg, tf, hosts, baseTime, duration)
	if err != nil {
		lg.Error("Failed to generate traffic")
		panic(err)
	}
}

func runTraffic(ctx context.Context, lg *zap.Logger, tf traffic.Traffic, hosts []string, baseTime time.Time, duration time.Duration) ([]report.ClientReport, error) {
	ids := identity.NewIDProvider()
	r, err := traffic.CheckEmptyDatabaseAtStart(ctx, lg, hosts, ids, baseTime)
	if err != nil {
		lg.Fatal("Failed empty database at start check", zap.Error(err))
	}
	trafficReports := []report.ClientReport{r}
	watchReport := []report.ClientReport{}
	maxRevisionChan := make(chan int64, 1)
	watchConfig := client.WatchConfig{
		RequestProgress: true,
	}
	g := errgroup.Group{}
	startTime := time.Since(baseTime)
	g.Go(func() error {
		defer close(maxRevisionChan)
		trafficReports = slices.Concat(trafficReports, simulateTraffic(ctx, tf, hosts, ids, baseTime, duration))
		maxRevision := report.OperationsMaxRevision(trafficReports)
		maxRevisionChan <- maxRevision
		lg.Info("Finished simulating Traffic", zap.Int64("max-revision", maxRevision))
		return nil
	})
	g.Go(func() error {
		var err error
		watchReport, err = client.CollectClusterWatchEvents(ctx, lg, hosts, maxRevisionChan, watchConfig, baseTime, ids)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	endTime := time.Since(baseTime)
	reports := slices.Concat(trafficReports, watchReport)
	totalStats := traffic.CalculateStats(reports, startTime, endTime)
	lg.Info("Completed traffic generation",
		zap.Int("successes", totalStats.Successes),
		zap.Int("failures", totalStats.Failures),
		zap.Float64("successRate", totalStats.SuccessRate()),
		zap.Duration("period", totalStats.Period),
		zap.Float64("qps", totalStats.QPS()),
	)
	return reports, nil
}

func simulateTraffic(ctx context.Context, tf traffic.Traffic, hosts []string, ids identity.Provider, baseTime time.Time, duration time.Duration) []report.ClientReport {
	var mux sync.Mutex
	var wg sync.WaitGroup
	storage := identity.NewLeaseIDStorage()
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), profile.BurstableQPS)
	concurrencyLimiter := traffic.NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)
	finish := closeAfter(ctx, duration)
	reports := []report.ClientReport{}
	keyStore := traffic.NewKeyStore(10, "key")
	for i := 0; i < profile.ClientCount; i++ {
		c := connect([]string{hosts[i%len(hosts)]}, ids, baseTime)
		wg.Add(1)
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()

			tf.RunTrafficLoop(ctx, c, limiter,
				ids,
				storage,
				concurrencyLimiter,
				keyStore,
				finish,
			)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
		}(c)
	}
	wg.Add(1)
	compactClient := connect(hosts, ids, baseTime)
	go func(c *client.RecordingClient) {
		defer wg.Done()
		defer c.Close()
		tf.RunCompactLoop(ctx, c, traffic.DefaultCompactionPeriod, finish)
		mux.Lock()
		reports = append(reports, c.Report())
		mux.Unlock()
	}(compactClient)
	defragPeriod := traffic.DefaultCompactionPeriod * time.Duration(len(hosts))
	for _, h := range hosts {
		c := connect([]string{h}, ids, baseTime)
		wg.Add(1)
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()
			runDefragLoop(ctx, c, defragPeriod, finish)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
		}(c)
	}
	wg.Wait()
	return reports
}

func runDefragLoop(ctx context.Context, c *client.RecordingClient, period time.Duration, finish <-chan struct{}) {
	jittered := time.Duration(robustnessrand.RandRange(int64(period-period/2), int64(period+period/2)))
	ticker := time.NewTicker(jittered)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-finish:
			return
		case <-ticker.C:
		}
		dctx, cancel := context.WithTimeout(ctx, traffic.RequestTimeout)
		_, err := c.Defragment(dctx)
		cancel()
		if err != nil {
			continue
		}
	}
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
