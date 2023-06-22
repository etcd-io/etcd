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
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

// RecordingClient provides a semi etcd client (different interface than
// clientv3.Client) that records all the requests and responses made. Doesn't
// allow for concurrent requests to confirm to model.AppendableHistory requirements.
type RecordingClient struct {
	id     int
	client clientv3.Client
	// using baseTime time-measuring operation to get monotonic clock reading
	// see https://github.com/golang/go/blob/master/src/time/time.go#L17
	baseTime time.Time

	watchMux       sync.Mutex
	watchResponses []WatchResponse
	// mux ensures order of request appending.
	opMux      sync.Mutex
	operations *model.AppendableHistory
}

type WatchResponse struct {
	Events           []model.WatchEvent
	IsProgressNotify bool
	Revision         int64
	Time             time.Duration
}

type TimedWatchEvent struct {
	model.WatchEvent
	Time time.Duration
}

func NewClient(endpoints []string, ids identity.Provider, baseTime time.Time) (*RecordingClient, error) {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	return &RecordingClient{
		id:         ids.NewClientId(),
		client:     *cc,
		operations: model.NewAppendableHistory(ids),
		baseTime:   baseTime,
	}, nil
}

func (c *RecordingClient) Close() error {
	return c.client.Close()
}

func (c *RecordingClient) Report() ClientReport {
	return ClientReport{
		ClientId:         c.id,
		OperationHistory: c.operations.History,
		Watch:            c.watchResponses,
	}
}

type ClientReport struct {
	ClientId         int
	OperationHistory model.History
	Watch            []WatchResponse
}

func (r ClientReport) WatchEventCount() int {
	count := 0
	for _, resp := range r.Watch {
		count += len(resp.Events)
	}
	return count
}

func (c *RecordingClient) Get(ctx context.Context, key string, revision int64) (kv *mvccpb.KeyValue, rev int64, err error) {
	resp, err := c.Range(ctx, key, false, revision)
	if err != nil {
		return nil, 0, err
	}
	if len(resp.Kvs) == 1 {
		kv = resp.Kvs[0]
	}
	return kv, resp.Header.Revision, nil
}

func (c *RecordingClient) Range(ctx context.Context, key string, withPrefix bool, revision int64) (*clientv3.GetResponse, error) {
	ops := []clientv3.OpOption{}
	if withPrefix {
		ops = append(ops, clientv3.WithPrefix())
	}
	if revision != 0 {
		ops = append(ops, clientv3.WithRev(revision))
	}
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Get(ctx, key, ops...)
	if err != nil {
		return nil, err
	}
	returnTime := time.Since(c.baseTime)
	c.operations.AppendRange(key, withPrefix, revision, callTime, returnTime, resp)
	return resp, nil
}

func (c *RecordingClient) Put(ctx context.Context, key, value string) (*clientv3.PutResponse, error) {
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Put(ctx, key, value)
	returnTime := time.Since(c.baseTime)
	c.operations.AppendPut(key, value, callTime, returnTime, resp, err)
	return resp, err
}

func (c *RecordingClient) Delete(ctx context.Context, key string) (*clientv3.DeleteResponse, error) {
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Delete(ctx, key)
	returnTime := time.Since(c.baseTime)
	c.operations.AppendDelete(key, callTime, returnTime, resp, err)
	return resp, err
}

func (c *RecordingClient) Txn(ctx context.Context, conditions []clientv3.Cmp, onSuccess []clientv3.Op, onFailure []clientv3.Op) (*clientv3.TxnResponse, error) {
	txn := c.client.Txn(ctx).If(
		conditions...,
	).Then(
		onSuccess...,
	).Else(
		onFailure...,
	)
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := txn.Commit()
	returnTime := time.Since(c.baseTime)
	c.operations.AppendTxn(conditions, onSuccess, onFailure, callTime, returnTime, resp, err)
	return resp, err
}

func (c *RecordingClient) LeaseGrant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Lease.Grant(ctx, ttl)
	returnTime := time.Since(c.baseTime)
	c.operations.AppendLeaseGrant(callTime, returnTime, resp, err)
	return resp, err
}

func (c *RecordingClient) LeaseRevoke(ctx context.Context, leaseId int64) (*clientv3.LeaseRevokeResponse, error) {
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Lease.Revoke(ctx, clientv3.LeaseID(leaseId))
	returnTime := time.Since(c.baseTime)
	c.operations.AppendLeaseRevoke(leaseId, callTime, returnTime, resp, err)
	return resp, err
}

func (c *RecordingClient) PutWithLease(ctx context.Context, key string, value string, leaseId int64) (*clientv3.PutResponse, error) {
	opts := clientv3.WithLease(clientv3.LeaseID(leaseId))
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Put(ctx, key, value, opts)
	returnTime := time.Since(c.baseTime)
	c.operations.AppendPutWithLease(key, value, leaseId, callTime, returnTime, resp, err)
	return resp, err
}

func (c *RecordingClient) Defragment(ctx context.Context) (*clientv3.DefragmentResponse, error) {
	c.opMux.Lock()
	defer c.opMux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Defragment(ctx, c.client.Endpoints()[0])
	returnTime := time.Since(c.baseTime)
	c.operations.AppendDefragment(callTime, returnTime, resp, err)
	return resp, err
}

func (c *RecordingClient) Watch(ctx context.Context, key string, rev int64, withPrefix bool, withProgressNotify bool) clientv3.WatchChan {
	ops := []clientv3.OpOption{clientv3.WithProgressNotify()}
	if withPrefix {
		ops = append(ops, clientv3.WithPrefix())
	}
	if rev != 0 {
		ops = append(ops, clientv3.WithRev(rev))
	}
	if withProgressNotify {
		ops = append(ops, clientv3.WithProgressNotify())
	}
	respCh := make(chan clientv3.WatchResponse)
	go func() {
		defer close(respCh)
		for r := range c.client.Watch(ctx, key, ops...) {
			c.watchMux.Lock()
			c.watchResponses = append(c.watchResponses, ToWatchResponse(r, c.baseTime))
			c.watchMux.Unlock()
			select {
			case respCh <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return respCh
}

func (c *RecordingClient) RequestProgress(ctx context.Context) error {
	return c.client.RequestProgress(ctx)
}

func ToWatchResponse(r clientv3.WatchResponse, baseTime time.Time) WatchResponse {
	// using time.Since time-measuring operation to get monotonic clock reading
	// see https://github.com/golang/go/blob/master/src/time/time.go#L17
	resp := WatchResponse{Time: time.Since(baseTime)}
	for _, event := range r.Events {
		resp.Events = append(resp.Events, toWatchEvent(*event))
	}
	resp.IsProgressNotify = r.IsProgressNotify()
	resp.Revision = r.Header.Revision
	return resp
}

func toWatchEvent(event clientv3.Event) (watch model.WatchEvent) {
	watch.Revision = event.Kv.ModRevision
	watch.Key = string(event.Kv.Key)
	watch.Value = model.ToValueOrHash(string(event.Kv.Value))

	switch event.Type {
	case mvccpb.PUT:
		watch.Type = model.PutOperation
	case mvccpb.DELETE:
		watch.Type = model.DeleteOperation
	default:
		panic(fmt.Sprintf("Unexpected event type: %s", event.Type))
	}
	return watch
}
