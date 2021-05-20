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
	"testing"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/tests/v3/integration"
)

func TestRoleError(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authapi := clus.RandClient()

	_, err := authapi.RoleAdd(context.TODO(), "test-role")
	if err != nil {
		t.Fatal(err)
	}

	_, err = authapi.RoleAdd(context.TODO(), "test-role")
	if err != rpctypes.ErrRoleAlreadyExist {
		t.Fatalf("expected %v, got %v", rpctypes.ErrRoleAlreadyExist, err)
	}

	_, err = authapi.RoleAdd(context.TODO(), "")
	if err != rpctypes.ErrRoleEmpty {
		t.Fatalf("expected %v, got %v", rpctypes.ErrRoleEmpty, err)
	}
}
