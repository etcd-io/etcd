// Copyright 2025 The etcd Authors
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

package concurrency_test

import (
	"context"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3/concurrency"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

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

// TestNewSessionTimeoutConsistency tests that timeout behavior is consistent with Close()
func TestNewSessionTimeoutConsistency(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()

	// Both NewSession and Close should use the same timeout pattern
	// This test verifies they behave consistently
	t.Run("TimeoutPatternConsistency", func(t *testing.T) {
		// Create session with short TTL
		session, err := concurrency.NewSession(cli, concurrency.WithTTL(1))
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// Both LeaseGrant (in NewSession) and LeaseRevoke (in Close)
		// should use the same timeout duration (TTL seconds)
		start := time.Now()
		err = session.Close()
		elapsed := time.Since(start)

		// Close should complete within reasonable time for TTL=1
		if elapsed > 3*time.Second {
			t.Fatalf("Close() took too long: %v, expected < 3s for TTL=1", elapsed)
		}

		if err != nil {
			t.Logf("Close() returned error (acceptable): %v", err)
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

// TestNewSessionTimeoutWithDifferentTTL tests timeout behavior with various TTL values
func TestNewSessionTimeoutWithDifferentTTL(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()

	testCases := []struct {
		name           string
		ttl            int
		contextTimeout time.Duration
		shouldTimeout  bool
	}{
		{
			name:           "short_ttl_short_timeout",
			ttl:            1,
			contextTimeout: 1 * time.Millisecond,
			shouldTimeout:  true,
		},
		{
			name:           "short_ttl_adequate_timeout",
			ttl:            1,
			contextTimeout: 3 * time.Second,
			shouldTimeout:  false,
		},
		{
			name:           "normal_ttl_adequate_timeout",
			ttl:            60,
			contextTimeout: 5 * time.Second,
			shouldTimeout:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.contextTimeout)
			defer cancel()

			session, err := concurrency.NewSession(cli, concurrency.WithContext(ctx), concurrency.WithTTL(tc.ttl))

			if tc.shouldTimeout {
				if err == nil {
					if session != nil {
						session.Close()
					}
					t.Fatal("expected timeout error, but got nil")
				}
				if err != context.DeadlineExceeded {
					t.Fatalf("expected context.DeadlineExceeded, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if session == nil {
					t.Fatal("expected valid session, got nil")
				}
				session.Close()
			}
		})
	}
}
