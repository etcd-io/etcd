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
	"context"
	"reflect"
	"testing"

	etcd "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"

	"go.etcd.io/etcd/pkg/v3/testutil"
	"go.etcd.io/etcd/tests/v3/integration"
)

func TestEndpointManager(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	em, err := endpoints.NewManager(clus.RandClient(), "foo")
	if err != nil {
		t.Fatal("failed to create EndpointManager", err)
	}
	ctx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	w, err := em.NewWatchChannel(ctx)
	if err != nil {
		t.Fatal("failed to establish watch", err)
	}

	e1 := endpoints.Endpoint{Addr: "127.0.0.1", Metadata: "metadata"}
	err = em.AddEndpoint(context.TODO(), "foo/a1", e1)
	if err != nil {
		t.Fatal("failed to add foo", err)
	}

	us := <-w

	if us == nil {
		t.Fatal("failed to get update")
	}

	wu := &endpoints.Update{
		Op:       endpoints.Add,
		Key:      "foo/a1",
		Endpoint: e1,
	}

	if !reflect.DeepEqual(us[0], wu) {
		t.Fatalf("up = %#v, want %#v", us[0], wu)
	}

	err = em.DeleteEndpoint(context.TODO(), "foo/a1")
	if err != nil {
		t.Fatalf("failed to udpate %v", err)
	}

	us = <-w
	if us == nil {
		t.Fatal("failed to get udpate")
	}

	wu = &endpoints.Update{
		Op:  endpoints.Delete,
		Key: "foo/a1",
	}

	if !reflect.DeepEqual(us[0], wu) {
		t.Fatalf("up = %#v, want %#v", us[1], wu)
	}
}

// TestEndpointManagerAtomicity ensures the resolver will initialize
// correctly with multiple hosts and correctly receive multiple
// updates in a single revision.
func TestEndpointManagerAtomicity(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	c := clus.RandClient()
	em, err := endpoints.NewManager(c, "foo")
	if err != nil {
		t.Fatal("failed to create EndpointManager", err)
	}

	err = em.Update(context.TODO(), []*endpoints.UpdateWithOpts{
		endpoints.NewAddUpdateOpts("foo/host", endpoints.Endpoint{Addr: "127.0.0.1:2000"}),
		endpoints.NewAddUpdateOpts("foo/host2", endpoints.Endpoint{Addr: "127.0.0.1:2001"})})
	if err != nil {
		t.Fatal(err)
	}

	ctx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	w, err := em.NewWatchChannel(ctx)
	if err != nil {
		t.Fatal(err)
	}

	updates := <-w
	if len(updates) != 2 {
		t.Fatalf("expected two updates, got %+v", updates)
	}

	_, err = c.Txn(context.TODO()).Then(etcd.OpDelete("foo/host"), etcd.OpDelete("foo/host2")).Commit()
	if err != nil {
		t.Fatal(err)
	}

	updates = <-w
	if len(updates) != 2 || (updates[0].Op != endpoints.Delete && updates[1].Op != endpoints.Delete) {
		t.Fatalf("expected two delete updates, got %+v", updates)
	}
}

func TestEndpointManagerCRUD(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	em, err := endpoints.NewManager(clus.RandClient(), "foo")
	if err != nil {
		t.Fatal("failed to create EndpointManager", err)
	}

	// Add
	k1 := "foo/a1"
	e1 := endpoints.Endpoint{Addr: "127.0.0.1", Metadata: "metadata1"}
	err = em.AddEndpoint(context.TODO(), k1, e1)
	if err != nil {
		t.Fatal("failed to add", k1, err)
	}

	k2 := "foo/a2"
	e2 := endpoints.Endpoint{Addr: "127.0.0.2", Metadata: "metadata2"}
	err = em.AddEndpoint(context.TODO(), k2, e2)
	if err != nil {
		t.Fatal("failed to add", k2, err)
	}

	eps, err := em.List(context.TODO())
	if err != nil {
		t.Fatal("failed to list foo")
	}
	if len(eps) != 2 {
		t.Fatalf("unexpected the number of endpoints: %d", len(eps))
	}
	if !reflect.DeepEqual(eps[k1], e1) {
		t.Fatalf("unexpected endpoints: %s", k1)
	}
	if !reflect.DeepEqual(eps[k2], e2) {
		t.Fatalf("unexpected endpoints: %s", k2)
	}

	// Delete
	err = em.DeleteEndpoint(context.TODO(), k1)
	if err != nil {
		t.Fatal("failed to delete", k2, err)
	}

	eps, err = em.List(context.TODO())
	if err != nil {
		t.Fatal("failed to list foo")
	}
	if len(eps) != 1 {
		t.Fatalf("unexpected the number of endpoints: %d", len(eps))
	}
	if !reflect.DeepEqual(eps[k2], e2) {
		t.Fatalf("unexpected endpoints: %s", k2)
	}

	// Update
	k3 := "foo/a3"
	e3 := endpoints.Endpoint{Addr: "127.0.0.3", Metadata: "metadata3"}
	updates := []*endpoints.UpdateWithOpts{
		{Update: endpoints.Update{Op: endpoints.Add, Key: k3, Endpoint: e3}},
		{Update: endpoints.Update{Op: endpoints.Delete, Key: k2}},
	}
	err = em.Update(context.TODO(), updates)
	if err != nil {
		t.Fatal("failed to update", err)
	}

	eps, err = em.List(context.TODO())
	if err != nil {
		t.Fatal("failed to list foo")
	}
	if len(eps) != 1 {
		t.Fatalf("unexpected the number of endpoints: %d", len(eps))
	}
	if !reflect.DeepEqual(eps[k3], e3) {
		t.Fatalf("unexpected endpoints: %s", k3)
	}
}
