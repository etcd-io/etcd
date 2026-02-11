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
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
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
				kvs := [][]string{
					{"a", "bar_01"},
					{"b", "bar_02"},
					{"c", "bar_03"},
					{"c", "bar_04"},
					{"c", "bar_05"},
					{"foo", "bar_06"},
					{"foo/abc", "bar_07"},
					{"fop", "bar_08"},
				}

				var firstHeader *etcdserverpb.ResponseHeader
				for i := range kvs {
					resp, err := cc.Put(ctx, kvs[i][0], kvs[i][1], config.PutOptions{})
					require.NoErrorf(t, err, "count not put key value %q", kvs[i])
					if i == 0 {
						firstHeader = resp.Header
					}
				}

				firstRev := firstHeader.Revision
				kvA := createKV("a", "bar_01", firstRev, firstRev, 1)
				kvB := createKV("b", "bar_02", firstRev+1, firstRev+1, 1)
				kvCV1 := createKV("c", "bar_03", firstRev+2, firstRev+2, 1)
				kvCV2 := createKV("c", "bar_04", firstRev+2, firstRev+3, 2)
				kvC := createKV("c", "bar_05", firstRev+2, firstRev+4, 3)
				kvFoo := createKV("foo", "bar_06", firstRev+5, firstRev+5, 1)
				kvFooAbc := createKV("foo/abc", "bar_07", firstRev+6, firstRev+6, 1)
				kvFop := createKV("fop", "bar_08", firstRev+7, firstRev+7, 1)

				allKvs := []*mvccpb.KeyValue{kvA, kvB, kvC, kvFoo, kvFooAbc, kvFop}
				kvsByVersion := []*mvccpb.KeyValue{kvA, kvB, kvFoo, kvFooAbc, kvFop, kvC}
				kvsByVersionDesc := []*mvccpb.KeyValue{kvC, kvA, kvB, kvFoo, kvFooAbc, kvFop}
				reversedKvs := []*mvccpb.KeyValue{kvFop, kvFooAbc, kvFoo, kvC, kvB, kvA}
				kvsByValue := []*mvccpb.KeyValue{kvA, kvB, kvC, kvFoo, kvFooAbc, kvFop}
				reversedKvsByValue := []*mvccpb.KeyValue{kvFop, kvFooAbc, kvFoo, kvC, kvB, kvA}

				type testcase struct {
					name    string
					begin   string
					options config.GetOptions

					wantResponse *clientv3.GetResponse
				}
				tests := []testcase{
					{name: "Get one specific key (a)", begin: "a", wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 1, Kvs: []*mvccpb.KeyValue{kvA}}},
					{name: "Get one specific key (a), serializable", begin: "a", options: config.GetOptions{Serializable: true}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 1, Kvs: []*mvccpb.KeyValue{kvA}}},
					{name: "Get [a, c)", begin: "a", options: config.GetOptions{End: "c"}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 2, Kvs: []*mvccpb.KeyValue{kvA, kvB}}},
					{name: "blank key with --prefix option -> all KVs", begin: "", options: config.GetOptions{Prefix: true}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: allKvs}},
					{name: "blank key with --from-key option -> all KVs", begin: "", options: config.GetOptions{FromKey: true}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: allKvs}},
					{name: "Range covering all keys -> all KVs", begin: "a", options: config.GetOptions{End: "x"}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: allKvs}},
					{name: "blank key with --prefix and revision -> [first key, entry at specified revision]", begin: "", options: config.GetOptions{Prefix: true, Revision: int(firstRev + 2)}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 3, Kvs: []*mvccpb.KeyValue{kvA, kvB, kvCV1}}},
					{name: "--count-only for one single key", begin: "a", options: config.GetOptions{CountOnly: true}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 1, Kvs: nil}},
					{name: "--prefix of foo -> all entries with the prefix", begin: "foo", options: config.GetOptions{Prefix: true}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 2, Kvs: []*mvccpb.KeyValue{kvFoo, kvFooAbc}}},
					{name: "--from-key of 'foo' -> [", begin: "foo", options: config.GetOptions{FromKey: true}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 3, Kvs: []*mvccpb.KeyValue{kvFoo, kvFooAbc, kvFop}}},
					{name: "blank key with limit set", begin: "", options: config.GetOptions{Prefix: true, Limit: 2}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: []*mvccpb.KeyValue{kvA, kvB}, More: true}},
					{name: "all KVs descending", begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortDescend}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: reversedKvs}},
					{name: "Get first version of 'c' by its revision", begin: "c", options: config.GetOptions{Revision: int(firstRev) + 2}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 1, Kvs: []*mvccpb.KeyValue{kvCV1}}},
					{name: "Get second version of 'c' by its revision", begin: "c", options: config.GetOptions{Revision: int(firstRev) + 3}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 1, Kvs: []*mvccpb.KeyValue{kvCV2}}},
					{name: "Get third version of 'c' by its revision", begin: "c", options: config.GetOptions{Revision: int(firstRev) + 4}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 1, Kvs: []*mvccpb.KeyValue{kvC}}},
					{name: "Get the latest version of 'c'", begin: "c", wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 1, Kvs: []*mvccpb.KeyValue{kvC}}},
					{name: "all KVs with mininum mod revision sorted by mod revision", begin: "", options: config.GetOptions{Prefix: true, MinModRevision: int(firstRev) + 2, SortBy: clientv3.SortByModRevision}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: []*mvccpb.KeyValue{kvC, kvFoo, kvFooAbc, kvFop}}},
					{name: "all KVs with maximum mod revision, sorted by key descending", begin: "", options: config.GetOptions{Prefix: true, MaxModRevision: int(firstRev) + 3, Order: clientv3.SortDescend, SortBy: clientv3.SortByKey}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: []*mvccpb.KeyValue{kvB, kvA}}},
					{name: "all KVs with minimum create revision, sorted by version, descending", begin: "", options: config.GetOptions{Prefix: true, MinCreateRevision: int(firstRev) + 2, Order: clientv3.SortDescend, SortBy: clientv3.SortByVersion}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: []*mvccpb.KeyValue{kvC, kvFoo, kvFooAbc, kvFop}}},
					{name: "all KVs with maximimum create revision, sorted by value", begin: "", options: config.GetOptions{Prefix: true, MaxCreateRevision: int(firstRev) + 5, SortBy: clientv3.SortByValue}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: []*mvccpb.KeyValue{kvA, kvB, kvC, kvFoo}}},
					{name: "all KVs sorted by key", begin: "", options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByKey}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: allKvs}},
					{name: "all KVs sorted by key descending", begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortDescend, SortBy: clientv3.SortByKey}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: reversedKvs}},
					{name: "all KVs sorted by create revision, unspecified order", begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortNone, SortBy: clientv3.SortByCreateRevision}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: allKvs}},
					{name: "all KVs sorted by create revision ascending", begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortAscend, SortBy: clientv3.SortByCreateRevision}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: allKvs}},
					{name: "all KVs sorted by create revision descending", begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortDescend, SortBy: clientv3.SortByCreateRevision}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: reversedKvs}},
					{name: "all KVs sorted by value", begin: "", options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByValue}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: kvsByValue}},
					{name: "all KVs sorted by value ascending", begin: "", options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByValue, Order: clientv3.SortAscend}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: kvsByValue}},
					{name: "all KVs sorted by value descending", begin: "", options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByValue, Order: clientv3.SortDescend}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: reversedKvsByValue}},
					{name: "all KVs sorted by value descending limit 2", begin: "", options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByValue, Order: clientv3.SortDescend, Limit: 2}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, More: true, Kvs: reversedKvsByValue[:2]}},
					{name: "all KVs sorted by version ascending", begin: "", options: config.GetOptions{Prefix: true, Order: clientv3.SortAscend, SortBy: clientv3.SortByVersion}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: kvsByVersion}},
					{name: "all KVs sorted by version descending", begin: "", options: config.GetOptions{Prefix: true, SortBy: clientv3.SortByVersion, Order: clientv3.SortDescend}, wantResponse: &clientv3.GetResponse{Header: createHeader(firstHeader, 7), Count: 6, Kvs: kvsByVersionDesc}},
				}
				testsWithKeysOnly := make([]testcase, 0, len(tests))
				for _, otc := range tests {
					if otc.options.CountOnly {
						continue // can't use both --count-only and --keys-only at the same time
					}
					withKeysOnly := otc
					withKeysOnly.name = fmt.Sprintf("%s --keys-only", withKeysOnly.name)
					withKeysOnly.options.KeysOnly = true
					wantResponse := *otc.wantResponse
					wantResponse.Kvs = dropValue(withKeysOnly.wantResponse.Kvs)
					withKeysOnly.wantResponse = &wantResponse
					testsWithKeysOnly = append(testsWithKeysOnly, withKeysOnly)
				}
				for _, tt := range slices.Concat(tests, testsWithKeysOnly) {
					t.Run(tt.name, func(t *testing.T) {
						resp, err := cc.Get(ctx, tt.begin, tt.options)
						require.NoErrorf(t, err, "count not get key %q, err: %s", tt.begin, err)
						resp.Header.MemberId = 0
						assert.Equal(t, tt.wantResponse, resp)
					})
				}
			})
		})
	}
}

func createKV(key, val string, createRev, modRev, ver int64) *mvccpb.KeyValue {
	return &mvccpb.KeyValue{
		Key:            []byte(key),
		Value:          []byte(val),
		CreateRevision: createRev,
		ModRevision:    modRev,
		Version:        ver,
	}
}

//nolint:unparam
func createHeader(firstHeader *etcdserverpb.ResponseHeader, offset int64) *etcdserverpb.ResponseHeader {
	return &etcdserverpb.ResponseHeader{
		ClusterId: firstHeader.ClusterId,
		Revision:  firstHeader.Revision + offset,
		RaftTerm:  firstHeader.RaftTerm,
	}
}

func dropValue(s []*mvccpb.KeyValue) []*mvccpb.KeyValue {
	ss := make([]*mvccpb.KeyValue, 0, len(s))
	for _, kv := range s {
		clone := *kv
		clone.Value = nil
		ss = append(ss, &clone)
	}
	return ss
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
