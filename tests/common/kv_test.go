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
			config: config.ClusterConfig{ClusterSize: 1, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, PeerTLS: config.AutoTLS},
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
			config: config.ClusterConfig{ClusterSize: 1, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, PeerTLS: config.AutoTLS},
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
					kvs    = []testutils.KeyValue{{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"}}
					revkvs = []testutils.KeyValue{{"key3", "val3"}, {"key2", "val2"}, {"key1", "val1"}}
				)
				for i := range kvs {
					if err := cc.Put(kvs[i].Key, kvs[i].Value); err != nil {
						t.Fatalf("count not put key %q, err: %s", kvs[i].Key, err)
					}
				}
				tests := []struct {
					key     string
					options config.GetOptions

					wkv []testutils.KeyValue
				}{
					{key: "key1", wkv: []testutils.KeyValue{{"key1", "val1"}}},
					{key: "", options: config.GetOptions{Prefix: true}, wkv: kvs},
					{key: "", options: config.GetOptions{FromKey: true}, wkv: kvs},
					{key: "key", options: config.GetOptions{Prefix: true}, wkv: kvs},
					{key: "key", options: config.GetOptions{Prefix: true, Limit: 2}, wkv: kvs[:2]},
					{key: "key", options: config.GetOptions{Prefix: true, Order: clientv3.SortAscend, SortBy: clientv3.SortByModRevision}, wkv: kvs},
					{key: "key", options: config.GetOptions{Prefix: true, Order: clientv3.SortAscend, SortBy: clientv3.SortByVersion}, wkv: kvs},
					{key: "key", options: config.GetOptions{Prefix: true, Order: clientv3.SortNone, SortBy: clientv3.SortByCreateRevision}, wkv: kvs},
					{key: "key", options: config.GetOptions{Prefix: true, Order: clientv3.SortDescend, SortBy: clientv3.SortByCreateRevision}, wkv: revkvs},
					{key: "key", options: config.GetOptions{Prefix: true, Order: clientv3.SortDescend, SortBy: clientv3.SortByKey}, wkv: revkvs},
				}
				for _, tt := range tests {
					resp, err := cc.Get(tt.key, tt.options)
					if err != nil {
						t.Fatalf("count not get key %q, err: %s", tt.key[0], err)
					}
					kvs := testutils.FromGetResponse(resp)
					assert.Equal(t, tt.wkv, kvs)
				}
			})
		})
	}
}
