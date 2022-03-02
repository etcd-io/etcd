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
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
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

func (e e2eRunner) NewCluster(t testing.TB, cfg config.ClusterConfig) Cluster {
	e2eConfig := e2e.EtcdProcessClusterConfig{
		InitialToken:      "new",
		ClusterSize:       cfg.ClusterSize,
		QuotaBackendBytes: cfg.QuotaBackendBytes,
	}
	switch cfg.ClientTLS {
	case config.NoTLS:
		e2eConfig.ClientTLS = e2e.ClientNonTLS
	case config.AutoTLS:
		e2eConfig.IsClientAutoTLS = true
		e2eConfig.ClientTLS = e2e.ClientTLS
	case config.ManualTLS:
		e2eConfig.IsClientAutoTLS = false
		e2eConfig.ClientTLS = e2e.ClientTLS
	default:
		t.Fatalf("ClientTLS config %q not supported", cfg.ClientTLS)
	}
	switch cfg.PeerTLS {
	case config.NoTLS:
		e2eConfig.IsPeerTLS = false
		e2eConfig.IsPeerAutoTLS = false
	case config.AutoTLS:
		e2eConfig.IsPeerTLS = true
		e2eConfig.IsPeerAutoTLS = true
	case config.ManualTLS:
		e2eConfig.IsPeerTLS = true
		e2eConfig.IsPeerAutoTLS = false
	default:
		t.Fatalf("PeerTLS config %q not supported", cfg.PeerTLS)
	}
	epc, err := e2e.NewEtcdProcessCluster(t, &e2eConfig)
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

func (c *e2eCluster) Members() (ms []Member) {
	for _, proc := range c.EtcdProcessCluster.Procs {
		ms = append(ms, e2eMember{EtcdProcess: proc, Cfg: c.Cfg})
	}
	return ms
}

type e2eClient struct {
	*e2e.EtcdctlV3
}

type e2eMember struct {
	e2e.EtcdProcess
	Cfg *e2e.EtcdProcessClusterConfig
}

func (m e2eMember) Client() Client {
	return e2eClient{e2e.NewEtcdctl(m.Cfg, m.EndpointsV3())}
}

func (m e2eMember) Start() error {
	return m.Restart()
}

func (m e2eMember) Stop() {
	m.EtcdProcess.Stop()
}
