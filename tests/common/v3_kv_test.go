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

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
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

			// Table-driven test for various Get operations
			tcs := []struct {
				name         string
				key          string
				options      config.GetOptions
				validateFunc func(t *testing.T, resp *clientv3.GetResponse)
			}{
				{
					name:    "get single key",
					key:     "key1",
					options: config.GetOptions{},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, 1, len(resp.Kvs))
						require.Equal(t, "val1", string(resp.Kvs[0].Value))
					},
				},
				{
					name:    "get with prefix",
					key:     "key",
					options: config.GetOptions{Prefix: true},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, 3, len(resp.Kvs))
					},
				},
				{
					name:    "get keys only",
					key:     "key",
					options: config.GetOptions{Prefix: true, KeysOnly: true},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, 3, len(resp.Kvs))
						for _, kv := range resp.Kvs {
							require.Empty(t, kv.Value)
						}
					},
				},
				{
					name:    "get count only",
					key:     "key",
					options: config.GetOptions{Prefix: true, CountOnly: true},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, int64(3), resp.Count)
					},
				},
				{
					name:    "get with limit",
					key:     "key",
					options: config.GetOptions{Prefix: true, Limit: 2},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, 2, len(resp.Kvs))
					},
				},
				{
					name:    "get with sort by key ascending",
					key:     "key",
					options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByKey, Order: clientv3.SortAscend},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, 3, len(resp.Kvs))
						require.Equal(t, "key1", string(resp.Kvs[0].Key))
						require.Equal(t, "key2", string(resp.Kvs[1].Key))
						require.Equal(t, "key3", string(resp.Kvs[2].Key))
					},
				},
				{
					name:    "get with sort by key descending",
					key:     "key",
					options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByKey, Order: clientv3.SortDescend},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, 3, len(resp.Kvs))
						require.Equal(t, "key3", string(resp.Kvs[0].Key))
						require.Equal(t, "key2", string(resp.Kvs[1].Key))
						require.Equal(t, "key1", string(resp.Kvs[2].Key))
					},
				},
				{
					name:    "get with sort by modify revision ascending",
					key:     "key",
					options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByModRevision, Order: clientv3.SortAscend},
					validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
						require.Equal(t, 3, len(resp.Kvs))
						// Verify keys are in order of modification revision
						for i := 1; i < len(resp.Kvs); i++ {
							require.LessOrEqual(t, resp.Kvs[i-1].ModRevision, resp.Kvs[i].ModRevision)
						}
					},
				},
			}

			for _, tc := range tcs {
				t.Run(tc.name, func(t *testing.T) {
					getResp, err := client.Get(ctx, tc.key, tc.options)
					require.NoError(t, err)
					tc.validateFunc(t, getResp)
				})
			}
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

// putTest - Basic put/get test functionality
func putTest(ctx context.Context, t *testing.T, client intf.Client) {
	key, value := "foo", "bar"

	// Put the key-value pair
	err := client.Put(ctx, key, value, config.PutOptions{})
	require.NoError(t, err)

	// Verify it was stored correctly
	getResp, err := client.Get(ctx, key, config.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, len(getResp.Kvs))
	require.Equal(t, key, string(getResp.Kvs[0].Key))
	require.Equal(t, value, string(getResp.Kvs[0].Value))
}

// getTest - Comprehensive get operations with prefix, sorting, and limits
func getTest(ctx context.Context, t *testing.T, client intf.Client) {
	// Test data
	kvs := []struct{ key, val string }{
		{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"},
	}
	revkvs := []struct{ key, val string }{
		{"key3", "val3"}, {"key2", "val2"}, {"key1", "val1"},
	}

	// Put test data
	for i := range kvs {
		err := client.Put(ctx, kvs[i].key, kvs[i].val, config.PutOptions{})
		require.NoError(t, err, "getTest #%d: Put error", i)
	}

	// Test cases
	tests := []struct {
		name    string
		options config.GetOptions
		wkv     []struct{ key, val string }
	}{
		{"single key", config.GetOptions{}, []struct{ key, val string }{{"key1", "val1"}}},
		{"prefix all", config.GetOptions{Prefix: true}, kvs},
		{"prefix with key", config.GetOptions{Prefix: true}, kvs},
		{"prefix with limit", config.GetOptions{Prefix: true, Limit: 2}, kvs[:2]},
		{"sort by modify ascending", config.GetOptions{Prefix: true, SortBy: clientv3.SortByModRevision, Order: clientv3.SortAscend}, kvs},
		{"sort by version ascending", config.GetOptions{Prefix: true, SortBy: clientv3.SortByVersion, Order: clientv3.SortAscend}, kvs},
		{"sort by create ascending", config.GetOptions{Prefix: true, SortBy: clientv3.SortByCreateRevision, Order: clientv3.SortAscend}, kvs},
		{"sort by create descending", config.GetOptions{Prefix: true, SortBy: clientv3.SortByCreateRevision, Order: clientv3.SortDescend}, revkvs},
		{"sort by key descending", config.GetOptions{Prefix: true, SortBy: clientv3.SortByKey, Order: clientv3.SortDescend}, revkvs},
	}

	testKeys := []string{"key1", "", "", "key", "key", "key", "key", "key", "key"}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getResp, err := client.Get(ctx, testKeys[i], tt.options)
			require.NoError(t, err, "getTest #%d: Get error", i)

			require.Equal(t, len(tt.wkv), len(getResp.Kvs), "getTest #%d: wrong number of keys", i)
			for j, expectedKv := range tt.wkv {
				require.Equal(t, expectedKv.key, string(getResp.Kvs[j].Key), "getTest #%d: wrong key at index %d", i, j)
				require.Equal(t, expectedKv.val, string(getResp.Kvs[j].Value), "getTest #%d: wrong value at index %d", i, j)
			}
		})
	}
}

// getFormatTest - Test different ways of getting formatted data
func getFormatTest(ctx context.Context, t *testing.T, client intf.Client) {
	// Put test data
	key, value := "abc", "123"
	err := client.Put(ctx, key, value, config.PutOptions{})
	require.NoError(t, err)

	tests := []struct {
		name         string
		options      config.GetOptions
		validateFunc func(t *testing.T, resp *clientv3.GetResponse)
	}{
		{
			name:    "normal get",
			options: config.GetOptions{},
			validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
				require.Equal(t, 1, len(resp.Kvs))
				require.Equal(t, key, string(resp.Kvs[0].Key))
				require.Equal(t, value, string(resp.Kvs[0].Value))
			},
		},
		{
			name:    "keys only",
			options: config.GetOptions{KeysOnly: true},
			validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
				require.Equal(t, 1, len(resp.Kvs))
				require.Equal(t, key, string(resp.Kvs[0].Key))
				require.Empty(t, resp.Kvs[0].Value)
			},
		},
		{
			name:    "count only",
			options: config.GetOptions{CountOnly: true},
			validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
				require.Equal(t, int64(1), resp.Count)
				require.Equal(t, 0, len(resp.Kvs)) // No keys returned with CountOnly
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getResp, err := client.Get(ctx, key, tt.options)
			require.NoError(t, err)
			tt.validateFunc(t, getResp)
		})
	}
}

// getMinMaxCreateModRevTest - Test get operations with revision validation
func getMinMaxCreateModRevTest(ctx context.Context, t *testing.T, client intf.Client) {
	kvs := []struct{ key, val string }{
		{"key1", "val1"}, // First put
		{"key2", "val2"}, // Second put
		{"key1", "val3"}, // Update key1
		{"key4", "val4"}, // Third unique key
	}

	// Track revisions for validation
	var revisions []int64
	for i := range kvs {
		err := client.Put(ctx, kvs[i].key, kvs[i].val, config.PutOptions{})
		require.NoError(t, err, "getMinMaxCreateModRevTest #%d: Put error", i)

		// Get the current revision
		getResp, err := client.Get(ctx, kvs[i].key, config.GetOptions{})
		require.NoError(t, err)
		if len(getResp.Kvs) > 0 {
			revisions = append(revisions, getResp.Kvs[0].ModRevision)
		}
	}

	tests := []struct {
		name         string
		key          string
		options      config.GetOptions
		validateFunc func(t *testing.T, resp *clientv3.GetResponse)
	}{
		{
			name: "prefix get all keys",
			key:  "key",
			options: config.GetOptions{
				Prefix: true,
			},
			validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
				require.Equal(t, 3, len(resp.Kvs)) // key1, key2, key4
				keys := make(map[string]string)
				for _, kv := range resp.Kvs {
					keys[string(kv.Key)] = string(kv.Value)
				}
				require.Equal(t, "val3", keys["key1"]) // Updated value
				require.Equal(t, "val2", keys["key2"])
				require.Equal(t, "val4", keys["key4"])
			},
		},
		{
			name: "get with revision filter",
			key:  "key1",
			options: config.GetOptions{
				Revision: int(revisions[0]), // Get key1 at first revision
			},
			validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
				require.Equal(t, 1, len(resp.Kvs))
				require.Equal(t, "key1", string(resp.Kvs[0].Key))
				require.Equal(t, "val1", string(resp.Kvs[0].Value))
			},
		},
		{
			name: "verify create vs modify revisions",
			key:  "key",
			options: config.GetOptions{
				Prefix: true,
				SortBy: clientv3.SortByCreateRevision,
				Order:  clientv3.SortAscend,
			},
			validateFunc: func(t *testing.T, resp *clientv3.GetResponse) {
				require.Equal(t, 3, len(resp.Kvs))
				// Verify that CreateRevision and ModRevision are tracked correctly
				for _, kv := range resp.Kvs {
					require.Greater(t, kv.CreateRevision, int64(0))
					require.Greater(t, kv.ModRevision, int64(0))
					require.GreaterOrEqual(t, kv.ModRevision, kv.CreateRevision)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getResp, err := client.Get(ctx, tt.key, tt.options)
			require.NoError(t, err)
			tt.validateFunc(t, getResp)
		})
	}
}

// delTest - Test comprehensive delete operations
func delTest(ctx context.Context, t *testing.T, client intf.Client) {
	tests := []struct {
		name       string
		puts       []struct{ key, val string }
		deleteKey  string
		deleteOpts config.DeleteOptions
		deletedNum int64
	}{
		{
			name: "delete all with prefix",
			puts: []struct{ key, val string }{
				{"foo1", "bar"}, {"foo2", "bar"}, {"foo3", "bar"},
			},
			deleteKey:  "",
			deleteOpts: config.DeleteOptions{Prefix: true},
			deletedNum: 3,
		},
		{
			name: "delete non-existent key",
			puts: []struct{ key, val string }{
				{"this", "value"},
			},
			deleteKey:  "that",
			deleteOpts: config.DeleteOptions{},
			deletedNum: 0,
		},
		{
			name: "delete single key",
			puts: []struct{ key, val string }{
				{"sample", "value"},
			},
			deleteKey:  "sample",
			deleteOpts: config.DeleteOptions{},
			deletedNum: 1,
		},
		{
			name: "delete with prefix pattern",
			puts: []struct{ key, val string }{
				{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"},
			},
			deleteKey:  "key",
			deleteOpts: config.DeleteOptions{Prefix: true},
			deletedNum: 3,
		},
		{
			name: "delete range",
			puts: []struct{ key, val string }{
				{"zoo1", "bar"}, {"zoo2", "bar2"}, {"zoo3", "bar3"},
			},
			deleteKey:  "zoo1",
			deleteOpts: config.DeleteOptions{Prefix: true}, // Similar to --from-key
			deletedNum: 3,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Put test data
			for j := range tt.puts {
				err := client.Put(ctx, tt.puts[j].key, tt.puts[j].val, config.PutOptions{})
				require.NoError(t, err, "delTest #%d-%d: Put error", i, j)
			}

			// Delete operation
			delResp, err := client.Delete(ctx, tt.deleteKey, tt.deleteOpts)
			require.NoError(t, err, "delTest #%d: Delete error", i)
			require.Equal(t, tt.deletedNum, delResp.Deleted, "delTest #%d: wrong deleted count", i)

			// Verify deletion by checking remaining keys
			if tt.deletedNum > 0 {
				getResp, err := client.Get(ctx, tt.deleteKey, config.GetOptions{Prefix: true})
				require.NoError(t, err)
				if tt.deleteOpts.Prefix {
					require.Equal(t, 0, len(getResp.Kvs), "delTest #%d: keys still exist after prefix delete", i)
				}
			}
		})
	}
}
