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

package common

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestLock(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		testLock(ctx, t, cc)
	})
}

func testLock(ctx context.Context, t *testing.T, cc interfaces.Client) {
	const name = "test-lock"

	holder, err := cc.Lock(ctx, name)
	require.NoError(t, err)
	defer func() { _ = holder.Release() }()

	var l1 string
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timed out locking")
	case res := <-holder.Acquired():
		require.NoError(t, res.Err)
		require.True(t, strings.Contains(res.Name, name), "got %q, expected it to contain %q", res.Name, name)
		l1 = res.Name
	}

	// blocked client that won't acquire the lock
	blocked, err := cc.Lock(ctx, name)
	require.NoError(t, err)
	defer func() { _ = blocked.Release() }()
	select {
	case <-time.After(100 * time.Millisecond):
	case res := <-blocked.Acquired():
		t.Fatalf("should block, got %q", res.Name)
	}

	// another client that will acquire the lock once it gets released
	blockAcquire, err := cc.Lock(ctx, name)
	require.NoError(t, err)
	defer func() { _ = blockAcquire.Release() }()
	select {
	case <-time.After(100 * time.Millisecond):
	case res := <-blockAcquire.Acquired():
		t.Fatalf("should block, got %q", res.Name)
	}

	// stop the blocked waiter; releasing it while it is still waiting for
	// the lock may report an error (e.g. an unexpected exit code)
	_ = blocked.Release()

	// release the holder, blockAcquire should acquire the lock
	require.NoError(t, holder.Release())

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the lock handoff")
	case res := <-blockAcquire.Acquired():
		require.NoError(t, res.Err)
		require.True(t, strings.Contains(res.Name, name), "got %q, expected it to contain %q", res.Name, name)
		require.NotEqual(t, l1, res.Name)
	}
}