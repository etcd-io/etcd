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

package traffic

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

var (
	DefaultLeaseTTL         int64 = 7200
	RequestTimeout                = 200 * time.Millisecond
	WatchTimeout                  = time.Second
	MultiOpTxnOpCount             = 4
	DefaultCompactionPeriod       = 200 * time.Millisecond

	LowTraffic = Profile{
		MinimalQPS:                     100,
		MaximalQPS:                     200,
		BurstableQPS:                   1000,
		ClientCount:                    8,
		MaxNonUniqueRequestConcurrency: 3,
	}
	HighTrafficProfile = Profile{
		MinimalQPS:                     100,
		MaximalQPS:                     1000,
		BurstableQPS:                   1000,
		ClientCount:                    8,
		MaxNonUniqueRequestConcurrency: 3,
	}
)

func SimulateTraffic(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, profile Profile, traffic Traffic, failpointInjected <-chan report.FailpointInjection, baseTime time.Time, ids identity.Provider) []report.ClientReport {
	mux := sync.Mutex{}
	endpoints := clus.EndpointsGRPC()

	lm := identity.NewLeaseIDStorage()
	reports := []report.ClientReport{}
	// Use the highest MaximalQPS of all traffic profiles as burst otherwise actual traffic may be accidentally limited
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), profile.BurstableQPS)

	cc, err := client.NewRecordingClient(endpoints, ids, baseTime)
	require.NoError(t, err)
	defer cc.Close()
	// Ensure that first operation succeeds
	_, err = cc.Put(ctx, "start", "true")
	require.NoErrorf(t, err, "First operation failed, validation requires first operation to succeed")
	wg := sync.WaitGroup{}
	nonUniqueWriteLimiter := NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)
	finish := make(chan struct{})
	lg.Info("Start traffic")
	startTime := time.Since(baseTime)
	for i := 0; i < profile.ClientCount; i++ {
		wg.Add(1)
		c, nerr := client.NewRecordingClient([]string{endpoints[i%len(endpoints)]}, ids, baseTime)
		require.NoError(t, nerr)
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()

			traffic.RunTrafficLoop(ctx, c, limiter, ids, lm, nonUniqueWriteLimiter, finish)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
		}(c)
	}
	if !profile.ForbidCompaction {
		wg.Add(1)
		c, nerr := client.NewRecordingClient(endpoints, ids, baseTime)
		if nerr != nil {
			t.Fatal(nerr)
		}
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()

			compactionPeriod := DefaultCompactionPeriod
			if profile.CompactPeriod != time.Duration(0) {
				compactionPeriod = profile.CompactPeriod
			}

			traffic.RunCompactLoop(ctx, c, compactionPeriod, finish)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
		}(c)
	}
	var fr *report.FailpointInjection
	select {
	case frp, ok := <-failpointInjected:
		require.Truef(t, ok, "Failed to collect failpoint report")
		fr = &frp
	case <-ctx.Done():
		t.Fatalf("Traffic finished before failure was injected: %s", ctx.Err())
	}
	close(finish)
	wg.Wait()
	lg.Info("Finished traffic")
	endTime := time.Since(baseTime)

	time.Sleep(time.Second)
	// Ensure that last operation succeeds
	_, err = cc.Put(ctx, "tombstone", "true")
	require.NoErrorf(t, err, "Last operation failed, validation requires last operation to succeed")
	reports = append(reports, cc.Report())

	totalStats := calculateStats(reports, startTime, endTime)
	beforeFailpointStats := calculateStats(reports, startTime, fr.Start)
	duringFailpointStats := calculateStats(reports, fr.Start, fr.End)
	afterFailpointStats := calculateStats(reports, fr.End, endTime)

	lg.Info("Reporting complete traffic", zap.Int("successes", totalStats.successes), zap.Int("failures", totalStats.failures), zap.Float64("successRate", totalStats.successRate()), zap.Duration("period", totalStats.period), zap.Float64("qps", totalStats.QPS()))
	lg.Info("Reporting traffic before failure injection", zap.Int("successes", beforeFailpointStats.successes), zap.Int("failures", beforeFailpointStats.failures), zap.Float64("successRate", beforeFailpointStats.successRate()), zap.Duration("period", beforeFailpointStats.period), zap.Float64("qps", beforeFailpointStats.QPS()))
	lg.Info("Reporting traffic during failure injection", zap.Int("successes", duringFailpointStats.successes), zap.Int("failures", duringFailpointStats.failures), zap.Float64("successRate", duringFailpointStats.successRate()), zap.Duration("period", duringFailpointStats.period), zap.Float64("qps", duringFailpointStats.QPS()))
	lg.Info("Reporting traffic after failure injection", zap.Int("successes", afterFailpointStats.successes), zap.Int("failures", afterFailpointStats.failures), zap.Float64("successRate", afterFailpointStats.successRate()), zap.Duration("period", afterFailpointStats.period), zap.Float64("qps", afterFailpointStats.QPS()))

	if beforeFailpointStats.QPS() < profile.MinimalQPS {
		t.Errorf("Requiring minimal %f qps before failpoint injection for test results to be reliable, got %f qps", profile.MinimalQPS, beforeFailpointStats.QPS())
	}
	// TODO: Validate QPS post failpoint injection to ensure the that we sufficiently cover period when cluster recovers.
	return reports
}

func calculateStats(reports []report.ClientReport, start, end time.Duration) (ts trafficStats) {
	ts.period = end - start

	for _, r := range reports {
		for _, op := range r.KeyValue {
			if op.Call < start.Nanoseconds() || op.Call > end.Nanoseconds() {
				continue
			}
			resp := op.Output.(model.MaybeEtcdResponse)
			if resp.Error == "" {
				ts.successes++
			} else {
				ts.failures++
			}
		}
	}
	return ts
}

type trafficStats struct {
	successes, failures int
	period              time.Duration
}

func (ts *trafficStats) successRate() float64 {
	return float64(ts.successes) / float64(ts.successes+ts.failures)
}

func (ts *trafficStats) QPS() float64 {
	return float64(ts.successes) / ts.period.Seconds()
}

type Profile struct {
	MinimalQPS                     float64
	MaximalQPS                     float64
	BurstableQPS                   int
	MaxNonUniqueRequestConcurrency int
	ClientCount                    int
	ForbidCompaction               bool
	CompactPeriod                  time.Duration
}

func (p Profile) WithoutCompaction() Profile {
	p.ForbidCompaction = true
	return p
}

func (p Profile) WithCompactionPeriod(cp time.Duration) Profile {
	p.CompactPeriod = cp
	return p
}

type Traffic interface {
	RunTrafficLoop(ctx context.Context, c *client.RecordingClient, qpsLimiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIDStorage, nonUniqueWriteLimiter ConcurrencyLimiter, finish <-chan struct{})
	RunCompactLoop(ctx context.Context, c *client.RecordingClient, period time.Duration, finish <-chan struct{})
	ExpectUniqueRevision() bool
}
