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

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestDefragOnline(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	options := config.DefragOption{Timeout: 10 * time.Second}
	clus := testRunner.NewCluster(ctx, t)
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		defer clus.Close()
		var kvs = []testutils.KV{{Key: "key", Val: "val1"}, {Key: "key", Val: "val2"}, {Key: "key", Val: "val3"}}
		for i := range kvs {
			if err := cc.Put(ctx, kvs[i].Key, kvs[i].Val, config.PutOptions{}); err != nil {
				t.Fatalf("compactTest #%d: put kv error (%v)", i, err)
			}
		}
		_, err := cc.Compact(ctx, 4, config.CompactOption{Physical: true, Timeout: 10 * time.Second})
		if err != nil {
			t.Fatalf("defrag_test: compact with revision error (%v)", err)
		}

		if err = cc.Defragment(ctx, options); err != nil {
			t.Fatalf("defrag_test: defrag error (%v)", err)
		}
	})
}
