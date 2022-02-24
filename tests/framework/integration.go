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

package framework

import (
	"context"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

type integrationRunner struct{}

func (e integrationRunner) TestMain(m *testing.M) {
	testutil.MustTestMainWithLeakDetection(m)
}

func (e integrationRunner) BeforeTest(t testing.TB) {
	integration.BeforeTest(t)
}

func (e integrationRunner) NewCluster(t testing.TB) Cluster {
	return &integrationCluster{
		Cluster: integration.NewCluster(t, &integration.ClusterConfig{Size: 1}),
		t:       t,
	}
}

type integrationCluster struct {
	*integration.Cluster
	t testing.TB
}

func (c *integrationCluster) Close() error {
	c.Terminate(c.t)
	return nil
}

func (c *integrationCluster) Client() Client {
	cc, err := c.ClusterClient()
	if err != nil {
		c.t.Fatal(err)
	}
	return &integrationClient{cc}
}

type integrationClient struct {
	*clientv3.Client
}

func (c integrationClient) Get(key string, opts ...testutils.GetOption) (*clientv3.GetResponse, error) {
	o := testutils.GetOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	clientOpts := []clientv3.OpOption{}
	if o.Serializable {
		clientOpts = append(clientOpts, clientv3.WithSerializable())
	}
	return c.Client.Get(context.Background(), key, clientOpts...)
}

func (c integrationClient) Put(key, value string) error {
	_, err := c.Client.Put(context.Background(), key, value)
	return err
}
