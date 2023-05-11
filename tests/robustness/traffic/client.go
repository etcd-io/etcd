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
	client   clientv3.Client
	baseTime time.Time
	// mux ensures order of request appending.
	mux     sync.Mutex
	history *model.AppendableHistory
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
		client:   *cc,
		history:  model.NewAppendableHistory(ids),
		baseTime: baseTime,
	}, nil
}

func (c *RecordingClient) Close() error {
	return c.client.Close()
}

func (c *RecordingClient) Operations() model.History {
	return c.history.History
}

func (c *RecordingClient) Get(ctx context.Context, key string) (*mvccpb.KeyValue, error) {
	resp, err := c.Range(ctx, key, false)
	if err != nil || len(resp) == 0 {
		return nil, err
	}
	if len(resp) == 1 {
		return resp[0], err
	}
	panic(fmt.Sprintf("Unexpected response size: %d", len(resp)))
}

func (c *RecordingClient) Range(ctx context.Context, key string, withPrefix bool) ([]*mvccpb.KeyValue, error) {
	ops := []clientv3.OpOption{}
	if withPrefix {
		ops = append(ops, clientv3.WithPrefix())
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Get(ctx, key, ops...)
	if err != nil {
		return nil, err
	}
	returnTime := time.Since(c.baseTime)
	c.history.AppendRange(key, withPrefix, callTime, returnTime, resp)
	return resp.Kvs, nil
}

func (c *RecordingClient) Put(ctx context.Context, key, value string) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Put(ctx, key, value)
	returnTime := time.Since(c.baseTime)
	c.history.AppendPut(key, value, callTime, returnTime, resp, err)
	return err
}

func (c *RecordingClient) Delete(ctx context.Context, key string) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Delete(ctx, key)
	returnTime := time.Since(c.baseTime)
	c.history.AppendDelete(key, callTime, returnTime, resp, err)
	return nil
}

func (c *RecordingClient) CompareRevisionAndDelete(ctx context.Context, key string, expectedRevision int64) error {
	txn := c.compareRevisionTxn(ctx, key, expectedRevision, clientv3.OpDelete(key))
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := txn.Commit()
	returnTime := time.Since(c.baseTime)
	c.history.AppendCompareRevisionAndDelete(key, expectedRevision, callTime, returnTime, resp, err)
	return err
}

func (c *RecordingClient) CompareRevisionAndPut(ctx context.Context, key, value string, expectedRevision int64) error {
	txn := c.compareRevisionTxn(ctx, key, expectedRevision, clientv3.OpPut(key, value))
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := txn.Commit()
	returnTime := time.Since(c.baseTime)
	c.history.AppendCompareRevisionAndPut(key, expectedRevision, value, callTime, returnTime, resp, err)
	return err
}

func (c *RecordingClient) compareRevisionTxn(ctx context.Context, key string, expectedRevision int64, op clientv3.Op) clientv3.Txn {
	txn := c.client.Txn(ctx)
	var cmp clientv3.Cmp
	if expectedRevision == 0 {
		cmp = clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
	} else {
		cmp = clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)
	}
	return txn.If(
		cmp,
	).Then(
		op,
	)
}

func (c *RecordingClient) Txn(ctx context.Context, cmp []clientv3.Cmp, ops []clientv3.Op) error {
	txn := c.client.Txn(ctx).If(
		cmp...,
	).Then(
		ops...,
	)
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := txn.Commit()
	returnTime := time.Since(c.baseTime)
	c.history.AppendTxn(cmp, ops, callTime, returnTime, resp, err)
	return err
}

func (c *RecordingClient) LeaseGrant(ctx context.Context, ttl int64) (int64, error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Lease.Grant(ctx, ttl)
	returnTime := time.Since(c.baseTime)
	c.history.AppendLeaseGrant(callTime, returnTime, resp, err)
	var leaseId int64
	if resp != nil {
		leaseId = int64(resp.ID)
	}
	return leaseId, err
}

func (c *RecordingClient) LeaseRevoke(ctx context.Context, leaseId int64) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Lease.Revoke(ctx, clientv3.LeaseID(leaseId))
	returnTime := time.Since(c.baseTime)
	c.history.AppendLeaseRevoke(leaseId, callTime, returnTime, resp, err)
	return err
}

func (c *RecordingClient) PutWithLease(ctx context.Context, key string, value string, leaseId int64) error {
	opts := clientv3.WithLease(clientv3.LeaseID(leaseId))
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Put(ctx, key, value, opts)
	returnTime := time.Since(c.baseTime)
	c.history.AppendPutWithLease(key, value, leaseId, callTime, returnTime, resp, err)
	return err
}

func (c *RecordingClient) Defragment(ctx context.Context) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Defragment(ctx, c.client.Endpoints()[0])
	returnTime := time.Since(c.baseTime)
	c.history.AppendDefragment(callTime, returnTime, resp, err)
	return err
}
