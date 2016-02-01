// Copyright 2016 CoreOS, Inc.
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

package integration

import (
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestLeaseCreate(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	lapi := clientv3.NewLease(clus.RandClient())
	defer lapi.Close()

	kv := clientv3.NewKV(clus.RandClient())

	resp, err := lapi.Create(context.Background(), 10)
	if err != nil {
		t.Errorf("failed to create lease %v", err)
	}

	_, err = kv.Put("foo", "bar", lease.LeaseID(resp.ID))
	if err != nil {
		t.Fatalf("failed to create key with lease %v", err)
	}
}

func TestLeaseRevoke(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	lapi := clientv3.NewLease(clus.RandClient())
	defer lapi.Close()

	kv := clientv3.NewKV(clus.RandClient())

	resp, err := lapi.Create(context.Background(), 10)
	if err != nil {
		t.Errorf("failed to create lease %v", err)
	}

	_, err = lapi.Revoke(context.Background(), lease.LeaseID(resp.ID))
	if err != nil {
		t.Errorf("failed to revoke lease", err)
	}

	_, err = kv.Put("foo", "bar", lease.LeaseID(resp.ID))
	if err != v3rpc.ErrLeaseNotFound {
		t.Fatalf("err = %v, want %v", err, v3rpc.ErrLeaseNotFound)
	}
}
