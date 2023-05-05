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
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

type recordingClient struct {
	client   clientv3.Client
	history  *model.AppendableHistory
	baseTime time.Time
}

func NewClient(endpoints []string, ids identity.Provider, baseTime time.Time) (*recordingClient, error) {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	return &recordingClient{
		client:   *cc,
		history:  model.NewAppendableHistory(ids),
		baseTime: baseTime,
	}, nil
}

func (c *recordingClient) Close() error {
	return c.client.Close()
}

func (c *recordingClient) Get(ctx context.Context, key string) (*mvccpb.KeyValue, error) {
	resp, err := c.Range(ctx, key, false)
	if err != nil || len(resp) == 0 {
		return nil, err
	}
	if len(resp) == 1 {
		return resp[0], err
	}
	panic(fmt.Sprintf("Unexpected response size: %d", len(resp)))
}

func (c *recordingClient) Range(ctx context.Context, key string, withPrefix bool) ([]*mvccpb.KeyValue, error) {
	callTime := time.Since(c.baseTime)
	ops := []clientv3.OpOption{}
	if withPrefix {
		ops = append(ops, clientv3.WithPrefix())
	}
	resp, err := c.client.Get(ctx, key, ops...)
	returnTime := time.Since(c.baseTime)
	if err != nil {
		return nil, err
	}
	c.history.AppendRange(key, withPrefix, callTime, returnTime, resp)
	return resp.Kvs, nil
}

func (c *recordingClient) Put(ctx context.Context, key, value string) error {
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Put(ctx, key, value)
	returnTime := time.Since(c.baseTime)
	c.history.AppendPut(key, value, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) Delete(ctx context.Context, key string) error {
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Delete(ctx, key)
	returnTime := time.Since(c.baseTime)
	c.history.AppendDelete(key, callTime, returnTime, resp, err)
	return nil
}

func (c *recordingClient) CompareRevisionAndDelete(ctx context.Context, key string, expectedRevision int64) error {
	callTime := time.Since(c.baseTime)
	resp, err := c.compareRevisionTxn(ctx, key, expectedRevision, clientv3.OpDelete(key)).Commit()
	returnTime := time.Since(c.baseTime)
	c.history.AppendCompareRevisionAndDelete(key, expectedRevision, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) CompareRevisionAndPut(ctx context.Context, key, value string, expectedRevision int64) error {
	callTime := time.Since(c.baseTime)
	resp, err := c.compareRevisionTxn(ctx, key, expectedRevision, clientv3.OpPut(key, value)).Commit()
	returnTime := time.Since(c.baseTime)
	c.history.AppendCompareRevisionAndPut(key, expectedRevision, value, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) compareRevisionTxn(ctx context.Context, key string, expectedRevision int64, op clientv3.Op) clientv3.Txn {
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

func (c *recordingClient) Txn(ctx context.Context, cmp []clientv3.Cmp, ops []clientv3.Op) error {
	callTime := time.Since(c.baseTime)
	txn := c.client.Txn(ctx)
	resp, err := txn.If(
		cmp...,
	).Then(
		ops...,
	).Commit()
	returnTime := time.Since(c.baseTime)
	c.history.AppendTxn(cmp, ops, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) LeaseGrant(ctx context.Context, ttl int64) (int64, error) {
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

func (c *recordingClient) LeaseRevoke(ctx context.Context, leaseId int64) error {
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Lease.Revoke(ctx, clientv3.LeaseID(leaseId))
	returnTime := time.Since(c.baseTime)
	c.history.AppendLeaseRevoke(leaseId, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) PutWithLease(ctx context.Context, key string, value string, leaseId int64) error {
	callTime := time.Since(c.baseTime)
	opts := clientv3.WithLease(clientv3.LeaseID(leaseId))
	resp, err := c.client.Put(ctx, key, value, opts)
	returnTime := time.Since(c.baseTime)
	c.history.AppendPutWithLease(key, value, int64(leaseId), callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) Defragment(ctx context.Context) error {
	callTime := time.Since(c.baseTime)
	resp, err := c.client.Defragment(ctx, c.client.Endpoints()[0])
	returnTime := time.Since(c.baseTime)
	c.history.AppendDefragment(callTime, returnTime, resp, err)
	return err
}
