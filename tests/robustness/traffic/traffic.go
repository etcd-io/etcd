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

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
)

var (
	DefaultLeaseTTL   int64 = 7200
	RequestTimeout          = 40 * time.Millisecond
	WatchTimeout            = 400 * time.Millisecond
	MultiOpTxnOpCount       = 4
)

func SimulateTraffic(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, config Config, finish <-chan struct{}, baseTime time.Time, ids identity.Provider) []model.ClientReport {
	mux := sync.Mutex{}
	endpoints := clus.EndpointsGRPC()

	lm := identity.NewLeaseIdStorage()
	reports := []model.ClientReport{}
	limiter := rate.NewLimiter(rate.Limit(config.maximalQPS), 200)

	startTime := time.Now()
	cc, err := NewClient(endpoints, ids, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	wg := sync.WaitGroup{}
	for i := 0; i < config.ClientCount; i++ {
		wg.Add(1)
		c, err := NewClient([]string{endpoints[i%len(endpoints)]}, ids, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		go func(c *RecordingClient) {
			defer wg.Done()
			defer c.Close()

			config.Traffic.Run(ctx, c, limiter, ids, lm, finish)
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
		operationCount += r.KeyValue.Len()
	}
	lg.Info("Recorded operations", zap.Int("operationCount", operationCount))

	qps := float64(operationCount) / float64(endTime.Sub(startTime)) * float64(time.Second)
	lg.Info("Average traffic", zap.Float64("qps", qps))
	if qps < config.minimalQPS {
		t.Errorf("Requiring minimal %f qps for test results to be reliable, got %f qps", config.minimalQPS, qps)
	}
	return reports
}

type Config struct {
	Name        string
	minimalQPS  float64
	maximalQPS  float64
	ClientCount int
	Traffic     Traffic
}

type Traffic interface {
	Run(ctx context.Context, c TrafficClient, limiter Limiter, ids identity.Provider, lm identity.LeaseIdStorage, finish <-chan struct{})
	ExpectUniqueRevision() bool
}

type TrafficClient interface {
	ClientId() int
	Range(ctx context.Context, prefix string, end string, revision, limit int64) (*clientv3.GetResponse, error)
	Put(ctx context.Context, key string, value string) (*clientv3.PutResponse, error)
	Delete(ctx context.Context, key string) (*clientv3.DeleteResponse, error)
	Txn(ctx context.Context, conditions []clientv3.Cmp, onSuccess []clientv3.Op, onFailure []clientv3.Op) (*clientv3.TxnResponse, error)
	Get(ctx context.Context, key string, revision int64) (*mvccpb.KeyValue, int64, error)
	LeaseGrant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error)
	PutWithLease(ctx context.Context, key string, value string, revision int64) (*clientv3.PutResponse, error)
	LeaseRevoke(ctx context.Context, id int64) (*clientv3.LeaseRevokeResponse, error)
	Defragment(ctx context.Context) (*clientv3.DefragmentResponse, error)
	Watch(ctx context.Context, key string, rev int64, withPrefix bool, withProgressNotify bool) clientv3.WatchChan
}

type Limiter interface {
	Wait(ctx context.Context) error
}
