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

package integration

import (
	"testing"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
	"golang.org/x/net/context"
)

// TestAuthInvalidPutTxn ensures that invalid Put, Txn requests
// fail with ErrPermissionDenied.
func TestAuthInvalidPutTxn(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authapi := clientv3.NewAuth(clus.RandClient())

	_, err := authapi.RoleAdd(context.TODO(), "root")
	if err != nil {
		t.Fatal(err)
	}

	_, err = authapi.RoleGrantPermission(context.TODO(), "root", "foo", "", clientv3.PermissionType(clientv3.PermReadWrite))
	if err != nil {
		t.Fatal(err)
	}

	_, err = authapi.UserAdd(context.TODO(), "root", "123")
	if err != nil {
		t.Fatal(err)
	}

	_, err = authapi.UserGrantRole(context.TODO(), "root", "root")
	if err != nil {
		t.Fatal(err)
	}

	_, err = authapi.AuthEnable(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	var cli2 *clientv3.Client
	cli2, err = clus.NewClientWithAuth(0, "root", "123")
	if err != nil {
		t.Fatal(err)
	}
	defer cli2.Close()

	kv := clientv3.NewKV(cli2)
	_, err = kv.Put(context.TODO(), "foo1", "bar")
	if err != rpctypes.ErrPermissionDenied {
		t.Fatalf("unexpected %v, got %v", rpctypes.ErrPermissionDenied, err)
	}

	_, err = kv.Txn(context.TODO()).
		If(clientv3.Compare(clientv3.Value("foo1"), ">", "abc")).
		Then(clientv3.OpPut("foo", "XYZ")).
		Else(clientv3.OpPut("foo", "ABC")).
		Commit()
	if err != rpctypes.ErrPermissionDenied {
		t.Fatalf("unexpected %v, got %v", rpctypes.ErrPermissionDenied, err)
	}
}
