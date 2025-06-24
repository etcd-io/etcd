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
	"go.etcd.io/etcd/tests/v3/robustness/validate"
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
		MemberClientCount:              6,
		ClusterClientCount:             2,
		MaxNonUniqueRequestConcurrency: 3,
	}
	HighTrafficProfile = Profile{
		MinimalQPS:                     100,
		MaximalQPS:                     1000,
		BurstableQPS:                   1000,
		MemberClientCount:              6,
		ClusterClientCount:             2,
		MaxNonUniqueRequestConcurrency: 3,
	}
)

func SimulateTraffic(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, profile Profile, traffic Traffic, failpointInjected <-chan report.FailpointInjection, clientSet *client.ClientSet) []report.ClientReport {
	endpoints := clus.EndpointsGRPC()

	lm := identity.NewLeaseIDStorage()
	// Use the highest MaximalQPS of all traffic profiles as burst otherwise actual traffic may be accidentally limited
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), profile.BurstableQPS)

	err := CheckEmptyDatabaseAtStart(ctx, lg, endpoints, clientSet)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	nonUniqueWriteLimiter := NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)
	finish := make(chan struct{})

	keyStore := NewKeyStore(10, "key")

	lg.Info("Start traffic")
	startTime := time.Since(clientSet.BaseTime())
	for i := range profile.MemberClientCount {
		wg.Add(1)

		c, nerr := clientSet.NewClient([]string{endpoints[i%len(endpoints)]})
		require.NoError(t, nerr)
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()

			traffic.RunTrafficLoop(ctx, c, limiter, clientSet.IdentityProvider(), lm, nonUniqueWriteLimiter, keyStore, finish)
		}(c)
	}
	for range profile.ClusterClientCount {
		wg.Add(1)

		c, nerr := clientSet.NewClient(endpoints)
		require.NoError(t, nerr)
		go func(c *client.RecordingClient) {
			defer wg.Done()
			defer c.Close()

			traffic.RunTrafficLoop(ctx, c, limiter, clientSet.IdentityProvider(), lm, nonUniqueWriteLimiter, keyStore, finish)
		}(c)
	}
	if !profile.ForbidCompaction {
		wg.Add(1)
		c, nerr := clientSet.NewClient(endpoints)
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
	endTime := time.Since(clientSet.BaseTime())

	time.Sleep(time.Second)
	// Ensure that last operation succeeds
	cc, err := clientSet.NewClient(endpoints)
	require.NoError(t, err)
	defer cc.Close()
	_, err = cc.Put(ctx, "tombstone", "true")
	require.NoErrorf(t, err, "Last operation failed, validation requires last operation to succeed")
	reports := clientSet.Reports()

	totalStats := CalculateStats(reports, startTime, endTime)
	beforeFailpointStats := CalculateStats(reports, startTime, fr.Start)
	duringFailpointStats := CalculateStats(reports, fr.Start, fr.End)
	afterFailpointStats := CalculateStats(reports, fr.End, endTime)

	lg.Info("Reporting complete traffic", zap.Int("successes", totalStats.Successes), zap.Int("failures", totalStats.Failures), zap.Float64("successRate", totalStats.SuccessRate()), zap.Duration("period", totalStats.Period), zap.Float64("qps", totalStats.QPS()))
	lg.Info("Reporting traffic before failure injection", zap.Int("successes", beforeFailpointStats.Successes), zap.Int("failures", beforeFailpointStats.Failures), zap.Float64("successRate", beforeFailpointStats.SuccessRate()), zap.Duration("period", beforeFailpointStats.Period), zap.Float64("qps", beforeFailpointStats.QPS()))
	lg.Info("Reporting traffic during failure injection", zap.Int("successes", duringFailpointStats.Successes), zap.Int("failures", duringFailpointStats.Failures), zap.Float64("successRate", duringFailpointStats.SuccessRate()), zap.Duration("period", duringFailpointStats.Period), zap.Float64("qps", duringFailpointStats.QPS()))
	lg.Info("Reporting traffic after failure injection", zap.Int("successes", afterFailpointStats.Successes), zap.Int("failures", afterFailpointStats.Failures), zap.Float64("successRate", afterFailpointStats.SuccessRate()), zap.Duration("period", afterFailpointStats.Period), zap.Float64("qps", afterFailpointStats.QPS()))

	if beforeFailpointStats.QPS() < profile.MinimalQPS {
		t.Errorf("Requiring minimal %f qps before failpoint injection for test results to be reliable, got %f qps", profile.MinimalQPS, beforeFailpointStats.QPS())
	}
	// TODO: Validate QPS post failpoint injection to ensure the that we sufficiently cover period when cluster recovers.
	return reports
}

func CalculateStats(reports []report.ClientReport, start, end time.Duration) (ts trafficStats) {
	ts.Period = end - start

	for _, r := range reports {
		for _, op := range r.KeyValue {
			if op.Call < start.Nanoseconds() || op.Call > end.Nanoseconds() {
				continue
			}
			resp := op.Output.(model.MaybeEtcdResponse)
			if resp.Error == "" {
				ts.Successes++
			} else {
				ts.Failures++
			}
		}
	}
	return ts
}

type trafficStats struct {
	Successes, Failures int
	Period              time.Duration
}

func (ts *trafficStats) SuccessRate() float64 {
	return float64(ts.Successes) / float64(ts.Successes+ts.Failures)
}

func (ts *trafficStats) QPS() float64 {
	return float64(ts.Successes) / ts.Period.Seconds()
}

type Profile struct {
	MinimalQPS                     float64
	MaximalQPS                     float64
	BurstableQPS                   int
	MaxNonUniqueRequestConcurrency int
	MemberClientCount              int
	ClusterClientCount             int
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
	RunTrafficLoop(ctx context.Context, c *client.RecordingClient, qpsLimiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIDStorage, nonUniqueWriteLimiter ConcurrencyLimiter, keyStore *keyStore, finish <-chan struct{})
	RunCompactLoop(ctx context.Context, c *client.RecordingClient, period time.Duration, finish <-chan struct{})
	ExpectUniqueRevision() bool
}

func CheckEmptyDatabaseAtStart(ctx context.Context, lg *zap.Logger, endpoints []string, cs *client.ClientSet) error {
	c, err := cs.NewClient(endpoints)
	if err != nil {
		return err
	}
	defer c.Close()
	for {
		rCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
		resp, err := c.Get(rCtx, "key")
		cancel()
		if err != nil {
			lg.Warn("Failed to check if database empty at start, retrying", zap.Error(err))
			continue
		}
		if resp.Header.Revision != 1 {
			return validate.ErrNotEmptyDatabase
		}
		break
	}
	return nil
}
