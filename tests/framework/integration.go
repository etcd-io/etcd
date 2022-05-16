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
	"strings"
	"testing"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	etcdctlcmd "go.etcd.io/etcd/etcdctl/v3/ctlv3/command"

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

func (e integrationRunner) NewCluster(ctx context.Context, t testing.TB, cfg config.ClusterConfig) Cluster {
	var err error
	var integrationCfg integration.ClusterConfig
	integrationCfg.Size = cfg.ClusterSize
	integrationCfg.ClientTLS, err = tlsInfo(t, cfg.ClientTLS)
	integrationCfg.QuotaBackendBytes = cfg.QuotaBackendBytes
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
		ctx:     ctx,
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
	t   testing.TB
	ctx context.Context
}

func (c *integrationCluster) Members() (ms []Member) {
	for _, m := range c.Cluster.Members {
		ms = append(ms, integrationMember{Member: m, t: c.t, ctx: c.ctx})
	}
	return ms
}

type integrationMember struct {
	*integration.Member
	t   testing.TB
	ctx context.Context
}

func (m integrationMember) Client() Client {
	return integrationClient{Client: m.Member.Client, ctx: m.ctx}
}

func (m integrationMember) Start() error {
	return m.Member.Restart(m.t)
}

func (m integrationMember) Stop() {
	m.Member.Stop(m.t)
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
	return integrationClient{Client: cc, ctx: c.ctx}
}

type integrationClient struct {
	*clientv3.Client
	ctx context.Context
}

func (c integrationClient) Get(key string, o config.GetOptions) (*clientv3.GetResponse, error) {
	ctx := c.ctx
	if o.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}
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
	return c.Client.Get(ctx, key, clientOpts...)
}

func (c integrationClient) Put(key, value string, opts config.PutOptions) error {
	clientOpts := []clientv3.OpOption{}
	if opts.LeaseID != 0 {
		clientOpts = append(clientOpts, clientv3.WithLease(opts.LeaseID))
	}
	_, err := c.Client.Put(c.ctx, key, value, clientOpts...)
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
	return c.Client.Delete(c.ctx, key, clientOpts...)
}

func (c integrationClient) Compact(rev int64, o config.CompactOption) (*clientv3.CompactResponse, error) {
	ctx := c.ctx
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

func (c integrationClient) AlarmList() (*clientv3.AlarmResponse, error) {
	return c.Client.AlarmList(c.ctx)
}

func (c integrationClient) AlarmDisarm(alarmMember *clientv3.AlarmMember) (*clientv3.AlarmResponse, error) {
	return c.Client.AlarmDisarm(c.ctx, alarmMember)
}

func (c integrationClient) Status() ([]*clientv3.StatusResponse, error) {
	endpoints := c.Client.Endpoints()
	var resp []*clientv3.StatusResponse
	for _, ep := range endpoints {
		status, err := c.Client.Status(c.ctx, ep)
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
		hashKV, err := c.Client.HashKV(c.ctx, ep, rev)
		if err != nil {
			return nil, err
		}
		resp = append(resp, hashKV)
	}
	return resp, nil
}

func (c integrationClient) Health() error {
	cli := healthpb.NewHealthClient(c.Client.ActiveConnection())
	resp, err := cli.Check(c.ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("status expected %s, got %s", healthpb.HealthCheckResponse_SERVING, resp.Status)
	}
	return nil
}

func (c integrationClient) Defragment(o config.DefragOption) error {
	ctx := c.ctx
	if o.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}
	for _, ep := range c.Endpoints() {
		_, err := c.Client.Defragment(ctx, ep)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c integrationClient) Grant(ttl int64) (*clientv3.LeaseGrantResponse, error) {
	return c.Client.Grant(c.ctx, ttl)
}

func (c integrationClient) TimeToLive(id clientv3.LeaseID, o config.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error) {
	leaseOpts := []clientv3.LeaseOption{}
	if o.WithAttachedKeys {
		leaseOpts = append(leaseOpts, clientv3.WithAttachedKeys())
	}

	return c.Client.TimeToLive(c.ctx, id, leaseOpts...)
}

func (c integrationClient) LeaseList() (*clientv3.LeaseLeasesResponse, error) {
	return c.Client.Leases(c.ctx)
}

func (c integrationClient) LeaseKeepAliveOnce(id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error) {
	return c.Client.KeepAliveOnce(c.ctx, id)
}

func (c integrationClient) LeaseRevoke(id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	return c.Client.Revoke(c.ctx, id)
}

func (c integrationClient) UserAdd(name, password string, opts config.UserAddOptions) (*clientv3.AuthUserAddResponse, error) {
	return c.Client.UserAddWithOptions(c.ctx, name, password, &clientv3.UserAddOptions{
		NoPassword: opts.NoPassword,
	})
}

func (c integrationClient) UserList() (*clientv3.AuthUserListResponse, error) {
	return c.Client.UserList(c.ctx)
}

func (c integrationClient) UserDelete(name string) (*clientv3.AuthUserDeleteResponse, error) {
	return c.Client.UserDelete(c.ctx, name)
}

func (c integrationClient) UserChangePass(user, newPass string) error {
	_, err := c.Client.UserChangePassword(c.ctx, user, newPass)
	return err
}

func (c integrationClient) RoleAdd(name string) (*clientv3.AuthRoleAddResponse, error) {
	return c.Client.RoleAdd(c.ctx, name)
}

func (c integrationClient) RoleGrantPermission(name string, key, rangeEnd string, permType clientv3.PermissionType) (*clientv3.AuthRoleGrantPermissionResponse, error) {
	return c.Client.RoleGrantPermission(c.ctx, name, key, rangeEnd, permType)
}

func (c integrationClient) RoleGet(role string) (*clientv3.AuthRoleGetResponse, error) {
	return c.Client.RoleGet(c.ctx, role)
}

func (c integrationClient) RoleList() (*clientv3.AuthRoleListResponse, error) {
	return c.Client.RoleList(c.ctx)
}

func (c integrationClient) RoleRevokePermission(role string, key, rangeEnd string) (*clientv3.AuthRoleRevokePermissionResponse, error) {
	return c.Client.RoleRevokePermission(c.ctx, role, key, rangeEnd)
}

func (c integrationClient) RoleDelete(role string) (*clientv3.AuthRoleDeleteResponse, error) {
	return c.Client.RoleDelete(c.ctx, role)
}

func (c integrationClient) Txn(compares, ifSucess, ifFail []string, o config.TxnOptions) (*clientv3.TxnResponse, error) {
	txn := c.Client.Txn(c.ctx)
	cmps := []clientv3.Cmp{}
	for _, c := range compares {
		cmp, err := etcdctlcmd.ParseCompare(c)
		if err != nil {
			return nil, err
		}
		cmps = append(cmps, *cmp)
	}
	succOps, err := getOps(ifSucess)
	if err != nil {
		return nil, err
	}
	failOps, err := getOps(ifFail)
	if err != nil {
		return nil, err
	}
	txnrsp, err := txn.
		If(cmps...).
		Then(succOps...).
		Else(failOps...).
		Commit()
	return txnrsp, err
}

func getOps(ss []string) ([]clientv3.Op, error) {
	ops := []clientv3.Op{}
	for _, s := range ss {
		s = strings.TrimSpace(s)
		args := etcdctlcmd.Argify(s)
		switch args[0] {
		case "get":
			ops = append(ops, clientv3.OpGet(args[1]))
		case "put":
			ops = append(ops, clientv3.OpPut(args[1], args[2]))
		case "del":
			ops = append(ops, clientv3.OpDelete(args[1]))
		}
	}
	return ops, nil
}
