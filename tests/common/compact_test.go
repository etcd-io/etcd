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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestCompact(t *testing.T) {

	testRunner.BeforeTest(t)
	tcs := []struct {
		name    string
		options config.CompactOption
	}{
		{
			name:    "NoPhysical",
			options: config.CompactOption{Physical: false, Timeout: 10 * time.Second},
		},
		{
			name:    "Physical",
			options: config.CompactOption{Physical: true, Timeout: 10 * time.Second},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t)
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())
			testutils.ExecuteUntil(ctx, t, func() {
				var kvs = []testutils.KV{{Key: "key", Val: "val1"}, {Key: "key", Val: "val2"}, {Key: "key", Val: "val3"}}
				for i := range kvs {
					if err := cc.Put(ctx, kvs[i].Key, kvs[i].Val, config.PutOptions{}); err != nil {
						t.Fatalf("compactTest #%d: put kv error (%v)", i, err)
					}
				}
				get, err := cc.Get(ctx, "key", config.GetOptions{Revision: 3})
				if err != nil {
					t.Fatalf("compactTest: Get kv by revision error (%v)", err)
				}

				getkvs := testutils.KeyValuesFromGetResponse(get)
				assert.Equal(t, kvs[1:2], getkvs)

				_, err = cc.Compact(ctx, 4, tc.options)
				if err != nil {
					t.Fatalf("compactTest: Compact error (%v)", err)
				}

				_, err = cc.Get(ctx, "key", config.GetOptions{Revision: 3})
				if err != nil {
					if !strings.Contains(err.Error(), "required revision has been compacted") {
						t.Fatalf("compactTest: Get compact key error (%v)", err)
					}
				} else {
					t.Fatalf("expected '...has been compacted' error, got <nil>")
				}

				_, err = cc.Compact(ctx, 2, tc.options)
				if err != nil {
					if !strings.Contains(err.Error(), "required revision has been compacted") {
						t.Fatal(err)
					}
				} else {
					t.Fatalf("expected '...has been compacted' error, got <nil>")
				}
			})
		})
	}
}
