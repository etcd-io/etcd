// Copyright 2017 The etcd Authors
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

package readonly

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	epb "github.com/coreos/etcd/etcdserver/api/v3election/v3electionpb"
	lockpb "github.com/coreos/etcd/etcdserver/api/v3lock/v3lockpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/proxy/grpcproxy"

	"google.golang.org/grpc"
)

func TestReadOnlyKVProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	rops := newReadOnlyProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer rops.close()

	cfg := clientv3.Config{
		Endpoints:   []string{rops.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer client.Close()

	_, err = client.Put(context.TODO(), "foo", "bar")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.Delete(context.TODO(), "foo")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	tnx := client.Txn(context.TODO())
	_, err = tnx.Then(clientv3.OpPut("foo", "bar")).Commit()
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.Compact(context.TODO(), 0)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}
}

func TestReadOnlyClusterProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	rops := newReadOnlyProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer rops.close()

	cfg := clientv3.Config{
		Endpoints:   []string{rops.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer client.Close()

	_, err = client.MemberAdd(context.TODO(), []string{"http://localhost:12380"})
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	resp, err := client.MemberList(context.TODO())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(resp.Members) != 1 {
		t.Fatalf("cluster member num = %d, want 1", len(resp.Members))
	}

	_, err = client.MemberRemove(context.TODO(), resp.Members[0].ID)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.MemberUpdate(
		context.TODO(),
		resp.Members[0].ID,
		[]string{"http://localhost:12380"},
	)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}
}

func TestReadOnlyLeaseProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	rops := newReadOnlyProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer rops.close()

	cfg := clientv3.Config{
		Endpoints:   []string{rops.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer client.Close()

	_, err = client.Grant(context.TODO(), 5)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.Revoke(context.TODO(), 0)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}
}

func TestReadOnlyMaintenanceProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	rops := newReadOnlyProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer rops.close()

	ep := rops.l.Addr().String()
	cfg := clientv3.Config{
		Endpoints:   []string{ep},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer client.Close()

	_, err = client.Defragment(context.TODO(), ep)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.AlarmDisarm(
		context.TODO(),
		&clientv3.AlarmMember{Alarm: pb.AlarmType_NOSPACE},
	)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.MoveLeader(context.TODO(), 0)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}
}

func TestReadOnlyAuthProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	rops := newReadOnlyProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer rops.close()

	cfg := clientv3.Config{
		Endpoints:   []string{rops.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer client.Close()

	_, err = client.AuthEnable(context.TODO())
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.AuthDisable(context.TODO())
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.RoleAdd(context.TODO(), "root")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.RoleDelete(context.TODO(), "root")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.RoleRevokePermission(context.TODO(), "root", "foo", "bar")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.RoleGrantPermission(
		context.TODO(),
		"root",
		"foo",
		"bar",
		clientv3.PermissionType(clientv3.PermReadWrite),
	)
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.UserAdd(context.TODO(), "root", "123")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.UserDelete(context.TODO(), "root")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.UserGrantRole(context.TODO(), "root", "root")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.UserRevokeRole(context.TODO(), "root", "root")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = client.UserChangePassword(context.TODO(), "root", "123")
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}
}

func TestReadOnlyLockProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	rops := newReadOnlyProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer rops.close()

	cfg := clientv3.Config{
		Endpoints:   []string{rops.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer client.Close()

	lc := lockpb.NewLockClient(client.ActiveConnection())

	_, err = lc.Lock(context.TODO(), &lockpb.LockRequest{})
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = lc.Unlock(context.TODO(), &lockpb.UnlockRequest{})
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}
}

func TestReadOnlyElectionProxy(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	rops := newReadOnlyProxyServer([]string{clus.Members[0].GRPCAddr()}, t)
	defer rops.close()

	cfg := clientv3.Config{
		Endpoints:   []string{rops.l.Addr().String()},
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	defer client.Close()

	ec := epb.NewElectionClient(client.ActiveConnection())

	_, err = ec.Campaign(context.TODO(), &epb.CampaignRequest{})
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = ec.Proclaim(context.TODO(), &epb.ProclaimRequest{})
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}

	_, err = ec.Resign(context.TODO(), &epb.ResignRequest{})
	if !isErrReadOnly(err) {
		t.Fatalf("err = %v, want read-only error", err)
	}
}

func isErrReadOnly(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrReadOnly.Error())
}

type readOnlyProxyAPI struct {
	KV pb.KVServer

	Cluster pb.ClusterServer

	Lease pb.LeaseServer

	Maintenance pb.MaintenanceServer

	Auth pb.AuthServer

	Lock lockpb.LockServer

	Election epb.ElectionServer
}

type readOnlyProxyServer struct {
	api    *readOnlyProxyAPI
	c      *clientv3.Client
	server *grpc.Server
	l      net.Listener
}

func (ps *readOnlyProxyServer) close() {
	ps.server.Stop()
	ps.l.Close()
	ps.c.Close()
}

func newReadOnlyProxyServer(endpoints []string, t *testing.T) *readOnlyProxyServer {
	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	kvp, _ := grpcproxy.NewKvProxy(client)
	clusterp, _ := grpcproxy.NewClusterProxy(client, "", "")
	leasep, _ := grpcproxy.NewLeaseProxy(client)
	mainp := grpcproxy.NewMaintenanceProxy(client)
	authp := grpcproxy.NewAuthProxy(client)
	electionp := grpcproxy.NewElectionProxy(client)
	lockp := grpcproxy.NewLockProxy(client)

	api := &readOnlyProxyAPI{
		KV:          NewReadOnlyKvProxy(kvp),
		Cluster:     NewReadOnlyClusterProxy(clusterp),
		Lease:       NewReadOnlyLeaseProxy(leasep),
		Maintenance: NewReadOnlyMaintenanceProxy(mainp),
		Auth:        NewReadOnlyAuthProxy(authp),
		Lock:        NewReadOnlyLockProxy(lockp),
		Election:    NewReadOnlyElectionProxy(electionp),
	}

	ps := &readOnlyProxyServer{
		api: api,
		c:   client,
	}

	var opts []grpc.ServerOption
	ps.server = grpc.NewServer(opts...)

	pb.RegisterKVServer(ps.server, ps.api.KV)
	pb.RegisterClusterServer(ps.server, ps.api.Cluster)
	pb.RegisterLeaseServer(ps.server, ps.api.Lease)
	pb.RegisterMaintenanceServer(ps.server, ps.api.Maintenance)
	pb.RegisterAuthServer(ps.server, ps.api.Auth)
	epb.RegisterElectionServer(ps.server, ps.api.Election)
	lockpb.RegisterLockServer(ps.server, ps.api.Lock)

	ps.l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go ps.server.Serve(ps.l)

	return ps
}
