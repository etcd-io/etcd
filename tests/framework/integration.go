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
	"fmt"
	"testing"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"go.uber.org/zap"
)

type integrationRunner struct{}

func (e integrationRunner) TestMain(m *testing.M) {
	testutil.MustTestMainWithLeakDetection(m)
}

func (e integrationRunner) BeforeTest(t testing.TB) {
	integration.BeforeTest(t)
}

func (e integrationRunner) NewCluster(t testing.TB, cfg config.ClusterConfig) Cluster {
	var err error
	var integrationCfg integration.ClusterConfig
	integrationCfg.Size = cfg.ClusterSize
	integrationCfg.ClientTLS, err = tlsInfo(t, cfg.ClientTLS)
	if err != nil {
		t.Fatalf("ClientTLS: %s", err)
	}
	integrationCfg.PeerTLS, err = tlsInfo(t, cfg.PeerTLS)
	if err != nil {
		t.Fatalf("PeerTLS: %s", err)
	}
	return &integrationCluster{
		Cluster: integration.NewCluster(t, &integrationCfg),
		t:       t,
	}
}

func tlsInfo(t testing.TB, cfg config.TLSConfig) (*transport.TLSInfo, error) {
	switch cfg {
	case config.NoTLS:
		return nil, nil
	case config.AutoTLS:
		tls, err := transport.SelfCert(zap.NewNop(), t.TempDir(), []string{"localhost"}, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to generate cert: %s", err)
		}
		return &tls, nil
	case config.ManualTLS:
		return &integration.TestTLSInfo, nil
	default:
		return nil, fmt.Errorf("config %q not supported", cfg)
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

func (c integrationClient) Get(key string, o config.GetOptions) (*clientv3.GetResponse, error) {
	clientOpts := []clientv3.OpOption{}
	if o.Revision != 0 {
		clientOpts = append(clientOpts, clientv3.WithRev(int64(o.Revision)))
	}
	if o.End != "" {
		clientOpts = append(clientOpts, clientv3.WithRange(o.End))
	}
	if o.Serializable {
		clientOpts = append(clientOpts, clientv3.WithSerializable())
	}
	if o.Prefix {
		clientOpts = append(clientOpts, clientv3.WithPrefix())
	}
	if o.Limit != 0 {
		clientOpts = append(clientOpts, clientv3.WithLimit(int64(o.Limit)))
	}
	if o.FromKey {
		clientOpts = append(clientOpts, clientv3.WithFromKey())
	}
	if o.CountOnly {
		clientOpts = append(clientOpts, clientv3.WithCountOnly())
	}
	if o.SortBy != clientv3.SortByKey || o.Order != clientv3.SortNone {
		clientOpts = append(clientOpts, clientv3.WithSort(o.SortBy, o.Order))
	}
	return c.Client.Get(context.Background(), key, clientOpts...)
}

func (c integrationClient) Put(key, value string) error {
	_, err := c.Client.Put(context.Background(), key, value)
	return err
}

func (c integrationClient) Delete(key string, o config.DeleteOptions) (*clientv3.DeleteResponse, error) {
	clientOpts := []clientv3.OpOption{}
	if o.Prefix {
		clientOpts = append(clientOpts, clientv3.WithPrefix())
	}
	if o.FromKey {
		clientOpts = append(clientOpts, clientv3.WithFromKey())
	}
	if o.End != "" {
		clientOpts = append(clientOpts, clientv3.WithRange(o.End))
	}
	return c.Client.Delete(context.Background(), key, clientOpts...)
}

func (c integrationClient) Compact(rev int64, o config.CompactOption) (*clientv3.CompactResponse, error) {
	ctx := context.Background()
	if o.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}
	clientOpts := []clientv3.CompactOption{}
	if o.Physical {
		clientOpts = append(clientOpts, clientv3.WithCompactPhysical())
	}
	return c.Client.Compact(ctx, rev, clientOpts...)
}

func (c integrationClient) Status() ([]*clientv3.StatusResponse, error) {
	endpoints := c.Client.Endpoints()
	var resp []*clientv3.StatusResponse
	for _, ep := range endpoints {
		status, err := c.Client.Status(context.Background(), ep)
		if err != nil {
			return nil, err
		}
		resp = append(resp, status)
	}
	return resp, nil
}

func (c integrationClient) HashKV(rev int64) ([]*clientv3.HashKVResponse, error) {
	endpoints := c.Client.Endpoints()
	var resp []*clientv3.HashKVResponse
	for _, ep := range endpoints {
		hashKV, err := c.Client.HashKV(context.Background(), ep, rev)
		if err != nil {
			return nil, err
		}
		resp = append(resp, hashKV)
	}
	return resp, nil
}

func (c integrationClient) Health() error {
	cli := healthpb.NewHealthClient(c.Client.ActiveConnection())
	resp, err := cli.Check(context.TODO(), &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("status expected %s, got %s", healthpb.HealthCheckResponse_SERVING, resp.Status)
	}
	return nil
}
