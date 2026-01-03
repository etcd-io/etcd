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
	"errors"
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

				_, err := cc.Put(ctx, key, value, config.PutOptions{})
				require.NoErrorf(t, err, "count not put key %q", key)
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
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				puts := []testutils.KV{
					{Key: "a", Val: "fooa"},
					{Key: "x", Val: "foox"},
					{Key: "b", Val: "foob"},
					{Key: "y", Val: "fooy"},
					{Key: "c", Val: "fooc"},
					{Key: "z", Val: "fooz"},
					{Key: "d", Val: "food"},
					{Key: "e", Val: "fooe"},
					{Key: "f", Val: "foof"},
					{Key: "c/xyz", Val: "4ever"},
					{Key: "c", Val: "foocc"},
					{Key: "g", Val: "foog"},
					{Key: "c", Val: "fooccc"},
					{Key: "aa", Val: "bar"},
					{Key: "c/abc", Val: "egg"},
				}

				var firstRev int
				for i := range puts {
					resp, err := cc.Put(ctx, puts[i].Key, puts[i].Val, config.PutOptions{})
					require.NoErrorf(t, err, "count not put key %q", puts[i])
					if i == 0 {
						firstRev = int(resp.Header.Revision)
					}
				}
				tests := []struct {
					description string
					key         string
					options     config.GetOptions

					wkv           []testutils.KV
					expectedError error
					checkCount    bool
					expectedCount int64
				}{
					{
						description: "existing single key",
						key:         "a",
						wkv:         []testutils.KV{{Key: "a", Val: "fooa"}},
					},
					{
						description: "existing single key with Serializable",
						key:         "a",
						wkv:         []testutils.KV{{Key: "a", Val: "fooa"}},
						options: config.GetOptions{
							Serializable: true,
						},
					},
					{
						description: "non existent key",
						key:         "thiskeydoesntexist",
						wkv:         nil,
					},
					{
						description: "key exists but revision is in the future",
						key:         "a",
						options: config.GetOptions{
							Revision: 99999,
						},
						expectedError: errors.New("etcdserver: mvcc: required revision is a future revision"),
					},
					{
						description: "key doesn't exist and revision is in the future",
						key:         "thiskeydoesntexist",
						options: config.GetOptions{
							Revision: 99999,
						},
						expectedError: errors.New("etcdserver: mvcc: required revision is a future revision"),
					},
					{
						description: "range [a, b)",
						key:         "a",
						options: config.GetOptions{
							End: "b",
						},
						wkv: []testutils.KV{{Key: "a", Val: "fooa"}, {Key: "aa", Val: "bar"}},
					},
					{
						description: "no key specified with prefix -> return all",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
						},
						wkv: []testutils.KV{
							{Key: "a", Val: "fooa"},
							{Key: "aa", Val: "bar"},
							{Key: "b", Val: "foob"},
							{Key: "c", Val: "fooccc"},
							{Key: "c/abc", Val: "egg"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "d", Val: "food"},
							{Key: "e", Val: "fooe"},
							{Key: "f", Val: "foof"},
							{Key: "g", Val: "foog"},
							{Key: "x", Val: "foox"},
							{Key: "y", Val: "fooy"},
							{Key: "z", Val: "fooz"},
						},
					},
					{
						description: "no key with prefix and specific revision -> all KVs before the revision",
						key:         "",
						options: config.GetOptions{
							Prefix:   true,
							Revision: firstRev + 4,
						},
						wkv: []testutils.KV{
							{Key: "a", Val: "fooa"},
							{Key: "b", Val: "foob"},
							{Key: "c", Val: "fooc"},
							{Key: "x", Val: "foox"},
							{Key: "y", Val: "fooy"},
						},
					},
					{
						description: "a specific key with a specific revision",
						key:         "c",
						options: config.GetOptions{
							Revision: firstRev + 10, // first update on 'c'
						},
						wkv: []testutils.KV{
							{Key: "c", Val: "foocc"},
						},
					},
					{
						description: "key range with an end and min/max mod revs -> range filtered by min/max mod revs, sorted descending",
						key:         "a",
						options: config.GetOptions{
							End:            "z",
							MinModRevision: firstRev + 1,
							MaxModRevision: firstRev + 5,
							Order:          clientv3.SortDescend,
						},
						wkv: []testutils.KV{
							{Key: "y", Val: "fooy"},
							{Key: "x", Val: "foox"},
							{Key: "b", Val: "foob"},
						},
					},
					{
						description: "all keys with min/max create revs -> range filtered by min/max create revs",
						key:         "",
						options: config.GetOptions{
							Prefix:            true,
							MinCreateRevision: firstRev + 1,
							MaxCreateRevision: firstRev + 5,
						},
						wkv: []testutils.KV{
							{Key: "b", Val: "foob"},
							{Key: "c", Val: "fooccc"},
							{Key: "x", Val: "foox"},
							{Key: "y", Val: "fooy"},
							{Key: "z", Val: "fooz"},
						},
					},
					{
						description: "prefix of c",
						key:         "c",
						options: config.GetOptions{
							Prefix: true,
						},
						wkv: []testutils.KV{
							{Key: "c", Val: "fooccc"},
							{Key: "c/abc", Val: "egg"},
							{Key: "c/xyz", Val: "4ever"},
						},
					},
					{
						description: "from key c",
						key:         "c",
						options: config.GetOptions{
							FromKey: true,
						},
						wkv: []testutils.KV{
							{Key: "c", Val: "fooccc"},
							{Key: "c/abc", Val: "egg"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "d", Val: "food"},
							{Key: "e", Val: "fooe"},
							{Key: "f", Val: "foof"},
							{Key: "g", Val: "foog"},
							{Key: "x", Val: "foox"},
							{Key: "y", Val: "fooy"},
							{Key: "z", Val: "fooz"},
						},
					},
					{
						description: "fromkey without any specified key -> return all",
						key:         "",
						options: config.GetOptions{
							FromKey: true,
						},
						wkv: []testutils.KV{
							{Key: "a", Val: "fooa"},
							{Key: "aa", Val: "bar"},
							{Key: "b", Val: "foob"},
							{Key: "c", Val: "fooccc"},
							{Key: "c/abc", Val: "egg"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "d", Val: "food"},
							{Key: "e", Val: "fooe"},
							{Key: "f", Val: "foof"},
							{Key: "g", Val: "foog"},
							{Key: "x", Val: "foox"},
							{Key: "y", Val: "fooy"},
							{Key: "z", Val: "fooz"},
						},
					},
					{
						description: "start and end covering all entries",
						key:         "a",
						options: config.GetOptions{
							End: "zz",
						},
						wkv: []testutils.KV{
							{Key: "a", Val: "fooa"},
							{Key: "aa", Val: "bar"},
							{Key: "b", Val: "foob"},
							{Key: "c", Val: "fooccc"},
							{Key: "c/abc", Val: "egg"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "d", Val: "food"},
							{Key: "e", Val: "fooe"},
							{Key: "f", Val: "foof"},
							{Key: "g", Val: "foog"},
							{Key: "x", Val: "foox"},
							{Key: "y", Val: "fooy"},
							{Key: "z", Val: "fooz"},
						},
					},
					{
						description: "start and end covering all entries, but keys only",
						key:         "a",
						options: config.GetOptions{
							KeysOnly: true,
							End:      "zz",
						},
						wkv: []testutils.KV{
							{Key: "a"},
							{Key: "aa"},
							{Key: "b"},
							{Key: "c"},
							{Key: "c/abc"},
							{Key: "c/xyz"},
							{Key: "d"},
							{Key: "e"},
							{Key: "f"},
							{Key: "g"},
							{Key: "x"},
							{Key: "y"},
							{Key: "z"},
						},
					},
					{
						description: "start and end covering all entries, but keys only",
						key:         "a",
						options: config.GetOptions{
							CountOnly: true,
							End:       "zz",
						},
						wkv:           nil,
						checkCount:    true,
						expectedCount: int64(13),
					},
					{
						description: "--count-only overrides --keys-only when both are true",
						key:         "a",
						options: config.GetOptions{
							KeysOnly:  true,
							CountOnly: true,
							End:       "zz",
						},
						wkv:           nil,
						checkCount:    true,
						expectedCount: int64(13),
					},
					{
						description: "from key of c, but with a limit",
						key:         "c",
						options: config.GetOptions{
							FromKey: true,
							Limit:   2,
						},
						wkv: []testutils.KV{
							{Key: "c", Val: "fooccc"},
							{Key: "c/abc", Val: "egg"},
						},
					},
					{
						description: "all entries, sorted by their mod revision",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortNone,
							SortBy: clientv3.SortByModRevision,
						},
						wkv: []testutils.KV{
							{Key: "a", Val: "fooa"},
							{Key: "x", Val: "foox"},
							{Key: "b", Val: "foob"},
							{Key: "y", Val: "fooy"},
							{Key: "z", Val: "fooz"},
							{Key: "d", Val: "food"},
							{Key: "e", Val: "fooe"},
							{Key: "f", Val: "foof"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "g", Val: "foog"},
							{Key: "c", Val: "fooccc"},
							{Key: "aa", Val: "bar"},
							{Key: "c/abc", Val: "egg"},
						},
					},
					{
						description: "all entries sorted by key descending with limit",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Limit:  3,
							Order:  clientv3.SortDescend,
						},
						wkv: []testutils.KV{
							{Key: "z", Val: "fooz"},
							{Key: "y", Val: "fooy"},
							{Key: "x", Val: "foox"},
						},
					},
					{
						description: "all entries sorted by key descending with a limit, and keys only",
						key:         "",
						options: config.GetOptions{
							Prefix:   true,
							Limit:    3,
							KeysOnly: true,
							Order:    clientv3.SortDescend,
						},
						wkv: []testutils.KV{
							{Key: "z"},
							{Key: "y"},
							{Key: "x"},
						},
					},
					{
						description: "all entries, sorted by create revision",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortDescend,
							SortBy: clientv3.SortByCreateRevision,
						},
						wkv: []testutils.KV{
							{Key: "c/abc", Val: "egg"},
							{Key: "aa", Val: "bar"},
							{Key: "g", Val: "foog"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "f", Val: "foof"},
							{Key: "e", Val: "fooe"},
							{Key: "d", Val: "food"},
							{Key: "z", Val: "fooz"},
							{Key: "c", Val: "fooccc"},
							{Key: "y", Val: "fooy"},
							{Key: "b", Val: "foob"},
							{Key: "x", Val: "foox"},
							{Key: "a", Val: "fooa"},
						},
					},
					{
						description: "all entries, sorted by key descending",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortDescend,
							SortBy: clientv3.SortByKey,
						},
						wkv: []testutils.KV{
							{Key: "z", Val: "fooz"},
							{Key: "y", Val: "fooy"},
							{Key: "x", Val: "foox"},
							{Key: "g", Val: "foog"},
							{Key: "f", Val: "foof"},
							{Key: "e", Val: "fooe"},
							{Key: "d", Val: "food"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "c/abc", Val: "egg"},
							{Key: "c", Val: "fooccc"},
							{Key: "b", Val: "foob"},
							{Key: "aa", Val: "bar"},
							{Key: "a", Val: "fooa"},
						},
					},
					{
						description: "all entries, by default sorted by key descending",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortDescend,
						},
						wkv: []testutils.KV{
							{Key: "z", Val: "fooz"},
							{Key: "y", Val: "fooy"},
							{Key: "x", Val: "foox"},
							{Key: "g", Val: "foog"},
							{Key: "f", Val: "foof"},
							{Key: "e", Val: "fooe"},
							{Key: "d", Val: "food"},
							{Key: "c/xyz", Val: "4ever"},
							{Key: "c/abc", Val: "egg"},
							{Key: "c", Val: "fooccc"},
							{Key: "b", Val: "foob"},
							{Key: "aa", Val: "bar"},
							{Key: "a", Val: "fooa"},
						},
					},
				}
				for _, tt := range tests {
					resp, err := cc.Get(ctx, tt.key, tt.options)
					if tt.expectedError != nil {
						require.EqualError(t, err, tt.expectedError.Error())
						require.Nil(t, resp)
					} else {
						require.NoErrorf(t, err, "count not get key %q, err: %s", tt.key, err)
						kvs := testutils.KeyValuesFromGetResponse(resp)
						assert.Equal(t, tt.wkv, kvs)
						if tt.checkCount {
							assert.Equal(t, tt.expectedCount, resp.Count)
						}
					}
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
						_, err := cc.Put(ctx, kvs[i], "bar", config.PutOptions{})
						require.NoErrorf(t, err, "count not put key %q", kvs[i])
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
