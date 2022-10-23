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
	id     int

	operations []porcupine.Operation
}

func NewClient(endpoints []string, id int) (*recordingClient, error) {
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
		id:         id,
		operations: []porcupine.Operation{},
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
		Input:    etcdRequest{op: Get, key: key},
		Call:     callTime.UnixNano(),
		Output:   etcdResponse{getData: readData},
		Return:   returnTime.UnixNano(),
	})
	return nil
}

func (c *recordingClient) Put(ctx context.Context, key, value string) error {
	callTime := time.Now()
	_, err := c.client.Put(ctx, key, value)
	returnTime := time.Now()
	c.operations = append(c.operations, porcupine.Operation{
		ClientId: c.id,
		Input:    etcdRequest{op: Put, key: key, putData: value},
		Call:     callTime.UnixNano(),
		Output:   etcdResponse{err: err},
		Return:   returnTime.UnixNano(),
	})
	return nil
}
