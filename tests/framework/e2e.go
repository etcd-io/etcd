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
	"os"
	"strings"
	"testing"
	"time"

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

func (e e2eRunner) NewCluster(ctx context.Context, t testing.TB, cfg config.ClusterConfig) Cluster {
	e2eConfig := e2e.EtcdProcessClusterConfig{
		InitialToken:               "new",
		ClusterSize:                cfg.ClusterSize,
		QuotaBackendBytes:          cfg.QuotaBackendBytes,
		DisableStrictReconfigCheck: cfg.DisableStrictReconfigCheck,
		SnapshotCount:              cfg.SnapshotCount,
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
	epc, err := e2e.NewEtcdProcessCluster(ctx, t, &e2eConfig)
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

// WaitLeader returns index of the member in c.Members() that is leader
// or fails the test (if not established in 30s).
func (c *e2eCluster) WaitLeader(t testing.TB) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return c.WaitMembersForLeader(ctx, t, c.Members())
}

// WaitMembersForLeader waits until given members agree on the same leader,
// and returns its 'index' in the 'membs' list
func (c *e2eCluster) WaitMembersForLeader(ctx context.Context, t testing.TB, membs []Member) int {
	cc := c.Client()

	// ensure leader is up via linearizable get
	for {
		select {
		case <-ctx.Done():
			t.Fatal("WaitMembersForLeader timeout")
		default:
		}
		_, err := cc.Get(ctx, "0", config.GetOptions{Timeout: 10*config.TickDuration + time.Second})
		if err == nil || strings.Contains(err.Error(), "Key not found") {
			break
		}
	}

	leaders := make(map[uint64]struct{})
	members := make(map[uint64]int)
	for {
		select {
		case <-ctx.Done():
			t.Fatal("WaitMembersForLeader timeout")
		default:
		}
		for i := range membs {
			resp, err := membs[i].Client().Status(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "connection refused") {
					// if member[i] has stopped
					continue
				} else {
					t.Fatal(err)
				}
			}
			members[resp[0].Header.MemberId] = i
			leaders[resp[0].Leader] = struct{}{}
		}
		// members agree on the same leader
		if len(leaders) == 1 {
			break
		}
		leaders = make(map[uint64]struct{})
		members = make(map[uint64]int)
		time.Sleep(10 * config.TickDuration)
	}
	for l := range leaders {
		if index, ok := members[l]; ok {
			t.Logf("members agree on a leader, members:%v , leader:%v", members, l)
			return index
		}
		t.Fatalf("members agree on a leader which is not one of members, members:%v , leader:%v", members, l)
	}
	t.Fatal("impossible path of execution")
	return -1
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

func (m e2eMember) Start(ctx context.Context) error {
	return m.EtcdProcess.Start(ctx)
}

func (m e2eMember) Stop() {
	m.EtcdProcess.Stop()
}
