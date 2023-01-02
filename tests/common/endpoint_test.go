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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestEndpointStatus(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t)
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		_, err := cc.Status(ctx)
		if err != nil {
			t.Fatalf("get endpoint status error: %v", err)
		}
	})
}

func TestEndpointHashKV(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t)
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())

	t.Log("Add some entries")
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := cc.Put(ctx, key, value, config.PutOptions{}); err != nil {
			t.Fatalf("count not put key %q, err: %s", key, err)
		}
	}

	t.Log("Check all members' Hash and HashRevision")
	require.Eventually(t, func() bool {
		resp, err := cc.HashKV(ctx, 0)
		require.NoError(t, err, "failed to get endpoint hashkv: %v", err)

		require.Equal(t, 3, len(resp))
		if resp[0].HashRevision == resp[1].HashRevision && resp[0].HashRevision == resp[2].HashRevision {
			require.Equal(t, resp[0].Hash, resp[1].Hash)
			require.Equal(t, resp[0].Hash, resp[2].Hash)
			return true
		}
		t.Logf("HashRevisions are not equal: [%d, %d, %d], retry...", resp[0].HashRevision, resp[1].HashRevision, resp[2].HashRevision)
		return false
	}, 5*time.Second, 200*time.Millisecond)
}

func TestEndpointHealth(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t)
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		if err := cc.Health(ctx); err != nil {
			t.Fatalf("get endpoint health error: %v", err)
		}
	})
}
