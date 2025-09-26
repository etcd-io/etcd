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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// Test basic Put/Get/Delete
func TestKV_PutGetDelete(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()

			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			client := testutils.MustClient(clus.Client())

			key, value := "foo", "bar"
			// Put
			err := client.Put(ctx, key, value, config.PutOptions{})
			require.NoError(t, err)

			// Get
			getResp, err := client.Get(ctx, key, config.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, 1, len(getResp.Kvs))
			require.Equal(t, value, string(getResp.Kvs[0].Value))

			// Delete
			delResp, err := client.Delete(ctx, key, config.DeleteOptions{})
			require.NoError(t, err)
			require.Equal(t, int64(1), delResp.Deleted)
		})
	}
}

// Test Get with prefix, sort, limit, count-only, keys-only
func TestKV_GetVariants(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			client := testutils.MustClient(clus.Client())

			kvs := []struct{ key, val string }{
				{"key1", "val1"},
				{"key2", "val2"},
				{"key3", "val3"},
			}
			for _, kv := range kvs {
				err := client.Put(ctx, kv.key, kv.val, config.PutOptions{})
				require.NoError(t, err)
			}

			// Get key1
			getResp, err := client.Get(ctx, "key1", config.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, "val1", string(getResp.Kvs[0].Value))

			// Get with prefix
			getResp, err = client.Get(ctx, "key", config.GetOptions{Prefix: true})
			require.NoError(t, err)
			require.Equal(t, 3, len(getResp.Kvs))

			// Get keys only
			getResp, err = client.Get(ctx, "key", config.GetOptions{Prefix: true, KeysOnly: true})
			require.NoError(t, err)
			for _, kv := range getResp.Kvs {
				require.Empty(t, kv.Value)
			}

			// Get count only
			getResp, err = client.Get(ctx, "key", config.GetOptions{Prefix: true, CountOnly: true})
			require.NoError(t, err)
			require.Equal(t, int64(3), getResp.Count)

			// Get with limit
			getResp, err = client.Get(ctx, "key", config.GetOptions{Prefix: true, Limit: 2})
			require.NoError(t, err)
			require.Equal(t, 2, len(getResp.Kvs))
		})
	}
}

// Test Delete with prefix
func TestKV_DeleteVariants(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			client := testutils.MustClient(clus.Client())

			kvs := []struct{ key, val string }{
				{"foo1", "bar"},
				{"foo2", "bar"},
				{"foo3", "bar"},
			}
			for _, kv := range kvs {
				err := client.Put(ctx, kv.key, kv.val, config.PutOptions{})
				require.NoError(t, err)
			}

			// Delete with prefix
			delResp, err := client.Delete(ctx, "foo", config.DeleteOptions{Prefix: true})
			require.NoError(t, err)
			require.Equal(t, int64(3), delResp.Deleted)
		})
	}
}

// Test Get with revision (simulate updates)
func TestKV_GetRev(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			client := testutils.MustClient(clus.Client())

			// Put same key multiple times
			var revs []int64
			key := "key"
			vals := []string{"val1", "val2", "val3"}
			for _, v := range vals {
				err := client.Put(ctx, key, v, config.PutOptions{})
				require.NoError(t, err)
				// Get the revision after the put
				getResp, err := client.Get(ctx, key, config.GetOptions{})
				require.NoError(t, err)
				require.Len(t, getResp.Kvs, 1)
				revs = append(revs, getResp.Kvs[0].ModRevision)
			}

			// Get with rev
			for i, rev := range revs {
				getResp, err := client.Get(ctx, key, config.GetOptions{Revision: int(rev)})
				require.NoError(t, err)
				require.Equal(t, vals[i], string(getResp.Kvs[0].Value))
			}
		})
	}
}
