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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestKVPut(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				key, value := "foo", "bar"

				require.NoErrorf(t, cc.Put(ctx, key, value, config.PutOptions{}), "count not put key %q", key)
				resp, err := cc.Get(ctx, key, config.GetOptions{})
				require.NoErrorf(t, err, "count not get key %q, err: %s", key, err)
				assert.Lenf(t, resp.Kvs, 1, "Unexpected length of response, got %d", len(resp.Kvs))
				assert.Equalf(t, string(resp.Kvs[0].Key), key, "Unexpected key, want %q, got %q", key, resp.Kvs[0].Key)
				assert.Equalf(t, string(resp.Kvs[0].Value), value, "Unexpected value, want %q, got %q", value, resp.Kvs[0].Value)
			})
		})
	}
}

func TestKVGet(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				var (
					kvs          = []string{"a", "b", "c", "c", "c", "foo", "foo/abc", "fop"}
					wantKvs      = []string{"a", "b", "c", "foo", "foo/abc", "fop"}
					kvsByVersion = []string{"a", "b", "foo", "foo/abc", "fop", "c"}
					reversedKvs  = []string{"fop", "foo/abc", "foo", "c", "b", "a"}
				)

				for i := range kvs {
					require.NoErrorf(t, cc.Put(ctx, kvs[i], "bar", config.PutOptions{}), "count not put key %q", kvs[i])
				}
				tests := []struct {
					begin   string
					end     string
					options config.GetOptions

					wkv []string
				}{
					{begin: "a", wkv: wantKvs[:1]},
					{begin: "a", options: config.GetOptions{Serializable: true}, wkv: wantKvs[:1]},
					{begin: "a", options: config.GetOptions{End: "c"}, wkv: wantKvs[:2]},
					{begin: "", options: config.GetOptions{Prefix: true}, wkv: wantKvs},
					{begin: "", options: config.GetOptions{FromKey: true}, wkv: wantKvs},
					{begin: "a", options: config.GetOptions{End: "x"}, wkv: wantKvs},
					{begin: "", options: config.GetOptions{Prefix: true, Revision: 4}, wkv: kvs[:3]},
					{begin: "a", options: config.GetOptions{CountOnly: true}, wkv: nil},
					{begin: "foo", options: config.GetOptions{Prefix: true}, wkv: []string{"foo", "foo/abc"}},
					{begin: "foo", options: config.GetOptions{FromKey: true}, wkv: []string{"foo", "foo/abc", "fop"}},
					{begin: "", options: config.GetOptions{Prefix: true, Limit: 2}, wkv: wantKvs[:2]},
					{begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortAscend, SortBy: clientv3.SortByModRevision}, wkv: wantKvs},
					{begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortAscend, SortBy: clientv3.SortByVersion}, wkv: kvsByVersion},
					{begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortNone, SortBy: clientv3.SortByCreateRevision}, wkv: wantKvs},
					{begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortDescend, SortBy: clientv3.SortByCreateRevision}, wkv: reversedKvs},
					{begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortDescend, SortBy: clientv3.SortByKey}, wkv: reversedKvs},
				}
				for _, tt := range tests {
					resp, err := cc.Get(ctx, tt.begin, tt.options)
					require.NoErrorf(t, err, "count not get key %q, err: %s", tt.begin, err)
					kvs := testutils.KeysFromGetResponse(resp)
					assert.Equal(t, tt.wkv, kvs)
				}
			})
		})
	}
}

func TestKVDelete(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())
			testutils.ExecuteUntil(ctx, t, func() {
				kvs := []string{"a", "b", "c", "c/abc", "d"}
				tests := []struct {
					deleteKey string
					options   config.DeleteOptions

					wantDeleted int
					wantKeys    []string
				}{
					{ // delete all keys
						deleteKey:   "",
						options:     config.DeleteOptions{Prefix: true},
						wantDeleted: 5,
					},
					{ // delete all keys
						deleteKey:   "",
						options:     config.DeleteOptions{FromKey: true},
						wantDeleted: 5,
					},
					{
						deleteKey:   "a",
						options:     config.DeleteOptions{End: "c"},
						wantDeleted: 2,
						wantKeys:    []string{"c", "c/abc", "d"},
					},
					{
						deleteKey:   "c",
						wantDeleted: 1,
						wantKeys:    []string{"a", "b", "c/abc", "d"},
					},
					{
						deleteKey:   "c",
						options:     config.DeleteOptions{Prefix: true},
						wantDeleted: 2,
						wantKeys:    []string{"a", "b", "d"},
					},
					{
						deleteKey:   "c",
						options:     config.DeleteOptions{FromKey: true},
						wantDeleted: 3,
						wantKeys:    []string{"a", "b"},
					},
					{
						deleteKey:   "e",
						wantDeleted: 0,
						wantKeys:    kvs,
					},
				}
				for _, tt := range tests {
					for i := range kvs {
						require.NoErrorf(t, cc.Put(ctx, kvs[i], "bar", config.PutOptions{}), "count not put key %q", kvs[i])
					}
					del, err := cc.Delete(ctx, tt.deleteKey, tt.options)
					require.NoErrorf(t, err, "count not get key %q, err", tt.deleteKey)
					assert.Equal(t, tt.wantDeleted, int(del.Deleted))
					get, err := cc.Get(ctx, "", config.GetOptions{Prefix: true})
					require.NoErrorf(t, err, "count not get key")
					kvs := testutils.KeysFromGetResponse(get)
					assert.Equal(t, tt.wantKeys, kvs)
				}
			})
		})
	}
}

func TestKVGetNoQuorum(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name    string
		options config.GetOptions

		wantError bool
	}{
		{
			name:    "Serializable",
			options: config.GetOptions{Serializable: true},
		},
		{
			name:      "Linearizable",
			options:   config.GetOptions{Serializable: false, Timeout: time.Second},
			wantError: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t)
			defer clus.Close()

			clus.Members()[0].Stop()
			clus.Members()[1].Stop()

			cc := clus.Members()[2].Client()
			testutils.ExecuteUntil(ctx, t, func() {
				key := "foo"
				_, err := cc.Get(ctx, key, tc.options)
				if tc.wantError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		})
	}
}
