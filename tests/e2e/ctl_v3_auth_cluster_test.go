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
	"github.com/stretchr/testify/require"

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
	require.NoErrorf(t, err, "could not start etcd process cluster (%v)", err)
	defer func() {
		require.NoErrorf(t, epc.Close(), "could not close test cluster")
	}()

	epcClient := epc.Etcdctl()
	createUsers(ctx, t, epcClient)

	require.NoErrorf(t, epcClient.AuthEnable(ctx), "could not enable Auth")

	testUserClientOpts := e2e.WithAuth("test", "testPassword")
	rootUserClientOpts := e2e.WithAuth("root", "rootPassword")

	// write more than SnapshotCount keys to single leader to make sure snapshot is created
	for i := 0; i <= 10; i++ {
		require.NoErrorf(t, epc.Etcdctl(testUserClientOpts).Put(ctx, fmt.Sprintf("/test/%d", i), "test", config.PutOptions{}), "failed to Put")
	}

	// start second process
	_, err = epc.StartNewProc(ctx, nil, t, false /* addAsLearner */, rootUserClientOpts)
	require.NoErrorf(t, err, "could not start second etcd process")

	// make sure writes to both endpoints are successful
	endpoints := epc.EndpointsGRPC()
	assert.Len(t, endpoints, 2)
	for _, endpoint := range epc.EndpointsGRPC() {
		require.NoErrorf(t, epc.Etcdctl(testUserClientOpts, e2e.WithEndpoints([]string{endpoint})).Put(ctx, "/test/key", endpoint, config.PutOptions{}), "failed to write to Put to %q", endpoint)
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
	_, err := client.UserAdd(ctx, "root", "rootPassword", config.UserAddOptions{})
	require.NoErrorf(t, err, "could not add root user")
	_, err = client.RoleAdd(ctx, "root")
	require.NoErrorf(t, err, "could not create 'root' role")
	_, err = client.UserGrantRole(ctx, "root", "root")
	require.NoErrorf(t, err, "could not grant root role to root user")

	_, err = client.RoleAdd(ctx, "test")
	require.NoErrorf(t, err, "could not create 'test' role")
	_, err = client.RoleGrantPermission(ctx, "test", "/test/", "/test0", clientv3.PermissionType(clientv3.PermReadWrite))
	require.NoErrorf(t, err, "could not RoleGrantPermission")
	_, err = client.UserAdd(ctx, "test", "testPassword", config.UserAddOptions{})
	require.NoErrorf(t, err, "could not add user test")
	_, err = client.UserGrantRole(ctx, "test", "test")
	require.NoErrorf(t, err, "could not grant test role user")
}
