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

package concurrency_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestSessionOptions(t *testing.T) {
	cli, err := integration.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	require.NoError(t, err)
	defer cli.Close()
	lease, err := cli.Grant(t.Context(), 100)
	require.NoError(t, err)
	s, err := concurrency.NewSession(cli, concurrency.WithLease(lease.ID))
	require.NoError(t, err)
	defer s.Close()
	assert.Equal(t, s.Lease(), lease.ID)

	go s.Orphan()
	select {
	case <-s.Done():
	case <-time.After(time.Millisecond * 100):
		t.Fatal("session did not get orphaned as expected")
	}
}

func TestSessionTTLOptions(t *testing.T) {
	cli, err := integration.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	require.NoError(t, err)
	defer cli.Close()

	setTTL := 90
	s, err := concurrency.NewSession(cli, concurrency.WithTTL(setTTL))
	require.NoError(t, err)
	defer s.Close()

	leaseID := s.Lease()
	// TTL retrieved should be less than the set TTL, but not equal to default:60 or exprired:-1
	resp, err := cli.Lease.TimeToLive(t.Context(), leaseID)
	if err != nil {
		t.Log(err)
	}
	if resp.TTL == -1 {
		t.Errorf("client lease should not be expired: %d", resp.TTL)
	}
	if resp.TTL == 60 {
		t.Errorf("default TTL value is used in the session, instead of set TTL: %d", setTTL)
	}
	if resp.TTL >= int64(setTTL) || resp.TTL < int64(setTTL)-20 {
		t.Errorf("Session TTL from lease should be less, but close to set TTL %d, have: %d", setTTL, resp.TTL)
	}
}

func TestSessionCtx(t *testing.T) {
	cli, err := integration.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	require.NoError(t, err)
	defer cli.Close()
	lease, err := cli.Grant(t.Context(), 100)
	require.NoError(t, err)
	s, err := concurrency.NewSession(cli, concurrency.WithLease(lease.ID))
	require.NoError(t, err)
	defer s.Close()
	assert.Equal(t, s.Lease(), lease.ID)

	childCtx, cancel := context.WithCancel(s.Ctx())
	defer cancel()

	go s.Orphan()
	select {
	case <-childCtx.Done():
	case <-time.After(time.Millisecond * 100):
		t.Fatal("child context of session context is not canceled")
	}
	assert.Equal(t, childCtx.Err(), context.Canceled)
}

// TestNewSessionLeaseGrantTimeout tests that NewSession respects timeout when creating a lease
func TestNewSessionLeaseGrantTimeout(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()

	// Test case 1: Very short timeout should fail
	t.Run("ShortTimeoutShouldFail", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		start := time.Now()
		session, err := concurrency.NewSession(cli, concurrency.WithContext(ctx), concurrency.WithTTL(5))
		elapsed := time.Since(start)

		if err == nil {
			if session != nil {
				session.Close()
			}
			t.Fatal("expected timeout error, but got nil")
		}

		if err != context.DeadlineExceeded {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}

		// Should fail quickly (within reasonable time)
		if elapsed > 100*time.Millisecond {
			t.Fatalf("timeout took too long: %v", elapsed)
		}
	})

	// Test case 2: Adequate timeout should succeed
	t.Run("AdequateTimeoutShouldSucceed", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		session, err := concurrency.NewSession(cli, concurrency.WithContext(ctx), concurrency.WithTTL(5))
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if session == nil {
			t.Fatal("expected valid session, got nil")
		}
		defer session.Close()

		// Verify session has a valid lease
		if session.Lease() == 0 {
			t.Fatal("session should have a valid lease ID")
		}
	})
}

// TestNewSessionNormalOperationAfterFix tests that normal session creation still works
func TestNewSessionNormalOperationAfterFix(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()

	// Verify normal session operations work correctly after the timeout fix
	session, err := concurrency.NewSession(cli, concurrency.WithTTL(60))
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Session should be immediately usable
	if session.Lease() == 0 {
		t.Fatal("session should have a valid lease")
	}

	// Keep-alive should be working
	select {
	case <-session.Done():
		t.Fatal("session should not be done immediately")
	case <-time.After(100 * time.Millisecond):
		// Expected - session should remain alive
	}

	// Should be able to use session for mutex
	mutex := concurrency.NewMutex(session, "test-mutex")
	if mutex == nil {
		t.Fatal("should be able to create mutex with session")
	}
}

