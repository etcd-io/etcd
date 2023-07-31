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

package naming_test

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	testpb "google.golang.org/grpc/interop/grpc_testing"

	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"go.etcd.io/etcd/pkg/v3/grpc_testing"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func testEtcdGrpcResolver(t *testing.T, lbPolicy string) {

	// Setup two new dummy stub servers
	payloadBody := []byte{'1'}
	s1 := grpc_testing.NewDummyStubServer(payloadBody)
	if err := s1.Start(nil); err != nil {
		t.Fatal("failed to start dummy grpc server (s1)", err)
	}
	defer s1.Stop()

	s2 := grpc_testing.NewDummyStubServer(payloadBody)
	if err := s2.Start(nil); err != nil {
		t.Fatal("failed to start dummy grpc server (s2)", err)
	}
	defer s2.Stop()

	// Create new cluster with endpoint manager with two endpoints
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	em, err := endpoints.NewManager(clus.Client(0), "foo")
	if err != nil {
		t.Fatal("failed to create EndpointManager", err)
	}

	e1 := endpoints.Endpoint{Addr: s1.Addr()}
	e2 := endpoints.Endpoint{Addr: s2.Addr()}

	err = em.AddEndpoint(context.TODO(), "foo/e1", e1)
	if err != nil {
		t.Fatal("failed to add foo", err)
	}

	err = em.AddEndpoint(context.TODO(), "foo/e2", e2)
	if err != nil {
		t.Fatal("failed to add foo", err)
	}

	b, err := resolver.NewBuilder(clus.Client(1))
	if err != nil {
		t.Fatal("failed to new resolver builder", err)
	}

	// Create connection with provided lb policy
	conn, err := grpc.Dial("etcd:///foo", grpc.WithInsecure(), grpc.WithResolvers(b),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, lbPolicy)))
	if err != nil {
		t.Fatal("failed to connect to foo", err)
	}
	defer conn.Close()

	// Send an initial request that should go to e1
	c := testpb.NewTestServiceClient(conn)
	resp, err := c.UnaryCall(context.TODO(), &testpb.SimpleRequest{}, grpc.WaitForReady(true))
	if err != nil {
		t.Fatal("failed to invoke rpc to foo (e1)", err)
	}
	if resp.GetPayload() == nil || !bytes.Equal(resp.GetPayload().GetBody(), payloadBody) {
		t.Fatalf("unexpected response from foo (e1): %s", resp.GetPayload().GetBody())
	}

	// Send more requests
	lastResponse := []byte{'1'}
	totalRequests := 3500
	for i := 1; i < totalRequests; i++ {
		resp, err := c.UnaryCall(context.TODO(), &testpb.SimpleRequest{}, grpc.WaitForReady(true))
		if err != nil {
			t.Fatal("failed to invoke rpc to foo", err)
		}

		t.Logf("Response: %v", string(resp.GetPayload().GetBody()))

		if resp.GetPayload() == nil {
			t.Fatalf("unexpected response from foo: %s", resp.GetPayload().GetBody())
		}
		lastResponse = resp.GetPayload().GetBody()
	}

	// If the load balancing policy is pick first then return payload should equal number of requests
	t.Logf("Last response: %v", string(lastResponse))
	if lbPolicy == "pick_first" {
		if string(lastResponse) != "3500" {
			t.Fatalf("unexpected total responses from foo: %s", string(lastResponse))
		}
	}

	// If the load balancing policy is round robin we should see roughly half total requests served by each server
	if lbPolicy == "round_robin" {
		responses, err := strconv.Atoi(string(lastResponse))
		if err != nil {
			t.Fatalf("couldn't convert to int: %s", string(lastResponse))
		}

		// Allow 25% tolerance as round robin is not perfect and we don't want the test to flake
		expected := float64(totalRequests) * 0.5
		assert.InEpsilon(t, float64(expected), float64(responses), 0.25, "unexpected total responses from foo: %s", string(lastResponse))
	}
}

// TestEtcdGrpcResolverPickFirst mimics scenarios described in grpc_naming.md doc.
func TestEtcdGrpcResolverPickFirst(t *testing.T) {

	integration2.BeforeTest(t)

	// Pick first is the default load balancer policy for grpc-go
	testEtcdGrpcResolver(t, "pick_first")
}

// TestEtcdGrpcResolverRoundRobin mimics scenarios described in grpc_naming.md doc.
func TestEtcdGrpcResolverRoundRobin(t *testing.T) {

	integration2.BeforeTest(t)

	// Round robin is a common alternative for more production oriented scenarios
	testEtcdGrpcResolver(t, "round_robin")
}

func TestEtcdEndpointManager(t *testing.T) {
	integration2.BeforeTest(t)

	s1PayloadBody := []byte{'1'}
	s1 := grpc_testing.NewDummyStubServer(s1PayloadBody)
	err := s1.Start(nil)
	assert.NoError(t, err)
	defer s1.Stop()

	s2PayloadBody := []byte{'2'}
	s2 := grpc_testing.NewDummyStubServer(s2PayloadBody)
	err = s2.Start(nil)
	assert.NoError(t, err)
	defer s2.Stop()

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	// Check if any endpoint with the same prefix "foo" will not break the logic with multiple endpoints
	em, err := endpoints.NewManager(clus.Client(0), "foo")
	assert.NoError(t, err)
	emOther, err := endpoints.NewManager(clus.Client(1), "foo_other")
	assert.NoError(t, err)

	e1 := endpoints.Endpoint{Addr: s1.Addr()}
	e2 := endpoints.Endpoint{Addr: s2.Addr()}

	em.AddEndpoint(context.Background(), "foo/e1", e1)
	emOther.AddEndpoint(context.Background(), "foo_other/e2", e2)

	epts, err := em.List(context.Background())
	assert.NoError(t, err)
	eptsOther, err := emOther.List(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(epts), 1)
	assert.Equal(t, len(eptsOther), 1)
}
