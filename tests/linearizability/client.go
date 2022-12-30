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

package linearizability

import (
	"context"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type recordingClient struct {
	client  clientv3.Client
	history *appendableHistory
}

func NewClient(endpoints []string, ids idProvider) (*recordingClient, error) {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    1 * time.Millisecond,
		DialKeepAliveTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	return &recordingClient{
		client:  *cc,
		history: newAppendableHistory(ids),
	}, nil
}

func (c *recordingClient) Close() error {
	return c.client.Close()
}

func (c *recordingClient) Get(ctx context.Context, key string) ([]*mvccpb.KeyValue, error) {
	callTime := time.Now()
	resp, err := c.client.Get(ctx, key)
	returnTime := time.Now()
	if err != nil {
		return nil, err
	}
	c.history.AppendGet(key, callTime, returnTime, resp)
	return resp.Kvs, nil
}

func (c *recordingClient) Put(ctx context.Context, key, value string) error {
	callTime := time.Now()
	resp, err := c.client.Put(ctx, key, value)
	returnTime := time.Now()
	c.history.AppendPut(key, value, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) Delete(ctx context.Context, key string) error {
	callTime := time.Now()
	resp, err := c.client.Delete(ctx, key)
	returnTime := time.Now()
	c.history.AppendDelete(key, callTime, returnTime, resp, err)
	return nil
}

func (c *recordingClient) Txn(ctx context.Context, key, expectedValue, newValue string) error {
	callTime := time.Now()
	txn := c.client.Txn(ctx)
	var cmp clientv3.Cmp
	if expectedValue == "" {
		cmp = clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
	} else {
		cmp = clientv3.Compare(clientv3.Value(key), "=", expectedValue)
	}
	resp, err := txn.If(
		cmp,
	).Then(
		clientv3.OpPut(key, newValue),
	).Commit()
	returnTime := time.Now()
	c.history.AppendTxn(key, expectedValue, newValue, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) LeaseGrant(ctx context.Context, ttl int64) (int64, error) {
	callTime := time.Now()
	resp, err := c.client.Lease.Grant(ctx, ttl)
	returnTime := time.Now()
	c.history.AppendLeaseGrant(callTime, returnTime, resp, err)
	var leaseId int64
	if resp != nil {
		leaseId = int64(resp.ID)
	}
	return leaseId, err
}

func (c *recordingClient) LeaseRevoke(ctx context.Context, leaseId int64) error {
	callTime := time.Now()
	resp, err := c.client.Lease.Revoke(ctx, clientv3.LeaseID(leaseId))
	returnTime := time.Now()
	c.history.AppendLeaseRevoke(leaseId, callTime, returnTime, resp, err)
	return err
}

func (c *recordingClient) PutWithLease(ctx context.Context, key string, value string, leaseId int64) error {
	callTime := time.Now()
	opts := clientv3.WithLease(clientv3.LeaseID(leaseId))
	resp, err := c.client.Put(ctx, key, value, opts)
	returnTime := time.Now()
	c.history.AppendPutWithLease(key, value, int64(leaseId), callTime, returnTime, resp, err)
	return err
}
