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

package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestAuthCluster(t *testing.T) {
	e2e.BeforeTest(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithSnapshotCount(2),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("could not close test cluster (%v)", err)
		}
	}()

	epcClient := epc.Etcdctl()
	createUsers(ctx, t, epcClient)

	if err := epcClient.AuthEnable(ctx); err != nil {
		t.Fatalf("could not enable Auth: (%v)", err)
	}

	testUserClientOpts := e2e.WithAuth("test", "testPassword")
	rootUserClientOpts := e2e.WithAuth("root", "rootPassword")

	// write more than SnapshotCount keys to single leader to make sure snapshot is created
	for i := 0; i <= 10; i++ {
		if err := epc.Etcdctl(testUserClientOpts).Put(ctx, fmt.Sprintf("/test/%d", i), "test", config.PutOptions{}); err != nil {
			t.Fatalf("failed to Put (%v)", err)
		}
	}

	// start second process
	if _, err := epc.StartNewProc(ctx, nil, t, false /* addAsLearner */, rootUserClientOpts); err != nil {
		t.Fatalf("could not start second etcd process (%v)", err)
	}

	// make sure writes to both endpoints are successful
	endpoints := epc.EndpointsGRPC()
	assert.Len(t, endpoints, 2)
	for _, endpoint := range epc.EndpointsGRPC() {
		if err := epc.Etcdctl(testUserClientOpts, e2e.WithEndpoints([]string{endpoint})).Put(ctx, "/test/key", endpoint, config.PutOptions{}); err != nil {
			t.Fatalf("failed to write to Put to %q (%v)", endpoint, err)
		}
	}

	// verify all nodes have exact same revision and hash
	assert.Eventually(t, func() bool {
		hashKvs, err := epc.Etcdctl(rootUserClientOpts).HashKV(ctx, 0)
		if err != nil {
			t.Logf("failed to get HashKV: %v", err)
			return false
		}
		if len(hashKvs) != 2 {
			t.Logf("not exactly 2 hashkv responses returned: %d", len(hashKvs))
			return false
		}
		if hashKvs[0].Header.Revision != hashKvs[1].Header.Revision {
			t.Logf("The two members' revision (%d, %d) are not equal", hashKvs[0].Header.Revision, hashKvs[1].Header.Revision)
			return false
		}
		assert.Equal(t, hashKvs[0].Hash, hashKvs[1].Hash)
		return true
	}, time.Second*5, time.Millisecond*100)
}

func applyTLSWithRootCommonName() func() {
	var (
		oldCertPath       = e2e.CertPath
		oldPrivateKeyPath = e2e.PrivateKeyPath
		oldCaPath         = e2e.CaPath

		newCertPath       = filepath.Join(e2e.FixturesDir, "CommonName-root.crt")
		newPrivateKeyPath = filepath.Join(e2e.FixturesDir, "CommonName-root.key")
		newCaPath         = filepath.Join(e2e.FixturesDir, "CommonName-root.crt")
	)

	e2e.CertPath = newCertPath
	e2e.PrivateKeyPath = newPrivateKeyPath
	e2e.CaPath = newCaPath

	return func() {
		e2e.CertPath = oldCertPath
		e2e.PrivateKeyPath = oldPrivateKeyPath
		e2e.CaPath = oldCaPath
	}
}

func createUsers(ctx context.Context, t *testing.T, client *e2e.EtcdctlV3) {
	if _, err := client.UserAdd(ctx, "root", "rootPassword", config.UserAddOptions{}); err != nil {
		t.Fatalf("could not add root user (%v)", err)
	}
	if _, err := client.RoleAdd(ctx, "root"); err != nil {
		t.Fatalf("could not create 'root' role (%v)", err)
	}
	if _, err := client.UserGrantRole(ctx, "root", "root"); err != nil {
		t.Fatalf("could not grant root role to root user (%v)", err)
	}

	if _, err := client.RoleAdd(ctx, "test"); err != nil {
		t.Fatalf("could not create 'test' role (%v)", err)
	}
	if _, err := client.RoleGrantPermission(ctx, "test", "/test/", "/test0", clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
		t.Fatalf("could not RoleGrantPermission (%v)", err)
	}
	if _, err := client.UserAdd(ctx, "test", "testPassword", config.UserAddOptions{}); err != nil {
		t.Fatalf("could not add user test (%v)", err)
	}
	if _, err := client.UserGrantRole(ctx, "test", "test"); err != nil {
		t.Fatalf("could not grant test role user (%v)", err)
	}
}
