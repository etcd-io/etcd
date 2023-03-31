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

package robustness

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

var (
	DefaultLeaseTTL   int64 = 7200
	RequestTimeout          = 40 * time.Millisecond
	MultiOpTxnOpCount       = 4
)

type TrafficRequestType string

const (
	Get           TrafficRequestType = "get"
	Put           TrafficRequestType = "put"
	LargePut      TrafficRequestType = "largePut"
	Delete        TrafficRequestType = "delete"
	MultiOpTxn    TrafficRequestType = "multiOpTxn"
	PutWithLease  TrafficRequestType = "putWithLease"
	LeaseRevoke   TrafficRequestType = "leaseRevoke"
	CompareAndSet TrafficRequestType = "compareAndSet"
	Defragment    TrafficRequestType = "defragment"
)

func SimulateTraffic(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, config TrafficConfig) []porcupine.Operation {
	mux := sync.Mutex{}
	endpoints := clus.EndpointsV3()

	ids := identity.NewIdProvider()
	lm := identity.NewLeaseIdStorage()
	h := model.History{}
	limiter := rate.NewLimiter(rate.Limit(config.MaximalQPS), 200)

	startTime := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < config.ClientCount; i++ {
		wg.Add(1)
		endpoints := []string{endpoints[i%len(endpoints)]}
		c, err := NewClient(endpoints, ids, startTime)
		if err != nil {
			t.Fatal(err)
		}
		go func(c *RecordingClient, clientId int) {
			defer wg.Done()
			defer c.Close()

			config.Traffic.Run(ctx, clientId, c, limiter, ids, lm)
			mux.Lock()
			h = h.Merge(c.History.History)
			mux.Unlock()
		}(c, i)
	}
	wg.Wait()
	endTime := time.Now()
	operations := h.Operations()
	lg.Info("Recorded operations", zap.Int("Count", len(operations)))

	qps := float64(len(operations)) / float64(endTime.Sub(startTime)) * float64(time.Second)
	lg.Info("Average Traffic", zap.Float64("qps", qps))
	if qps < config.MinimalQPS {
		t.Errorf("Requiring minimal %f qps for test results to be reliable, got %f qps", config.MinimalQPS, qps)
	}
	return operations
}

type TrafficConfig struct {
	Name        string
	MinimalQPS  float64
	MaximalQPS  float64
	ClientCount int
	Traffic     Traffic
}

type Traffic struct {
	KeyCount     int
	Writes       []RequestChance
	LeaseTTL     int64
	LargePutSize int
}

type RequestChance struct {
	Operation TrafficRequestType
	Chance    int
}

func (t Traffic) Run(ctx context.Context, clientId int, c *RecordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage) {

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		key := fmt.Sprintf("%d", rand.Int()%t.KeyCount)
		// Execute one read per one write to avoid operation history include too many failed Writes when etcd is down.
		resp, err := t.Read(ctx, c, limiter, key)
		if err != nil {
			continue
		}
		t.Write(ctx, c, limiter, key, ids, lm, clientId, resp)
	}
}

func (t Traffic) Read(ctx context.Context, c *RecordingClient, limiter *rate.Limiter, key string) ([]*mvccpb.KeyValue, error) {
	getCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	resp, err := c.Get(getCtx, key)
	cancel()
	if err == nil {
		limiter.Wait(ctx)
	}
	return resp, err
}

func (t Traffic) Write(ctx context.Context, c *RecordingClient, limiter *rate.Limiter, key string, id identity.Provider, lm identity.LeaseIdStorage, cid int, lastValues []*mvccpb.KeyValue) error {
	writeCtx, cancel := context.WithTimeout(ctx, RequestTimeout)

	var err error
	switch t.pickWriteRequest() {
	case Put:
		err = c.Put(writeCtx, key, fmt.Sprintf("%d", id.RequestId()))
	case LargePut:
		err = c.Put(writeCtx, key, randString(t.LargePutSize))
	case Delete:
		err = c.Delete(writeCtx, key)
	case MultiOpTxn:
		err = c.Txn(writeCtx, nil, t.pickMultiTxnOps(id))
	case CompareAndSet:
		var expectValue string
		if len(lastValues) != 0 {
			expectValue = string(lastValues[0].Value)
		}
		err = c.CompareAndSet(writeCtx, key, expectValue, fmt.Sprintf("%d", id.RequestId()))
	case PutWithLease:
		leaseId := lm.LeaseId(cid)
		if leaseId == 0 {
			leaseId, err = c.LeaseGrant(writeCtx, t.LeaseTTL)
			if err == nil {
				lm.AddLeaseId(cid, leaseId)
				limiter.Wait(ctx)
			}
		}
		if leaseId != 0 {
			putCtx, putCancel := context.WithTimeout(ctx, RequestTimeout)
			err = c.PutWithLease(putCtx, key, fmt.Sprintf("%d", id.RequestId()), leaseId)
			putCancel()
		}
	case LeaseRevoke:
		leaseId := lm.LeaseId(cid)
		if leaseId != 0 {
			err = c.LeaseRevoke(writeCtx, leaseId)
			//if LeaseRevoke has failed, do not remove the mapping.
			if err == nil {
				lm.RemoveLeaseId(cid)
			}
		}
	case Defragment:
		err = c.Defragment(writeCtx)
	default:
		panic("invalid operation")
	}
	cancel()
	if err == nil {
		limiter.Wait(ctx)
	}
	return err
}

func (t Traffic) pickWriteRequest() TrafficRequestType {
	sum := 0
	for _, op := range t.Writes {
		sum += op.Chance
	}
	roll := rand.Int() % sum
	for _, op := range t.Writes {
		if roll < op.Chance {
			return op.Operation
		}
		roll -= op.Chance
	}
	panic("unexpected")
}

func (t Traffic) pickMultiTxnOps(ids identity.Provider) (ops []clientv3.Op) {
	keys := rand.Perm(t.KeyCount)
	opTypes := make([]model.OperationType, 4)

	atLeastOnePut := false
	for i := 0; i < MultiOpTxnOpCount; i++ {
		opTypes[i] = t.pickOperationType()
		if opTypes[i] == model.Put {
			atLeastOnePut = true
		}
	}
	// Ensure at least one put to make operation unique
	if !atLeastOnePut {
		opTypes[0] = model.Put
	}

	for i, opType := range opTypes {
		key := fmt.Sprintf("%d", keys[i])
		switch opType {
		case model.Get:
			ops = append(ops, clientv3.OpGet(key))
		case model.Put:
			value := fmt.Sprintf("%d", ids.RequestId())
			ops = append(ops, clientv3.OpPut(key, value))
		case model.Delete:
			ops = append(ops, clientv3.OpDelete(key))
		default:
			panic("unsuported operation type")
		}
	}
	return ops
}

func (t Traffic) pickOperationType() model.OperationType {
	roll := rand.Int() % 100
	if roll < 10 {
		return model.Delete
	}
	if roll < 50 {
		return model.Get
	}
	return model.Put
}

func randString(size int) string {
	data := strings.Builder{}
	data.Grow(size)
	for i := 0; i < size; i++ {
		data.WriteByte(byte(int('a') + rand.Intn(26)))
	}
	return data.String()
}
