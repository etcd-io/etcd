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

package clientv3test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"go.etcd.io/etcd/tests/v3/integration"
	etcd "go.etcd.io/etcd/v3/clientv3"
	namingv3 "go.etcd.io/etcd/v3/clientv3/naming"
	"go.etcd.io/etcd/v3/pkg/testutil"

	"google.golang.org/grpc/naming"
)

func TestGRPCResolver(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	r := namingv3.GRPCResolver{
		Client: clus.RandClient(),
	}

	w, err := r.Resolve("foo")
	if err != nil {
		t.Fatal("failed to resolve foo", err)
	}
	defer w.Close()

	addOp := naming.Update{Op: naming.Add, Addr: "127.0.0.1", Metadata: "metadata"}
	err = r.Update(context.TODO(), "foo", addOp)
	if err != nil {
		t.Fatal("failed to add foo", err)
	}

	us, err := w.Next()
	if err != nil {
		t.Fatal("failed to get udpate", err)
	}

	wu := &naming.Update{
		Op:       naming.Add,
		Addr:     "127.0.0.1",
		Metadata: "metadata",
	}

	if !reflect.DeepEqual(us[0], wu) {
		t.Fatalf("up = %#v, want %#v", us[0], wu)
	}

	delOp := naming.Update{Op: naming.Delete, Addr: "127.0.0.1"}
	err = r.Update(context.TODO(), "foo", delOp)
	if err != nil {
		t.Fatalf("failed to udpate %v", err)
	}

	us, err = w.Next()
	if err != nil {
		t.Fatalf("failed to get udpate %v", err)
	}

	wu = &naming.Update{
		Op:       naming.Delete,
		Addr:     "127.0.0.1",
		Metadata: "metadata",
	}

	if !reflect.DeepEqual(us[0], wu) {
		t.Fatalf("up = %#v, want %#v", us[0], wu)
	}
}

// TestGRPCResolverMulti ensures the resolver will initialize
// correctly with multiple hosts and correctly receive multiple
// updates in a single revision.
func TestGRPCResolverMulti(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)
	c := clus.RandClient()

	v, verr := json.Marshal(naming.Update{Addr: "127.0.0.1", Metadata: "md"})
	if verr != nil {
		t.Fatal(verr)
	}
	if _, err := c.Put(context.TODO(), "foo/host", string(v)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Put(context.TODO(), "foo/host2", string(v)); err != nil {
		t.Fatal(err)
	}

	r := namingv3.GRPCResolver{Client: c}

	w, err := r.Resolve("foo")
	if err != nil {
		t.Fatal("failed to resolve foo", err)
	}
	defer w.Close()

	updates, nerr := w.Next()
	if nerr != nil {
		t.Fatal(nerr)
	}
	if len(updates) != 2 {
		t.Fatalf("expected two updates, got %+v", updates)
	}

	_, err = c.Txn(context.TODO()).Then(etcd.OpDelete("foo/host"), etcd.OpDelete("foo/host2")).Commit()
	if err != nil {
		t.Fatal(err)
	}

	updates, nerr = w.Next()
	if nerr != nil {
		t.Fatal(nerr)
	}
	if len(updates) != 2 || (updates[0].Op != naming.Delete && updates[1].Op != naming.Delete) {
		t.Fatalf("expected two updates, got %+v", updates)
	}
}
