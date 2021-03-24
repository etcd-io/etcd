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
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"go.etcd.io/etcd/pkg/v3/grpc_testing"
	"go.etcd.io/etcd/tests/v3/integration"

	"google.golang.org/grpc"
	testpb "google.golang.org/grpc/test/grpc_testing"
)

// This test mimics scenario described in grpc_naming.md doc.

func TestEtcdGrpcResolver(t *testing.T) {
	integration.BeforeTest(t)

	s1PayloadBody := []byte{'1'}
	s1 := grpc_testing.NewDummyStubServer(s1PayloadBody)
	if err := s1.Start(nil); err != nil {
		t.Fatal("failed to start dummy grpc server (s1)", err)
	}
	defer s1.Stop()

	s2PayloadBody := []byte{'2'}
	s2 := grpc_testing.NewDummyStubServer(s2PayloadBody)
	if err := s2.Start(nil); err != nil {
		t.Fatal("failed to start dummy grpc server (s2)", err)
	}
	defer s2.Stop()

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
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

	b, err := resolver.NewBuilder(clus.Client(1))
	if err != nil {
		t.Fatal("failed to new resolver builder", err)
	}

	conn, err := grpc.Dial("etcd:///foo", grpc.WithInsecure(), grpc.WithResolvers(b))
	if err != nil {
		t.Fatal("failed to connect to foo", err)
	}
	defer conn.Close()

	c := testpb.NewTestServiceClient(conn)
	resp, err := c.UnaryCall(context.TODO(), &testpb.SimpleRequest{}, grpc.WaitForReady(true))
	if err != nil {
		t.Fatal("failed to invoke rpc to foo (e1)", err)
	}
	if resp.GetPayload() == nil || !bytes.Equal(resp.GetPayload().GetBody(), s1PayloadBody) {
		t.Fatalf("unexpected response from foo (e1): %s", resp.GetPayload().GetBody())
	}

	em.DeleteEndpoint(context.TODO(), "foo/e1")
	em.AddEndpoint(context.TODO(), "foo/e2", e2)

	// We use a loop with deadline of 30s to avoid test getting flake
	// as it's asynchronous for gRPC Client to update underlying connections.
	maxRetries := 300
	retryPeriod := 100 * time.Millisecond
	retries := 0
	for {
		time.Sleep(retryPeriod)
		retries++

		resp, err = c.UnaryCall(context.TODO(), &testpb.SimpleRequest{})
		if err != nil {
			if retries < maxRetries {
				continue
			}
			t.Fatal("failed to invoke rpc to foo (e2)", err)
		}
		if resp.GetPayload() == nil || !bytes.Equal(resp.GetPayload().GetBody(), s2PayloadBody) {
			if retries < maxRetries {
				continue
			}
			t.Fatalf("unexpected response from foo (e2): %s", resp.GetPayload().GetBody())
		}
		break
	}
}
