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

//go:build !cluster_proxy
// +build !cluster_proxy

package clientv3test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/mirror"
	"go.etcd.io/etcd/tests/v3/integration"
	"google.golang.org/grpc"
)

func TestMirrorSync_Authenticated(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	initialClient := clus.Client(0)

	// Create a user to run the mirror process that only has access to /syncpath
	initialClient.RoleAdd(context.Background(), "syncer")
	initialClient.RoleGrantPermission(context.Background(), "syncer", "/syncpath", clientv3.GetPrefixRangeEnd("/syncpath"), clientv3.PermissionType(clientv3.PermReadWrite))
	initialClient.UserAdd(context.Background(), "syncer", "syncfoo")
	initialClient.UserGrantRole(context.Background(), "syncer", "syncer")

	// Seed /syncpath with some initial data
	_, err := initialClient.KV.Put(context.TODO(), "/syncpath/foo", "bar")
	if err != nil {
		t.Fatal(err)
	}

	// Require authentication
	authSetupRoot(t, initialClient.Auth)

	// Create a client as the `syncer` user.
	cfg := clientv3.Config{
		Endpoints:   initialClient.Endpoints(),
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		Username:    "syncer",
		Password:    "syncfoo",
	}
	syncClient, err := integration.NewClient(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer syncClient.Close()

	// Now run the sync process, create changes, and get the initial sync state
	syncer := mirror.NewSyncer(syncClient, "/syncpath", 0)
	gch, ech := syncer.SyncBase(context.TODO())
	wkvs := []*mvccpb.KeyValue{{Key: []byte("/syncpath/foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1}}

	for g := range gch {
		if !reflect.DeepEqual(g.Kvs, wkvs) {
			t.Fatalf("kv = %v, want %v", g.Kvs, wkvs)
		}
	}

	for e := range ech {
		t.Fatalf("unexpected error %v", e)
	}

	// Start a continuous sync
	wch := syncer.SyncUpdates(context.TODO())

	// Update state
	_, err = syncClient.KV.Put(context.TODO(), "/syncpath/foo", "baz")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the updated state to sync
	select {
	case r := <-wch:
		wkv := &mvccpb.KeyValue{Key: []byte("/syncpath/foo"), Value: []byte("baz"), CreateRevision: 2, ModRevision: 3, Version: 2}
		if !reflect.DeepEqual(r.Events[0].Kv, wkv) {
			t.Fatalf("kv = %v, want %v", r.Events[0].Kv, wkv)
		}
	case <-time.After(time.Second):
		t.Fatal("failed to receive update in one second")
	}
}
