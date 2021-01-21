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

package naming

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	"go.etcd.io/etcd/clientv3/naming/endpoints"
	"go.etcd.io/etcd/clientv3/naming/resolver"
	"go.etcd.io/etcd/integration"
	"go.etcd.io/etcd/pkg/testutil"
)

// This test mimics scenario described in grpc_naming.md doc.

func TestEtcdGrpcResolver(t *testing.T) {
	t.Skip("Not implemented yet")

	defer testutil.AfterTest(t)

	// s1 :=  // TODO: Dummy GRPC service listening on 127.0.0.1:20000
	// s2 :=  // TODO: Dummy GRPC service listening on 127.0.0.1:20001

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	em, err := endpoints.NewManager(clus.RandClient(), "foo")
	if err != nil {
		t.Fatal("failed to create EndpointManager", err)
	}

	e1 := endpoints.Endpoint{Addr: "127.0.0.1:20000"}
	e2 := endpoints.Endpoint{Addr: "127.0.0.1:20001"}

	err = em.AddEndpoint(context.TODO(), "foo/e1", e1)
	if err != nil {
		t.Fatal("failed to add foo", err)
	}
	etcdResolver, err := resolver.NewBuilder(clus.RandClient())

	conn, err := grpc.Dial("etc://foo", grpc.WithResolvers(etcdResolver))
	if err != nil {
		t.Fatal("failed to connect to foo (e1)", err)
	}

	// TODO: send requests to conn, ensure s1 received it.

	em.DeleteEndpoint(context.TODO(), "foo/e1")
	em.AddEndpoint(context.TODO(), "foo/e2", e2)

	// TODO: Send requests to conn and make sure s2 receive it.
	// Might require restarting s1 to break the existing (open) connection.

	conn.GetState() // this line is to avoid compiler warning that conn is unused.
}
