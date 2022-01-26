// Copyright 2016 The etcd Authors
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

package connectivity_test

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/v3"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
	clientv3test "go.etcd.io/etcd/tests/v3/integration/clientv3"
	"google.golang.org/grpc"
)

var (
	testTLSInfo = transport.TLSInfo{
		KeyFile:        integration2.MustAbsPath("../../../fixtures/server.key.insecure"),
		CertFile:       integration2.MustAbsPath("../../../fixtures/server.crt"),
		TrustedCAFile:  integration2.MustAbsPath("../../../fixtures/ca.crt"),
		ClientCertAuth: true,
	}

	testTLSInfoExpired = transport.TLSInfo{
		KeyFile:        integration2.MustAbsPath("../../fixtures-expired/server.key.insecure"),
		CertFile:       integration2.MustAbsPath("../../fixtures-expired/server.crt"),
		TrustedCAFile:  integration2.MustAbsPath("../../fixtures-expired/ca.crt"),
		ClientCertAuth: true,
	}
)

// TestDialTLSExpired tests client with expired certs fails to dial.
func TestDialTLSExpired(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1, PeerTLS: &testTLSInfo, ClientTLS: &testTLSInfo})
	defer clus.Terminate(t)

	tls, err := testTLSInfoExpired.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	// expect remote errors "tls: bad certificate"
	_, err = integration2.NewClient(t, clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCURL()},
		DialTimeout: 3 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         tls,
	})
	if !clientv3test.IsClientTimeout(err) {
		t.Fatalf("expected dial timeout error, got %v", err)
	}
}

// TestDialTLSNoConfig ensures the client fails to dial / times out
// when TLS endpoints (https, unixs) are given but no tls config.
func TestDialTLSNoConfig(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1, ClientTLS: &testTLSInfo})
	defer clus.Terminate(t)
	// expect "signed by unknown authority"
	c, err := integration2.NewClient(t, clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCURL()},
		DialTimeout: time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	})
	defer func() {
		if c != nil {
			c.Close()
		}
	}()
	if !clientv3test.IsClientTimeout(err) {
		t.Fatalf("expected dial timeout error, got %v", err)
	}
}

// TestDialSetEndpointsBeforeFail ensures SetEndpoints can replace unavailable
// endpoints with available ones.
func TestDialSetEndpointsBeforeFail(t *testing.T) {
	testDialSetEndpoints(t, true)
}

func TestDialSetEndpointsAfterFail(t *testing.T) {
	testDialSetEndpoints(t, false)
}

// testDialSetEndpoints ensures SetEndpoints can replace unavailable endpoints with available ones.
func testDialSetEndpoints(t *testing.T, setBefore bool) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	// get endpoint list
	eps := make([]string, 3)
	for i := range eps {
		eps[i] = clus.Members[i].GRPCURL()
	}
	toKill := rand.Intn(len(eps))

	cfg := clientv3.Config{
		Endpoints:   []string{eps[toKill]},
		DialTimeout: 1 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	cli, err := integration2.NewClient(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	if setBefore {
		cli.SetEndpoints(eps[toKill%3], eps[(toKill+1)%3])
	}
	// make a dead node
	clus.Members[toKill].Stop(t)
	clus.WaitLeader(t)

	if !setBefore {
		cli.SetEndpoints(eps[toKill%3], eps[(toKill+1)%3])
	}
	time.Sleep(time.Second * 2)
	ctx, cancel := context.WithTimeout(context.Background(), integration2.RequestWaitTimeout)
	if _, err = cli.Get(ctx, "foo", clientv3.WithSerializable()); err != nil {
		t.Fatal(err)
	}
	cancel()
}

// TestSwitchSetEndpoints ensures SetEndpoints can switch one endpoint
// with a new one that doesn't include original endpoint.
func TestSwitchSetEndpoints(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	// get non partitioned members endpoints
	eps := []string{clus.Members[1].GRPCURL(), clus.Members[2].GRPCURL()}

	cli := clus.Client(0)
	clus.Members[0].InjectPartition(t, clus.Members[1:]...)

	cli.SetEndpoints(eps...)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cli.Get(ctx, "foo"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectOldCluster(t *testing.T) {
	integration2.BeforeTest(t)
	// 2 endpoints to test multi-endpoint Status
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 2})
	defer clus.Terminate(t)

	cfg := clientv3.Config{
		Endpoints:        []string{clus.Members[0].GRPCURL(), clus.Members[1].GRPCURL()},
		DialTimeout:      5 * time.Second,
		DialOptions:      []grpc.DialOption{grpc.WithBlock()},
		RejectOldCluster: true,
	}
	cli, err := integration2.NewClient(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cli.Close()
}

// TestDialForeignEndpoint checks an endpoint that is not registered
// with the balancer can be dialed.
func TestDialForeignEndpoint(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 2})
	defer clus.Terminate(t)

	conn, err := clus.Client(0).Dial(clus.Client(1).Endpoints()[0])
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// grpc can return a lazy connection that's not connected yet; confirm
	// that it can communicate with the cluster.
	kvc := clientv3.NewKVFromKVClient(pb.NewKVClient(conn), clus.Client(0))
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	if _, gerr := kvc.Get(ctx, "abc"); gerr != nil {
		t.Fatal(err)
	}
}

// TestSetEndpointAndPut checks that a Put following a SetEndpoints
// to a working endpoint will always succeed.
func TestSetEndpointAndPut(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 2})
	defer clus.Terminate(t)

	clus.Client(1).SetEndpoints(clus.Members[0].GRPCURL())
	_, err := clus.Client(1).Put(context.TODO(), "foo", "bar")
	if err != nil && !strings.Contains(err.Error(), "closing") {
		t.Fatal(err)
	}
}
