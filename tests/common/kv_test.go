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

	"go.etcd.io/etcd/api/v3/mvccpb"
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

type rangeTestCase struct {
	description string
	key         string
	options     config.GetOptions

	wantResponse *clientv3.GetResponse
	wantError    error
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
				var (
					kvs          = []string{"a", "b", "c", "c", "c", "foo", "foo/abc", "fop"}
					wantKvs      = []string{"a", "b", "c", "foo", "foo/abc", "fop"}
					kvsByVersion = []string{"a", "b", "foo", "foo/abc", "fop", "c"}
					reversedKvs  = []string{"fop", "foo/abc", "foo", "c", "b", "a"}
				)

				for i := range kvs {
					_, err := cc.Put(ctx, kvs[i], "bar", config.PutOptions{})
					require.NoErrorf(t, err, "count not put key %q", kvs[i])
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

func TestKVGetWithHeader(t *testing.T) {
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
					{Key: "c", Val: "raz"},
					{Key: "z", Val: "fooz"},
					{Key: "d", Val: "food"},
					{Key: "e", Val: "fooe"},
					{Key: "f", Val: "foof"},
					{Key: "c/xyz", Val: "przedrostekraz"},
					{Key: "c", Val: "dwa"},
					{Key: "g", Val: "foog"},
					{Key: "c", Val: "trzy"},
					{Key: "aa", Val: "podwojnie"},
					{Key: "c/abc", Val: "przedrostekdwa"},
				}

				var firstRev int
				for i := range puts {
					resp, err := cc.Put(ctx, puts[i].Key, puts[i].Val, config.PutOptions{})
					require.NoErrorf(t, err, "could not put key %q", puts[i])
					if i == 0 {
						firstRev = int(resp.Header.Revision)
					}
				}
				baseTestCases := []rangeTestCase{
					{
						description: "single key without any options",
						key:         "a",
						options:     config.GetOptions{},
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "single key without any options serializable",
						key:         "a",
						options: config.GetOptions{
							Serializable: true,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "[a, c) range with default ordering",
						key:         "a",
						options: config.GetOptions{
							End: "c",
						},
						wantResponse: &clientv3.GetResponse{
							Count: 3,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
							},
							More: false,
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
						wantResponse: &clientv3.GetResponse{
							Count: 12,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "fromkey option without any specified key -> return all",
						key:         "",
						options: config.GetOptions{
							FromKey: true,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "prefix option without any specified key -> return all",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "start and end covering all entries",
						key:         "a",
						options: config.GetOptions{
							End: "zz",
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "no key with prefix and specific revision -> all KVs before the revision",
						key:         "",
						options: config.GetOptions{
							Prefix:   true,
							Revision: firstRev + 4,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 5,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("c"), Value: []byte("raz"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 4), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "single key with count only",
						key:         "a",
						options: config.GetOptions{
							CountOnly: true,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs:   nil,
							More:  false,
						},
					},
					{
						description: "start and end covering all entries, but count only",
						key:         "a",
						options: config.GetOptions{
							CountOnly: true,
							End:       "zz",
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs:   nil,
							More:  false,
						},
					},
					{
						description: "prefix of c",
						key:         "c",
						options: config.GetOptions{
							Prefix: true,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 3,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "from key c",
						key:         "c",
						options: config.GetOptions{
							FromKey: true,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 10,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "no key with prefix, but with a limit",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Limit:  2,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
							},
							More: true,
						},
					},
					{
						description: "from key of c, but with a limit",
						key:         "c",
						options: config.GetOptions{
							FromKey: true,
							Limit:   2,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 10,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
							},
							More: true,
						},
					},
					// sorting tests
					{
						description: "all entries, sorted by their mod revision, default ordering",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortNone,
							SortBy: clientv3.SortByModRevision,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "all entries, sorted by their mod revision, ordering ascendingly",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortAscend,
							SortBy: clientv3.SortByModRevision,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "all entries, sorted by their version, default ordering",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							SortBy: clientv3.SortByVersion,
							Order:  clientv3.SortAscend,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
							},
							More: false,
						},
					},
					{
						description: "all entries, sorted by create revision ordered by none implies ascendingly",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortNone,
							SortBy: clientv3.SortByCreateRevision,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "all entries, sorted by create revision ordered descendingly",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortDescend,
							SortBy: clientv3.SortByCreateRevision,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "all entries with no sorted by implies sorting by key",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Limit:  3,
							Order:  clientv3.SortDescend,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
							},
							More: true,
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
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
							},
							More: false,
						},
					},
					{
						description: "all entries, descending, on unspecified field implies key",
						key:         "",
						options: config.GetOptions{
							Prefix: true,
							Order:  clientv3.SortDescend,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("g"), Value: []byte("foog"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("f"), Value: []byte("foof"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("e"), Value: []byte("fooe"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("d"), Value: []byte("food"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("c/xyz"), Value: []byte("przedrostekraz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("c/abc"), Value: []byte("przedrostekdwa"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("aa"), Value: []byte("podwojnie"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("a"), Value: []byte("fooa"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
							},
							More: false,
						},
					},
					// filtering tests
					{
						description: "key range with an end and min/max mod revs -> range filtered by min/max mod revs, sorted descending",
						key:         "a",
						options: config.GetOptions{
							End:            "z",
							MinModRevision: firstRev + 1,
							MaxModRevision: firstRev + 5,
							Order:          clientv3.SortDescend,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 12,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
							},
							More: false,
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
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("b"), Value: []byte("foob"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("x"), Value: []byte("foox"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), Value: []byte("fooy"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), Value: []byte("fooz"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
							},
							More: false,
						},
					},
					// multiversion test
					{
						description: "a specific key with a specific revision - first update",
						key:         "c",
						options: config.GetOptions{
							Revision: firstRev + 10, // first update on 'c'
						},
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("dwa"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 10), Version: 2},
							},
							More: false,
						},
					},
					{
						description: "a specific key with a specific revision - after first update but before second update",
						key:         "c",
						options: config.GetOptions{
							Revision: firstRev + 11,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("dwa"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 10), Version: 2},
							},
							More: false,
						},
					},
					{
						description: "a specific key with a specific revision - second update",
						key:         "c",
						options: config.GetOptions{
							Revision: firstRev + 12,
						},
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
							},
							More: false,
						},
					},
					{
						description: "a specific key with a specific revision - after second update, but before current rev",
						key:         "c",
						options: config.GetOptions{
							Revision: firstRev + 12 + 2, // 2 revs after 2nd update
						},
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
							},
							More: false,
						},
					},
					{
						description: "a specific key with a specific revision - current rev",
						key:         "c",
						wantResponse: &clientv3.GetResponse{
							Count: 1,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("c"), Value: []byte("trzy"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
							},
							More: false,
						},
					},
					// keys-only option test
					{
						description: "start and end covering all entries, but keys only",
						key:         "a",
						options: config.GetOptions{
							KeysOnly: true,
							End:      "zz",
						},
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("a"), CreateRevision: int64(firstRev), ModRevision: int64(firstRev), Version: 1},
								{Key: []byte("aa"), CreateRevision: int64(firstRev + 13), ModRevision: int64(firstRev + 13), Version: 1},
								{Key: []byte("b"), CreateRevision: int64(firstRev + 2), ModRevision: int64(firstRev + 2), Version: 1},
								{Key: []byte("c"), CreateRevision: int64(firstRev + 4), ModRevision: int64(firstRev + 12), Version: 3},
								{Key: []byte("c/abc"), CreateRevision: int64(firstRev + 14), ModRevision: int64(firstRev + 14), Version: 1},
								{Key: []byte("c/xyz"), CreateRevision: int64(firstRev + 9), ModRevision: int64(firstRev + 9), Version: 1},
								{Key: []byte("d"), CreateRevision: int64(firstRev + 6), ModRevision: int64(firstRev + 6), Version: 1},
								{Key: []byte("e"), CreateRevision: int64(firstRev + 7), ModRevision: int64(firstRev + 7), Version: 1},
								{Key: []byte("f"), CreateRevision: int64(firstRev + 8), ModRevision: int64(firstRev + 8), Version: 1},
								{Key: []byte("g"), CreateRevision: int64(firstRev + 11), ModRevision: int64(firstRev + 11), Version: 1},
								{Key: []byte("x"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
								{Key: []byte("y"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("z"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
							},
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
						wantResponse: &clientv3.GetResponse{
							Count: 13,
							Kvs: []*mvccpb.KeyValue{
								{Key: []byte("z"), CreateRevision: int64(firstRev + 5), ModRevision: int64(firstRev + 5), Version: 1},
								{Key: []byte("y"), CreateRevision: int64(firstRev + 3), ModRevision: int64(firstRev + 3), Version: 1},
								{Key: []byte("x"), CreateRevision: int64(firstRev + 1), ModRevision: int64(firstRev + 1), Version: 1},
							},
							More: true,
						},
					},
				}
				for _, tt := range baseTestCases {
					t.Run(tt.description, func(t *testing.T) {
						resp, err := cc.Get(ctx, tt.key, tt.options)
						if tt.wantError != nil {
							require.Contains(t, err.Error(), tt.wantError.Error())
							require.Nil(t, resp)
							return
						}
						require.NoErrorf(t, err, "count not get key %q, err: %s", tt.key, err)
						assert.Equal(t, tt.wantResponse.Kvs, resp.Kvs)
						assert.Equal(t, tt.wantResponse.Count, resp.Count)
						assert.Equal(t, tt.wantResponse.More, resp.More)
					})
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
						_, err := cc.Put(ctx, kvs[i], "podwojnie", config.PutOptions{})
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
