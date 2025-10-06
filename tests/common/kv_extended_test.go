// Copyright 2024 The etcd Authors
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
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestKVPutWithEmptyValue(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				key := "empty-value-key"
				value := ""

				require.NoError(t, cc.Put(ctx, key, value, config.PutOptions{}))
				
				resp, err := cc.Get(ctx, key, config.GetOptions{})
				require.NoError(t, err)
				assert.Len(t, resp.Kvs, 1)
				assert.Equal(t, key, string(resp.Kvs[0].Key))
				assert.Equal(t, "", string(resp.Kvs[0].Value))
			})
		})
	}
}

func TestKVPutWithLargeKey(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				largeKey := strings.Repeat("k", 1024)
				value := "test-value"

				require.NoError(t, cc.Put(ctx, largeKey, value, config.PutOptions{}))
				
				resp, err := cc.Get(ctx, largeKey, config.GetOptions{})
				require.NoError(t, err)
				assert.Len(t, resp.Kvs, 1)
				assert.Equal(t, largeKey, string(resp.Kvs[0].Key))
				assert.Equal(t, value, string(resp.Kvs[0].Value))
			})
		})
	}
}

func TestKVPutOverwrite(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				key := "overwrite-key"
				value1 := "initial-value"
				value2 := "updated-value"

				require.NoError(t, cc.Put(ctx, key, value1, config.PutOptions{}))
				require.NoError(t, cc.Put(ctx, key, value2, config.PutOptions{}))
				
				resp, err := cc.Get(ctx, key, config.GetOptions{})
				require.NoError(t, err)
				assert.Len(t, resp.Kvs, 1)
				assert.Equal(t, value2, string(resp.Kvs[0].Value))
			})
		})
	}
}
