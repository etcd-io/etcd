// Copyright 2017 The etcd Authors
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
	"time"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	lockpb "go.etcd.io/etcd/server/v3/etcdserver/api/v3lock/v3lockpb"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestV3LockLockWaiter tests that a client will wait for a lock, then acquire it
// once it is unlocked.
func TestV3LockLockWaiter(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	lease1, err1 := integration.ToGRPC(clus.RandClient()).Lease.LeaseGrant(t.Context(), &pb.LeaseGrantRequest{TTL: 30})
	require.NoError(t, err1)
	lease2, err2 := integration.ToGRPC(clus.RandClient()).Lease.LeaseGrant(t.Context(), &pb.LeaseGrantRequest{TTL: 30})
	require.NoError(t, err2)

	lc := integration.ToGRPC(clus.Client(0)).Lock
	l1, lerr1 := lc.Lock(t.Context(), &lockpb.LockRequest{Name: []byte("foo"), Lease: lease1.ID})
	require.NoError(t, lerr1)

	lockc := make(chan struct{})
	go func() {
		l2, lerr2 := lc.Lock(t.Context(), &lockpb.LockRequest{Name: []byte("foo"), Lease: lease2.ID})
		if lerr2 != nil {
			t.Error(lerr2)
		}
		if l1.Header.Revision >= l2.Header.Revision {
			t.Errorf("expected l1 revision < l2 revision, got %d >= %d", l1.Header.Revision, l2.Header.Revision)
		}
		close(lockc)
	}()

	select {
	case <-time.After(200 * time.Millisecond):
	case <-lockc:
		t.Fatalf("locked before unlock")
	}

	_, uerr := lc.Unlock(t.Context(), &lockpb.UnlockRequest{Key: l1.Key})
	require.NoError(t, uerr)

	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("waiter did not lock after unlock")
	case <-lockc:
	}
}

func TestV3LockLockAuthTokenExpiration(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:      1,
		AuthToken: integration.DefaultTokenJWT,
	})
	defer clus.Terminate(t)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	rootc1, cerr1 := integration.NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, cerr1)
	defer rootc1.Close()

	authResp, aerr := rootc1.Authenticate(t.Context(), "root", "123")
	require.NoError(t, aerr)

	rootc2, cerr2 := integration.NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Token:     authResp.Token,
	})
	require.NoError(t, cerr2)
	defer rootc2.Close()

	lease1, err1 := rootc1.Grant(t.Context(), 30)
	require.NoError(t, err1)
	lease2, err2 := rootc2.Grant(t.Context(), 30)
	require.NoError(t, err2)

	lc1 := integration.ToGRPC(rootc1).Lock
	lc2 := integration.ToGRPC(rootc2).Lock

	req1 := &lockpb.LockRequest{Name: []byte("foo"), Lease: int64(lease1.ID)}
	l1, lerr1 := lc1.Lock(t.Context(), req1)
	require.NoError(t, lerr1)

	lockc := make(chan error, 1)
	go func() {
		req2 := &lockpb.LockRequest{Name: []byte("foo"), Lease: int64(lease2.ID)}
		_, lerr2 := lc2.Lock(t.Context(), req2)
		lockc <- lerr2
	}()

	// Wait 3 seconds so the JWT auth token (ttl=2s) expires while waiter is blocked in Lock
	time.Sleep(3 * time.Second)

	_, uerr := lc1.Unlock(t.Context(), &lockpb.UnlockRequest{Key: l1.Key})
	require.NoError(t, uerr)

	select {
	case err := <-lockc:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatalf("lock waiter timed out after unlock")
	}
}
