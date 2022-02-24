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

	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestKVPut(t *testing.T) {
	testRunner.BeforeTest(t)
	clus := testRunner.NewCluster(t)
	defer clus.Close()
	cc := clus.Client()

	testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
		key, value := "foo", "bar"

		if err := cc.Put(key, value); err != nil {
			t.Fatalf("count not put key %q, err: %s", key, err)
		}
		resp, err := cc.Get(key, testutils.WithSerializable())
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
}
