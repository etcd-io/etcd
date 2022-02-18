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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestKVPut(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				key, value := "foo", "bar"

				if err := cc.Put(key, value); err != nil {
					t.Fatalf("count not put key %q, err: %s", key, err)
				}
				resp, err := cc.Get(key, config.GetOptions{Serializable: true})
				if err != nil {
					t.Fatalf("count not get key %q, err: %s", key, err)
				}
				if len(resp.Kvs) != 1 {
					t.Errorf("Unexpected lenth of response, got %d", len(resp.Kvs))
				}
				if string(resp.Kvs[0].Key) != key {
					t.Errorf("Unexpected key, want %q, got %q", key, resp.Kvs[0].Key)
				}
				if string(resp.Kvs[0].Value) != value {
					t.Errorf("Unexpected value, want %q, got %q", value, resp.Kvs[0].Value)
				}
			})
		})
	}
}

func TestKVGet(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				var (
					kvs          = []string{"a", "b", "c", "c", "c", "foo", "foo/abc", "fop"}
					wantKvs      = []string{"a", "b", "c", "foo", "foo/abc", "fop"}
					kvsByVersion = []string{"a", "b", "foo", "foo/abc", "fop", "c"}
					reversedKvs  = []string{"fop", "foo/abc", "foo", "c", "b", "a"}
				)

				for i := range kvs {
					if err := cc.Put(kvs[i], "bar"); err != nil {
						t.Fatalf("count not put key %q, err: %s", kvs[i], err)
					}
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
					resp, err := cc.Get(tt.begin, tt.options)
					if err != nil {
						t.Fatalf("count not get key %q, err: %s", tt.begin, err)
					}
					kvs := testutils.KeysFromGetResponse(resp)
					assert.Equal(t, tt.wkv, kvs)
				}
			})
		})
	}
}
