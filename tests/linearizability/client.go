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

	"github.com/anishathalye/porcupine"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

type recordingClient struct {
	client clientv3.Client

	// id of the next write operation. If needed a new id might be requested from idProvider.
	id         int
	idProvider idProvider

	operations []porcupine.Operation
	failed     []porcupine.Operation
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
		client:     *cc,
		id:         ids.ClientId(),
		idProvider: ids,
		operations: []porcupine.Operation{},
		failed:     []porcupine.Operation{},
	}, nil
}

func (c *recordingClient) Close() error {
	return c.client.Close()
}

func (c *recordingClient) Get(ctx context.Context, key string) error {
	callTime := time.Now()
	resp, err := c.client.Get(ctx, key)
	returnTime := time.Now()
	if err != nil {
		return err
	}
	var readData string
	if len(resp.Kvs) == 1 {
		readData = string(resp.Kvs[0].Value)
	}
	c.operations = append(c.operations, porcupine.Operation{
		ClientId: c.id,
		Input:    EtcdRequest{Op: Get, Key: key},
		Call:     callTime.UnixNano(),
		Output:   EtcdResponse{GetData: readData, Revision: resp.Header.Revision},
		Return:   returnTime.UnixNano(),
	})
	return nil
}

func (c *recordingClient) Put(ctx context.Context, key, value string) error {
	callTime := time.Now()
	resp, err := c.client.Put(ctx, key, value)
	returnTime := time.Now()
	if err != nil {
		c.failed = append(c.failed, porcupine.Operation{
			ClientId: c.id,
			Input:    EtcdRequest{Op: Put, Key: key, PutData: value},
			Call:     callTime.UnixNano(),
			Output:   EtcdResponse{Err: err},
			Return:   0, // For failed writes we don't know when request has really finished.
		})
		// Operations of single client needs to be sequential.
		// As we don't know return time of failed operations, all new writes need to be done with new client id.
		c.id = c.idProvider.ClientId()
		return err
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	c.operations = append(c.operations, porcupine.Operation{
		ClientId: c.id,
		Input:    EtcdRequest{Op: Put, Key: key, PutData: value},
		Call:     callTime.UnixNano(),
		Output:   EtcdResponse{Err: err, Revision: revision},
		Return:   returnTime.UnixNano(),
	})
	return nil
}

func (c *recordingClient) Delete(ctx context.Context, key string) error {
	callTime := time.Now()
	resp, err := c.client.Delete(ctx, key)
	returnTime := time.Now()
	if err != nil {
		c.failed = append(c.failed, porcupine.Operation{
			ClientId: c.id,
			Input:    EtcdRequest{Op: Delete, Key: key},
			Call:     callTime.UnixNano(),
			Output:   EtcdResponse{Err: err},
			Return:   0, // For failed writes we don't know when request has really finished.
		})
		// Operations of single client needs to be sequential.
		// As we don't know return time of failed operations, all new writes need to be done with new client id.
		c.id = c.idProvider.ClientId()
		return err
	}
	var revision int64
	var deleted int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
		deleted = resp.Deleted
	}
	c.operations = append(c.operations, porcupine.Operation{
		ClientId: c.id,
		Input:    EtcdRequest{Op: Delete, Key: key},
		Call:     callTime.UnixNano(),
		Output:   EtcdResponse{Revision: revision, Deleted: deleted, Err: err},
		Return:   returnTime.UnixNano(),
	})
	return nil
}

func (c *recordingClient) Operations() []porcupine.Operation {
	operations := make([]porcupine.Operation, 0, len(c.operations)+len(c.failed))
	var maxTime int64
	for _, op := range c.operations {
		operations = append(operations, op)
		if op.Return > maxTime {
			maxTime = op.Return
		}
	}
	for _, op := range c.failed {
		if op.Call > maxTime {
			continue
		}
		op.Return = maxTime + 1
		operations = append(operations, op)
	}
	return operations
}
