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

	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/robustness/report"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
)

var (
	DefaultLeaseTTL   int64 = 7200
	RequestTimeout          = 40 * time.Millisecond
	WatchTimeout            = 400 * time.Millisecond
	MultiOpTxnOpCount       = 4

	LowTraffic = Profile{
		Name:                           "LowTraffic",
		MinimalQPS:                     100,
		MaximalQPS:                     200,
		ClientCount:                    8,
		MaxNonUniqueRequestConcurrency: 3,
	}
	HighTrafficProfile = Profile{
		Name:                           "HighTraffic",
		MinimalQPS:                     200,
		MaximalQPS:                     1000,
		ClientCount:                    12,
		MaxNonUniqueRequestConcurrency: 3,
	}
)

func SimulateTraffic(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, profile Profile, traffic Traffic, finish <-chan struct{}, baseTime time.Time, ids identity.Provider) []report.ClientReport {
	mux := sync.Mutex{}
	endpoints := clus.EndpointsGRPC()

	lm := identity.NewLeaseIdStorage()
	reports := []report.ClientReport{}
	limiter := rate.NewLimiter(rate.Limit(profile.MaximalQPS), 200)

	startTime := time.Now()
	cc, err := NewClient(endpoints, ids, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	wg := sync.WaitGroup{}
	nonUniqueWriteLimiter := NewConcurrencyLimiter(profile.MaxNonUniqueRequestConcurrency)
	for i := 0; i < profile.ClientCount; i++ {
		wg.Add(1)
		c, err := NewClient([]string{endpoints[i%len(endpoints)]}, ids, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		go func(c *RecordingClient) {
			defer wg.Done()
			defer c.Close()

			traffic.Run(ctx, c, limiter, ids, lm, nonUniqueWriteLimiter, finish)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
		}(c)
	}
	wg.Wait()
	endTime := time.Now()

	// Ensure that last operation is succeeds
	time.Sleep(time.Second)
	_, err = cc.Put(ctx, "tombstone", "true")
	if err != nil {
		t.Error(err)
	}
	reports = append(reports, cc.Report())

	var operationCount int
	for _, r := range reports {
		operationCount += len(r.KeyValue)
	}
	lg.Info("Recorded operations", zap.Int("operationCount", operationCount))

	qps := float64(operationCount) / float64(endTime.Sub(startTime)) * float64(time.Second)
	lg.Info("Average traffic", zap.Float64("qps", qps))
	if qps < profile.MinimalQPS {
		t.Errorf("Requiring minimal %f qps for test results to be reliable, got %f qps", profile.MinimalQPS, qps)
	}
	return reports
}

type Profile struct {
	Name                           string
	MinimalQPS                     float64
	MaximalQPS                     float64
	MaxNonUniqueRequestConcurrency int
	ClientCount                    int
}

type Traffic interface {
	Run(ctx context.Context, c *RecordingClient, qpsLimiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage, nonUniqueWriteLimiter ConcurrencyLimiter, finish <-chan struct{})
	ExpectUniqueRevision() bool
	Name() string
}
