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
	"errors"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
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

func KeyValuesFromGetResponse(resp *clientv3.GetResponse) (kvs []KV) {
	for _, kv := range resp.Kvs {
		kvs = append(kvs, KV{Key: string(kv.Key), Val: string(kv.Value)})
	}
	return kvs
}

func KeyValuesFromWatchResponse(resp clientv3.WatchResponse) (kvs []KV) {
	for _, event := range resp.Events {
		kvs = append(kvs, KV{Key: string(event.Kv.Key), Val: string(event.Kv.Value)})
	}
	return kvs
}

func KeyValuesFromWatchChan(wch clientv3.WatchChan, wantedLen int, timeout time.Duration) (kvs []KV, err error) {
	for {
		select {
		case watchResp, ok := <-wch:
			if ok {
				kvs = append(kvs, KeyValuesFromWatchResponse(watchResp)...)
				if len(kvs) == wantedLen {
					return kvs, nil
				}
			}
		case <-time.After(timeout):
			return nil, errors.New("closed watcher channel should not block")
		}
	}
}

func MustClient(c intf.Client, err error) intf.Client {
	if err != nil {
		panic(err)
	}
	return c
}
