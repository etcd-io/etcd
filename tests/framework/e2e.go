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
	"os"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

type e2eRunner struct{}

func (e e2eRunner) TestMain(m *testing.M) {
	e2e.InitFlags()
	v := m.Run()
	if v == 0 && testutil.CheckLeakedGoroutine() {
		os.Exit(1)
	}
	os.Exit(v)
}

func (e e2eRunner) BeforeTest(t testing.TB) {
	e2e.BeforeTest(t)
}

func (e e2eRunner) NewCluster(t testing.TB) Cluster {
	epc, err := e2e.NewEtcdProcessCluster(t, e2e.ConfigStandalone(*e2e.NewConfigAutoTLS()))
	if err != nil {
		t.Fatalf("could not start etcd integrationCluster: %s", err)
	}
	return &e2eCluster{*epc}
}

type e2eCluster struct {
	e2e.EtcdProcessCluster
}

func (c *e2eCluster) Client() Client {
	return e2eClient{e2e.NewEtcdctl(c.Cfg, c.EndpointsV3())}
}

type e2eClient struct {
	*e2e.EtcdctlV3
}

func (c e2eClient) Get(key string, opts ...testutils.GetOption) (*clientv3.GetResponse, error) {
	o := testutils.GetOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	return c.EtcdctlV3.Get(key, o)
}
