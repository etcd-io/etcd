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

package testutils

import (
	clientv3 "go.etcd.io/etcd/client/v3"
)

type KV struct {
	Key, Val string
}

func KeysFromGetResponse(resp *clientv3.GetResponse) (kvs []string) {
	for _, kv := range resp.Kvs {
		kvs = append(kvs, string(kv.Key))
	}
	return kvs
}

func KeyValuesFromWatchResponse(resp clientv3.WatchResponse) (kvs []KV) {
	for _, event := range resp.Events {
		kvs = append(kvs, KV{Key: string(event.Kv.Key), Val: string(event.Kv.Value)})
	}
	return kvs
}

func KeyValuesFromGetResponse(resp *clientv3.GetResponse) (kvs []KV) {
	for _, kv := range resp.Kvs {
		kvs = append(kvs, KV{Key: string(kv.Key), Val: string(kv.Value)})
	}
	return kvs
}
